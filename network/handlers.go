package network

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"torbi/database"
	torbicrypto "torbi/crypto"
	tsync "torbi/sync"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// initiateHandshake dials a peer, sends our E2EE public key, and reads theirs.
func (nm *NetworkManager) initiateHandshake(pID peer.ID) {
	s, err := nm.Host.NewStream(context.Background(), pID, HandshakeProtocolID)
	if err != nil {
		log.Printf("[Handshake] Error opening stream to %s: %v\n", pID, err)
		return
	}
	defer s.Close()

	_ = s.SetDeadline(time.Now().Add(10 * time.Second))

	// Send our E2EE public key
	req := HandshakeMessage{E2EEPubKey: nm.E2EEPubKey}
	if err := json.NewEncoder(s).Encode(req); err != nil {
		log.Printf("[Handshake] Error writing to %s: %v\n", pID, err)
		return
	}

	// Read their E2EE public key
	var resp HandshakeMessage
	if err := json.NewDecoder(s).Decode(&resp); err != nil {
		log.Printf("[Handshake] Error reading from %s: %v\n", pID, err)
		return
	}

	nm.finalizeHandshake(pID, resp.E2EEPubKey)
}

// handleHandshakeStream handles incoming handshake requests from other peers.
func (nm *NetworkManager) handleHandshakeStream(s network.Stream) {
	defer s.Close()
	_ = s.SetDeadline(time.Now().Add(10 * time.Second))

	pID := s.Conn().RemotePeer()

	// Read their E2EE public key
	var req HandshakeMessage
	if err := json.NewDecoder(s).Decode(&req); err != nil {
		log.Printf("[Handshake] Error reading from %s: %v\n", pID, err)
		return
	}

	// Send our E2EE public key
	resp := HandshakeMessage{E2EEPubKey: nm.E2EEPubKey}
	if err := json.NewEncoder(s).Encode(resp); err != nil {
		log.Printf("[Handshake] Error writing to %s: %v\n", pID, err)
		return
	}

	nm.finalizeHandshake(pID, req.E2EEPubKey)
}

func (nm *NetworkManager) finalizeHandshake(pID peer.ID, remoteE2EEPubKey []byte) {
	// Get remote libp2p raw public key
	remoteRawPubKey, err := libp2pcrypto.MarshalPublicKey(nm.Host.Peerstore().PubKey(pID))
	if err != nil {
		log.Printf("[Handshake] Failed to marshal remote public key: %v\n", err)
		return
	}

	// Save peer info
	peerRecord := &database.Peer{
		ID:         pID.String(),
		PubKey:     remoteRawPubKey,
		E2EEPubKey: remoteE2EEPubKey,
	}
	if err := nm.DB.SavePeer(peerRecord); err != nil {
		log.Printf("[Handshake] Failed to save peer: %v\n", err)
		return
	}

	// Calculate deterministic Chat ID: sort peer IDs alphabetically
	localID := nm.Host.ID().String()
	remoteID := pID.String()
	var chatID string
	if localID < remoteID {
		chatID = fmt.Sprintf("%s_%s", localID, remoteID)
	} else {
		chatID = fmt.Sprintf("%s_%s", remoteID, localID)
	}

	// Derive shared E2EE session key
	sharedSecret, err := torbicrypto.DeriveSharedSecret(nm.E2EEPrivKey, remoteE2EEPubKey)
	if err != nil {
		log.Printf("[Handshake] Failed to derive shared secret: %v\n", err)
		return
	}

	sessionKey, err := torbicrypto.DeriveSessionKey(sharedSecret, nil, []byte("torbi-direct-chat"))
	if err != nil {
		log.Printf("[Handshake] Failed to derive session key: %v\n", err)
		return
	}

	// Save Chat
	chatRecord := &database.Chat{
		ID:         chatID,
		Type:       "direct",
		SessionKey: sessionKey,
	}
	if err := nm.DB.SaveChat(chatRecord); err != nil {
		log.Printf("[Handshake] Failed to save chat: %v\n", err)
		return
	}

	// Trigger synchronization only for the peer with lexicographically smaller ID to avoid double sync loops
	if localID < remoteID {
		go nm.TriggerSync(pID, chatID)
	}
}

// handleChatStream receives live incoming messages.
func (nm *NetworkManager) handleChatStream(s network.Stream) {
	defer s.Close()

	var sm tsync.SyncMessage
	if err := json.NewDecoder(s).Decode(&sm); err != nil {
		log.Printf("[Chat] Error decoding message: %v\n", err)
		return
	}

	// Double-check if we already have this message
	exists, err := nm.DB.GetMessage(sm.ID)
	if err != nil {
		log.Printf("[Chat] Error looking up message: %v\n", err)
		return
	}
	if exists != nil {
		return // Ignore duplicate
	}

	// Save to DB
	m := &database.Message{
		ID:            sm.ID,
		ChatID:        sm.ChatID,
		SenderID:      sm.SenderID,
		EncryptedBody: sm.EncryptedBody,
		Timestamp:     sm.Timestamp,
		LamportClock:  sm.LamportClock,
	}
	if err := nm.DB.SaveMessage(m); err != nil {
		log.Printf("[Chat] Error saving message: %v\n", err)
		return
	}

	// Decrypt
	chat, err := nm.DB.GetChat(sm.ChatID)
	if err != nil || chat == nil {
		log.Printf("[Chat] Failed to get chat %s for decryption: %v\n", sm.ChatID, err)
		return
	}

	plain, err := torbicrypto.Decrypt(chat.SessionKey, sm.EncryptedBody)
	if err != nil {
		log.Printf("[Chat] Failed to decrypt message: %v\n", err)
		return
	}

	if nm.OnMessageReceived != nil {
		nm.OnMessageReceived(sm.ChatID, m, string(plain))
	}
}

