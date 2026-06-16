package sync

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"torbi/database"
)

func TestSynchronizationProtocol(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "torbi-sync-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPathA := filepath.Join(tempDir, "peerA.db")
	dbPathB := filepath.Join(tempDir, "peerB.db")
	pwd := "password"

	// 1. Initialize databases
	dbA, err := database.InitDB(dbPathA, pwd)
	if err != nil {
		t.Fatalf("Failed to init DB A: %v", err)
	}
	defer dbA.Close()

	dbB, err := database.InitDB(dbPathB, pwd)
	if err != nil {
		t.Fatalf("Failed to init DB B: %v", err)
	}
	defer dbB.Close()

	// 2. Setup the same chat session on both sides
	chatID := "chat_A_B"
	chatA := &database.Chat{ID: chatID, Type: "direct", SessionKey: []byte("session-key")}
	chatB := &database.Chat{ID: chatID, Type: "direct", SessionKey: []byte("session-key")}

	dbA.SaveChat(chatA)
	dbB.SaveChat(chatB)

	// 3. Simulate Peer A writing messages offline
	msgA1 := &database.Message{
		ID:            "msg-A-1",
		ChatID:        chatID,
		SenderID:      "peer-A",
		EncryptedBody: []byte("body-A-1"),
		Timestamp:     100,
		LamportClock:  1,
	}
	msgA2 := &database.Message{
		ID:            "msg-A-2",
		ChatID:        chatID,
		SenderID:      "peer-A",
		EncryptedBody: []byte("body-A-2"),
		Timestamp:     200,
		LamportClock:  2,
	}
	dbA.SaveMessage(msgA1)
	dbA.SaveMessage(msgA2)

	// Simulate Peer B writing messages offline
	msgB1 := &database.Message{
		ID:            "msg-B-1",
		ChatID:        chatID,
		SenderID:      "peer-B",
		EncryptedBody: []byte("body-B-1"),
		Timestamp:     150,
		LamportClock:  1, // Logical clocks overlap when offline!
	}
	dbB.SaveMessage(msgB1)

	// Verify initial count
	msgsA, _ := dbA.GetChatMessages(chatID)
	msgsB, _ := dbB.GetChatMessages(chatID)
	if len(msgsA) != 2 || len(msgsB) != 1 {
		t.Fatalf("Initial message counts mismatch: A has %d, B has %d", len(msgsA), len(msgsB))
	}

	// 4. Run the 3-step synchronization protocol simulation
	// Step 1: Peer A generates its state vector request and sends it to Peer B
	payload1, err := BuildInitialRequest(dbA)
	if err != nil {
		t.Fatalf("BuildInitialRequest failed: %v", err)
	}

	// Step 2: Peer B receives payload1, ingests A's data (none in A's request),
	// and replies with B's deltas for A + B's own state vector request
	payload2, err := ProcessRequestAndResponse(dbB, payload1, true)
	if err != nil {
		t.Fatalf("Step 2 processing failed: %v", err)
	}

	// Step 3: Peer A receives payload2, ingests B's deltas (msg-B-1),
	// and computes A's deltas for B (msg-A-1, msg-A-2)
	payload3, err := ProcessRequestAndResponse(dbA, payload2, false)
	if err != nil {
		t.Fatalf("Step 3 processing failed: %v", err)
	}

	// Step 4: Peer B receives payload3 and ingests A's deltas (msg-A-1, msg-A-2)
	_, err = ProcessRequestAndResponse(dbB, payload3, false)
	if err != nil {
		t.Fatalf("Step 4 processing failed: %v", err)
	}

	// 5. Verify that both databases are fully synchronized and contain all 3 messages
	syncedMsgsA, err := dbA.GetChatMessages(chatID)
	if err != nil {
		t.Fatalf("GetChatMessages A failed: %v", err)
	}
	syncedMsgsB, err := dbB.GetChatMessages(chatID)
	if err != nil {
		t.Fatalf("GetChatMessages B failed: %v", err)
	}

	if len(syncedMsgsA) != 3 || len(syncedMsgsB) != 3 {
		t.Fatalf("Synchronization failed! Message counts after sync: A has %d, B has %d", len(syncedMsgsA), len(syncedMsgsB))
	}

	// Map and compare messages on both sides to verify equality
	mapA := make(map[string]*database.Message)
	for _, m := range syncedMsgsA {
		mapA[m.ID] = m
	}

	for _, mB := range syncedMsgsB {
		mA, ok := mapA[mB.ID]
		if !ok {
			t.Fatalf("Message %s exists in B but not in A", mB.ID)
		}
		if mA.ChatID != mB.ChatID || mA.SenderID != mB.SenderID || !bytes.Equal(mA.EncryptedBody, mB.EncryptedBody) || mA.LamportClock != mB.LamportClock {
			t.Fatalf("Message %s details mismatch between A and B", mB.ID)
		}
	}
}
