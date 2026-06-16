package database

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDatabaseOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "torbi-db-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	password := "correct_password"

	// 1. Initialize DB with SQLCipher
	db, err := InitDB(dbPath, password)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	// 2. Verify wrong password rejection
	db.Close()
	_, err = InitDB(dbPath, "wrong_password")
	if err == nil {
		t.Fatalf("Expected error opening database with incorrect password, but got nil")
	}

	// Reopen with correct password
	db, err = InitDB(dbPath, password)
	if err != nil {
		t.Fatalf("Reopen failed: %v", err)
	}
	defer db.Close()

	// 3. Test Peers CRUD
	p := &Peer{
		ID:         "peer-1",
		PubKey:     []byte("libp2p-pubkey-bytes"),
		E2EEPubKey: []byte("e2ee-pubkey-bytes"),
	}
	if err := db.SavePeer(p); err != nil {
		t.Fatalf("SavePeer failed: %v", err)
	}

	retrievedPeer, err := db.GetPeer("peer-1")
	if err != nil {
		t.Fatalf("GetPeer failed: %v", err)
	}
	if retrievedPeer == nil || retrievedPeer.ID != p.ID || !bytes.Equal(retrievedPeer.PubKey, p.PubKey) || !bytes.Equal(retrievedPeer.E2EEPubKey, p.E2EEPubKey) {
		t.Fatalf("Retrieved peer doesn't match saved peer")
	}

	peers, err := db.ListPeers()
	if err != nil {
		t.Fatalf("ListPeers failed: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("Expected 1 peer, got %d", len(peers))
	}

	// 4. Test Chats CRUD
	c := &Chat{
		ID:         "chat-1",
		Type:       "direct",
		SessionKey: []byte("a-secure-32-byte-session-key-data"),
	}
	if err := db.SaveChat(c); err != nil {
		t.Fatalf("SaveChat failed: %v", err)
	}

	retrievedChat, err := db.GetChat("chat-1")
	if err != nil {
		t.Fatalf("GetChat failed: %v", err)
	}
	if retrievedChat == nil || retrievedChat.ID != c.ID || !bytes.Equal(retrievedChat.SessionKey, c.SessionKey) {
		t.Fatalf("Retrieved chat doesn't match saved chat")
	}

	chats, err := db.ListChats()
	if err != nil {
		t.Fatalf("ListChats failed: %v", err)
	}
	if len(chats) != 1 {
		t.Fatalf("Expected 1 chat, got %d", len(chats))
	}

	// 5. Test Messages CRUD
	m1 := &Message{
		ID:            "msg-1",
		ChatID:        "chat-1",
		SenderID:      "peer-1",
		EncryptedBody: []byte("cipher-1"),
		Timestamp:     1000,
		LamportClock:  1,
	}
	m2 := &Message{
		ID:            "msg-2",
		ChatID:        "chat-1",
		SenderID:      "peer-2",
		EncryptedBody: []byte("cipher-2"),
		Timestamp:     2000,
		LamportClock:  2,
	}

	if err := db.SaveMessage(m1); err != nil {
		t.Fatalf("SaveMessage 1 failed: %v", err)
	}
	if err := db.SaveMessage(m2); err != nil {
		t.Fatalf("SaveMessage 2 failed: %v", err)
	}

	msgs, err := db.GetChatMessages("chat-1")
	if err != nil {
		t.Fatalf("GetChatMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(msgs))
	}

	maxClock, err := db.GetMaxClock("chat-1")
	if err != nil {
		t.Fatalf("GetMaxClock failed: %v", err)
	}
	if maxClock != 2 {
		t.Fatalf("Expected max clock to be 2, got %d", maxClock)
	}

	// Test State Vector
	vector, err := db.GetStateVector("chat-1")
	if err != nil {
		t.Fatalf("GetStateVector failed: %v", err)
	}
	if vector["peer-1"] != 1 || vector["peer-2"] != 2 {
		t.Fatalf("State vector mismatch: expected peer-1:1, peer-2:2, got %v", vector)
	}

	// Test GetMessagesSince
	limitVector := map[string]int64{
		"peer-1": 0,
		"peer-2": 1,
	}
	deltas, err := db.GetMessagesSince("chat-1", limitVector)
	if err != nil {
		t.Fatalf("GetMessagesSince failed: %v", err)
	}
	if len(deltas) != 2 {
		t.Fatalf("Expected 2 delta messages, got %d", len(deltas))
	}

	limitVector2 := map[string]int64{
		"peer-1": 1,
		"peer-2": 2,
	}
	deltas2, err := db.GetMessagesSince("chat-1", limitVector2)
	if err != nil {
		t.Fatalf("GetMessagesSince failed: %v", err)
	}
	if len(deltas2) != 0 {
		t.Fatalf("Expected 0 delta messages, got %d", len(deltas2))
	}
}
