package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBridgeLifecycle(t *testing.T) {
	// 1. Create a temporary database path
	tmpDir, err := os.MkdirTemp("", "torbi-bridge-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	dbPass := "test_password_123"

	// 2. Start the engine using the native Go function
	port := StartEngineGo(dbPath, dbPass, 0)
	if port <= 0 {
		t.Fatalf("Failed to start engine, returned port: %d", port)
	}

	// 3. Query the status endpoint
	statusURL := fmt.Sprintf("http://127.0.0.1:%d/status", port)
	resp, err := http.Get(statusURL)
	if err != nil {
		t.Fatalf("Failed to query /status endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK (200), got: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var statusMap map[string]interface{}
	if err := json.Unmarshal(body, &statusMap); err != nil {
		t.Fatalf("Failed to unmarshal status JSON: %v", err)
	}

	if _, ok := statusMap["peer_id"]; !ok {
		t.Error("Expected 'peer_id' in status response")
	}
	if _, ok := statusMap["listen_addresses"]; !ok {
		t.Error("Expected 'listen_addresses' in status response")
	}

	// 4. Query the chats endpoint (should be empty initially)
	chatsURL := fmt.Sprintf("http://127.0.0.1:%d/chats", port)
	chatsResp, err := http.Get(chatsURL)
	if err != nil {
		t.Fatalf("Failed to query /chats endpoint: %v", err)
	}
	defer chatsResp.Body.Close()

	var chatsList []interface{}
	if err := json.NewDecoder(chatsResp.Body).Decode(&chatsList); err != nil {
		t.Fatalf("Failed to unmarshal chats JSON: %v", err)
	}

	if len(chatsList) != 0 {
		t.Errorf("Expected 0 chats, got: %d", len(chatsList))
	}

	// 5. Shutdown the engine
	StopEngineGo()

	// 6. Verify server is shut down
	client := http.Client{
		Timeout: 200 * time.Millisecond,
	}
	_, err = client.Get(statusURL)
	if err == nil {
		t.Error("Expected HTTP requests to fail after engine stop, but it succeeded")
	}
}
