package main

import (
	"C"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"torbi/database"
	"torbi/network"
	torbicrypto "torbi/crypto"

	"github.com/gorilla/websocket"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Global references to running engine state
var (
	stateMu    sync.Mutex
	dbInstance *database.DB
	netManager *network.NetworkManager
	httpServer *http.Server
	apiPort    int

	// WebSockets handling
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	wsClients   = make(map[*wsClient]bool)
	wsClientsMu sync.Mutex
)

type wsClient struct {
	conn *websocket.Conn
	send chan []byte
}

type WsEvent struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

// Main is required for c-shared buildmode, though it is not used directly.
func main() {}

//export start_engine
func start_engine(dbPathC *C.char, dbPassC *C.char, portC C.int) C.int {
	dbPath := C.GoString(dbPathC)
	dbPass := C.GoString(dbPassC)
	libp2pPort := int(portC)

	return C.int(StartEngineGo(dbPath, dbPass, libp2pPort))
}

//export stop_engine
func stop_engine() {
	StopEngineGo()
}

// StartEngineGo starts the Go messenger engine and returns the HTTP port, or -1 on error.
func StartEngineGo(dbPath, dbPass string, libp2pPort int) int {
	stateMu.Lock()
	defer stateMu.Unlock()

	// If engine is already running, return the active port
	if httpServer != nil {
		return apiPort
	}

	log.Printf("[Bridge] Starting Torbi Go Engine (DB: %s, NetPort: %d)\n", dbPath, libp2pPort)

	// 1. Initialize DB
	db, err := database.InitDB(dbPath, dbPass)
	if err != nil {
		log.Printf("[Bridge] Database initialization failed: %v\n", err)
		return -1
	}
	dbInstance = db

	// 2. Initialize Network Manager
	nm, err := network.NewNetworkManager(db, libp2pPort)
	if err != nil {
		db.Close()
		dbInstance = nil
		log.Printf("[Bridge] Network manager initialization failed: %v\n", err)
		return -1
	}
	netManager = nm

	// 3. Register Network Manager callbacks for WS broadcasting
	nm.OnMessageReceived = func(chatID string, msg *database.Message, plaintext string) {
		event := WsEvent{
			Event: "message_received",
			Data: map[string]interface{}{
				"chat_id": chatID,
				"message": map[string]interface{}{
					"id":            msg.ID,
					"chat_id":       msg.ChatID,
					"sender_id":     msg.SenderID,
					"body":          plaintext,
					"timestamp":     msg.Timestamp,
					"lamport_clock": msg.LamportClock,
				},
			},
		}
		broadcastEvent(event)
	}

	nm.OnPeerSyncDone = func(peerID string, chatID string) {
		event := WsEvent{
			Event: "sync_done",
			Data: map[string]interface{}{
				"peer_id": peerID,
				"chat_id": chatID,
			},
		}
		broadcastEvent(event)
	}

	nm.OnPeerStatusChanged = func(peerID string, isOnline bool) {
		event := WsEvent{
			Event: "peer_status",
			Data: map[string]interface{}{
				"peer_id":   peerID,
				"is_online": isOnline,
			},
		}
		broadcastEvent(event)
	}

	// 4. Setup Local HTTP API server
	mux := http.NewServeMux()
	mux.HandleFunc("/status", handleStatus)
	mux.HandleFunc("/peers", handlePeers)
	mux.HandleFunc("/chats", handleChats)
	mux.HandleFunc("/chats/", handleChatMessages) // Handles /chats/{id}/messages
	mux.HandleFunc("/connect", handleConnect)
	mux.HandleFunc("/chat", handleStartChat)
	mux.HandleFunc("/ws", handleWebSocket)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		nm.Host.Close()
		db.Close()
		dbInstance = nil
		netManager = nil
		log.Printf("[Bridge] Failed to bind local HTTP listener: %v\n", err)
		return -1
	}

	apiPort = listener.Addr().(*net.TCPAddr).Port
	httpServer = &http.Server{Handler: mux}

	go func() {
		log.Printf("[Bridge] Local HTTP server listening on http://127.0.0.1:%d\n", apiPort)
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("[Bridge] HTTP server error: %v\n", err)
		}
	}()

	return apiPort
}

func StopEngineGo() {
	stateMu.Lock()
	defer stateMu.Unlock()

	if httpServer == nil {
		return
	}

	log.Println("[Bridge] Stopping Go Engine...")

	// 1. Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)
	httpServer = nil

	// 2. Disconnect and close all WS clients
	wsClientsMu.Lock()
	for client := range wsClients {
		client.conn.Close()
		close(client.send)
		delete(wsClients, client)
	}
	wsClientsMu.Unlock()

	// 3. Stop network node
	if netManager != nil {
		netManager.Host.Close()
		netManager = nil
	}

	// 4. Close DB
	if dbInstance != nil {
		dbInstance.Close()
		dbInstance = nil
	}

	log.Println("[Bridge] Go Engine stopped successfully.")
}