// SendChatMessage encrypts and transmits a message over libp2p.
func (nm *NetworkManager) SendChatMessage(pID peer.ID, chatID string, text string) error {
	chat, err := nm.DB.GetChat(chatID)
	if err != nil {
		return err
	}
	if chat == nil {
		return fmt.Errorf("chat not found")
	}

	// Encrypt the body
	encrypted, err := torbicrypto.Encrypt(chat.SessionKey, []byte(text))
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	// Get latest Lamport clock, increment it
	maxClock, err := nm.DB.GetMaxClock(chatID)
	if err != nil {
		return err
	}
	newClock := maxClock + 1

	msgID := fmt.Sprintf("%d_%s", time.Now().UnixNano(), nm.Host.ID().String())
	m := &database.Message{
		ID:            msgID,
		ChatID:        chatID,
		SenderID:      nm.Host.ID().String(),
		EncryptedBody: encrypted,
		Timestamp:     time.Now().UnixMilli(),
		LamportClock:  newClock,
	}

	// Save to local database
	if err := nm.DB.SaveMessage(m); err != nil {
		return fmt.Errorf("failed to save message locally: %w", err)
	}

	// Open stream and write message
	s, err := nm.Host.NewStream(context.Background(), pID, ChatProtocolID)
	if err != nil {
		return fmt.Errorf("failed to open chat stream: %w", err)
	}
	defer s.Close()

	sm := tsync.SyncMessage{
		ID:            m.ID,
		ChatID:        m.ChatID,
		SenderID:      m.SenderID,
		EncryptedBody: m.EncryptedBody,
		Timestamp:     m.Timestamp,
		LamportClock:  m.LamportClock,
	}

	if err := json.NewEncoder(s).Encode(sm); err != nil {
		return fmt.Errorf("failed to write chat message to stream: %w", err)
	}

	return nil
}

// handleSyncStream implements the receiver side of the 3-step synchronization protocol.
func (nm *NetworkManager) handleSyncStream(s network.Stream) {
	defer s.Close()
	_ = s.SetDeadline(time.Now().Add(30 * time.Second))

	pID := s.Conn().RemotePeer()

	// 1. Read Step 1 SyncRequest from dialer
	var step1 tsync.SyncPayload
	if err := json.NewDecoder(s).Decode(&step1); err != nil {
		log.Printf("[Sync] Error decoding step 1 from %s: %v\n", pID, err)
		return
	}

	// 2. Process and create Step 2 response (our deltas + our request vector)
	step2, err := tsync.ProcessRequestAndResponse(nm.DB, &step1, true)
	if err != nil {
		log.Printf("[Sync] Error processing step 1 from %s: %v\n", pID, err)
		return
	}

	if err := json.NewEncoder(s).Encode(step2); err != nil {
		log.Printf("[Sync] Error encoding step 2 to %s: %v\n", pID, err)
		return
	}

	// 3. Read Step 3 response from dialer
	var step3 tsync.SyncPayload
	if err := json.NewDecoder(s).Decode(&step3); err != nil {
		log.Printf("[Sync] Error decoding step 3 from %s: %v\n", pID, err)
		return
	}

	// Ingest final deltas
	_, err = tsync.ProcessRequestAndResponse(nm.DB, &step3, false)
	if err != nil {
		log.Printf("[Sync] Error processing step 3 from %s: %v\n", pID, err)
		return
	}

	if nm.OnPeerSyncDone != nil {
		nm.OnPeerSyncDone(pID.String(), "")
	}
}

// TriggerSync initiates the 3-step synchronization protocol with a remote peer.
func (nm *NetworkManager) TriggerSync(pID peer.ID, chatID string) {
	s, err := nm.Host.NewStream(context.Background(), pID, SyncProtocolID)
	if err != nil {
		log.Printf("[Sync] Error opening stream to %s: %v\n", pID, err)
		return
	}
	defer s.Close()

	_ = s.SetDeadline(time.Now().Add(30 * time.Second))

	// 1. Build Step 1 payload (our request vectors)
	step1, err := tsync.BuildInitialRequest(nm.DB)
	if err != nil {
		log.Printf("[Sync] Error building initial request: %v\n", err)
		return
	}

	if err := json.NewEncoder(s).Encode(step1); err != nil {
		log.Printf("[Sync] Error writing step 1 to %s: %v\n", pID, err)
		return
	}

	// 2. Read Step 2 response (their deltas + their request vector)
	var step2 tsync.SyncPayload
	if err := json.NewDecoder(s).Decode(&step2); err != nil {
		log.Printf("[Sync] Error reading step 2 from %s: %v\n", pID, err)
		return
	}

	// Ingest their deltas, and compute Step 3 payload (our deltas for them)
	step3, err := tsync.ProcessRequestAndResponse(nm.DB, &step2, false)
	if err != nil {
		log.Printf("[Sync] Error processing step 2 from %s: %v\n", pID, err)
		return
	}

	// 3. Write Step 3 response to peer
	if err := json.NewEncoder(s).Encode(step3); err != nil {
		log.Printf("[Sync] Error writing step 3 to %s: %v\n", pID, err)
		return
	}

	if nm.OnPeerSyncDone != nil {
		nm.OnPeerSyncDone(pID.String(), chatID)
	}
}

// contextBackground returns a background context.
func contextBackground() context.Context {
	return context.Background()
}