// --- WebSocket Event Broadcast Helper ---

func broadcastEvent(event WsEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("[Bridge] Failed to marshal websocket event: %v\n", err)
		return
	}

	wsClientsMu.Lock()
	defer wsClientsMu.Unlock()
	for client := range wsClients {
		select {
		case client.send <- data:
		default:
			// Client block or disconnect, clean up
			client.conn.Close()
			close(client.send)
			delete(wsClients, client)
		}
	}
}

// --- HTTP Route Handlers ---

func handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	stateMu.Lock()
	nm := netManager
	stateMu.Unlock()

	if nm == nil {
		http.Error(w, "Engine Not Initialized", http.StatusServiceUnavailable)
		return
	}

	onlinePeers := nm.GetOnlinePeers()
	listenAddrs := []string{}
	for _, addr := range nm.Host.Addrs() {
		listenAddrs = append(listenAddrs, fmt.Sprintf("%s/p2p/%s", addr, nm.Host.ID()))
	}

	res := map[string]interface{}{
		"peer_id":          nm.Host.ID().String(),
		"listen_addresses": listenAddrs,
		"peers_count":      len(onlinePeers),
		"nat_type":         "AutoNAT/UPnP enabled",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func handlePeers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	stateMu.Lock()
	nm := netManager
	db := dbInstance
	stateMu.Unlock()

	if nm == nil || db == nil {
		http.Error(w, "Engine Not Initialized", http.StatusServiceUnavailable)
		return
	}

	// Fetch all registered peers in DB
	dbPeers, err := db.ListPeers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	onlineSet := make(map[string]bool)
	for _, pID := range nm.GetOnlinePeers() {
		onlineSet[pID.String()] = true
	}

	type PeerResponse struct {
		ID       string `json:"id"`
		IsOnline bool   `json:"is_online"`
	}

	res := []PeerResponse{}
	for _, p := range dbPeers {
		// Do not include self in peers list
		if p.ID == nm.Host.ID().String() {
			continue
		}
		res = append(res, PeerResponse{
			ID:       p.ID,
			IsOnline: onlineSet[p.ID],
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func handleChats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	stateMu.Lock()
	db := dbInstance
	nm := netManager
	stateMu.Unlock()

	if db == nil || nm == nil {
		http.Error(w, "Engine Not Initialized", http.StatusServiceUnavailable)
		return
	}

	dbChats, err := db.ListChats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type ChatResponse struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		PeerID      string `json:"peer_id"`
		LastMessage string `json:"last_message"`
	}

	res := []ChatResponse{}
	selfID := nm.Host.ID().String()

	for _, c := range dbChats {
		// Find remote peer ID from chat ID
		parts := strings.Split(c.ID, "_")
		peerID := ""
		if len(parts) == 2 {
			if parts[0] == selfID {
				peerID = parts[1]
			} else {
				peerID = parts[0]
			}
		}

		// Load last message as a decrypted preview
		lastMsgStr := ""
		msgs, err := db.GetChatMessages(c.ID)
		if err == nil && len(msgs) > 0 {
			lastMsg := msgs[len(msgs)-1]
			decrypted, err := torbicrypto.Decrypt(c.SessionKey, lastMsg.EncryptedBody)
			if err == nil {
				lastMsgStr = string(decrypted)
			} else {
				lastMsgStr = "[Encrypted Message]"
			}
		}

		res = append(res, ChatResponse{
			ID:          c.ID,
			Type:        c.Type,
			PeerID:      peerID,
			LastMessage: lastMsgStr,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func handleChatMessages(w http.ResponseWriter, r *http.Request) {
	// Path should match /chats/{id}/messages
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 || parts[3] != "messages" {
		http.NotFound(w, r)
		return
	}
	chatID := parts[2]

	stateMu.Lock()
	db := dbInstance
	stateMu.Unlock()

	if db == nil {
		http.Error(w, "Engine Not Initialized", http.StatusServiceUnavailable)
		return
	}

	if r.Method == http.MethodGet {
		// Fetch chat session key
		chat, err := db.GetChat(chatID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if chat == nil {
			http.Error(w, "Chat Not Found", http.StatusNotFound)
			return
		}

		// Retrieve messages
		msgs, err := db.GetChatMessages(chatID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		type MessageResponse struct {
			ID           string `json:"id"`
			ChatID       string `json:"chat_id"`
			SenderID     string `json:"sender_id"`
			Body         string `json:"body"`
			Timestamp    int64  `json:"timestamp"`
			LamportClock int64  `json:"lamport_clock"`
		}

		res := []MessageResponse{}
		for _, m := range msgs {
			plain, err := torbicrypto.Decrypt(chat.SessionKey, m.EncryptedBody)
			bodyStr := ""
			if err != nil {
				bodyStr = "[Decryption Failed]"
			} else {
				bodyStr = string(plain)
			}

			res = append(res, MessageResponse{
				ID:           m.ID,
				ChatID:       m.ChatID,
				SenderID:     m.SenderID,
				Body:         bodyStr,
				Timestamp:    m.Timestamp,
				LamportClock: m.LamportClock,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)

	} else if r.Method == http.MethodPost {
		// Send message
		type SendRequest struct {
			Body string `json:"body"`
		}

		var req SendRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		stateMu.Lock()
		nm := netManager
		stateMu.Unlock()

		if nm == nil {
			http.Error(w, "Engine Not Initialized", http.StatusServiceUnavailable)
			return
		}

		// Find recipient PeerID from ChatID
		parts := strings.Split(chatID, "_")
		if len(parts) != 2 {
			http.Error(w, "Invalid Chat ID structure", http.StatusBadRequest)
			return
		}
		recipientStr := parts[0]
		if recipientStr == nm.Host.ID().String() {
			recipientStr = parts[1]
		}

		recipientPeer, err := peer.Decode(recipientStr)
		if err != nil {
			http.Error(w, "Invalid Recipient Peer ID", http.StatusBadRequest)
			return
		}

		log.Printf("[Bridge] Sending message in chat %s to peer %s\n", chatID, recipientStr)
		err = nm.SendChatMessage(recipientPeer, chatID, req.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to send: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
	} else {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	type ConnectRequest struct {
		Multiaddr string `json:"multiaddr"`
	}

	var req ConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	stateMu.Lock()
	nm := netManager
	stateMu.Unlock()

	if nm == nil {
		http.Error(w, "Engine Not Initialized", http.StatusServiceUnavailable)
		return
	}

	err := nm.ConnectToPeer(req.Multiaddr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "connected"})
}

func handleStartChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	type StartChatRequest struct {
		PeerID string `json:"peer_id"`
	}

	var req StartChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	stateMu.Lock()
	db := dbInstance
	nm := netManager
	stateMu.Unlock()

	if db == nil || nm == nil {
		http.Error(w, "Engine Not Initialized", http.StatusServiceUnavailable)
		return
	}

	targetPeerID, err := peer.Decode(req.PeerID)
	if err != nil {
		http.Error(w, "Invalid Peer ID", http.StatusBadRequest)
		return
	}

	peerRec, err := db.GetPeer(targetPeerID.String())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if peerRec == nil {
		http.Error(w, "Peer credentials not found. Connect first or wait for mDNS discovery.", http.StatusNotFound)
		return
	}

	// Generate or retrieve chat session ID
	localID := nm.Host.ID().String()
	remoteID := targetPeerID.String()
	var chatID string
	if localID < remoteID {
		chatID = fmt.Sprintf("%s_%s", localID, remoteID)
	} else {
		chatID = fmt.Sprintf("%s_%s", remoteID, localID)
	}

	chat, err := db.GetChat(chatID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if chat == nil {
		secret, err := torbicrypto.DeriveSharedSecret(nm.E2EEPrivKey, peerRec.E2EEPubKey)
		if err != nil {
			http.Error(w, fmt.Sprintf("Shared secret derivation failed: %v", err), http.StatusInternalServerError)
			return
		}
		sessionKey, err := torbicrypto.DeriveSessionKey(secret, nil, []byte("torbi-direct-chat"))
		if err != nil {
			http.Error(w, fmt.Sprintf("Session key derivation failed: %v", err), http.StatusInternalServerError)
			return
		}
		chat = &database.Chat{
			ID:         chatID,
			Type:       "direct",
			SessionKey: sessionKey,
		}
		if err := db.SaveChat(chat); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save chat: %v", err), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"chat_id": chatID})
}

// --- WebSocket Connection Upgrade ---

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Bridge] WebSocket upgrade failed: %v\n", err)
		return
	}

	client := &wsClient{
		conn: conn,
		send: make(chan []byte, 256),
	}

	wsClientsMu.Lock()
	wsClients[client] = true
	wsClientsMu.Unlock()

	// Goroutine for handling client writes
	go func() {
		defer func() {
			wsClientsMu.Lock()
			delete(wsClients, client)
			wsClientsMu.Unlock()
			conn.Close()
		}()

		for {
			message, ok := <-client.send
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			err := conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				return
			}
		}
	}()

	// Keep alive / ping reader to detect client disconnects
	go func() {
		defer func() {
			wsClientsMu.Lock()
			delete(wsClients, client)
			wsClientsMu.Unlock()
			conn.Close()
		}()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()
}
