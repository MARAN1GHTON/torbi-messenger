package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"torbi/database"
	"torbi/network"
	torbicrypto "torbi/crypto"

	"github.com/libp2p/go-libp2p/core/peer"
)

func main() {
	// 1. Define CLI flags
	dbPathFlag := flag.String("db", "torbi.db", "Path to the SQLite database file")
	dbPassFlag := flag.String("pass", "", "Decryption password for the SQLCipher SQLite database (Required)")
	portFlag := flag.Int("port", 10001, "Port for the libp2p P2P network host")
	flag.Parse()

	if *dbPassFlag == "" {
		fmt.Println("Error: Database decryption password is required. Use the -pass flag.")
		flag.Usage()
		os.Exit(1)
	}

	// 2. Initialize encrypted database
	dbAbsPath, err := filepath.Abs(*dbPathFlag)
	if err != nil {
		log.Fatalf("Error getting database path: %v", err)
	}

	fmt.Printf("[Torbi] Initializing database at: %s...\n", dbAbsPath)
	db, err := database.InitDB(dbAbsPath, *dbPassFlag)
	if err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}
	defer db.Close()
	fmt.Println("[Torbi] Database decrypted and loaded successfully.")

	// 3. Start libp2p network host
	fmt.Printf("[Torbi] Launching libp2p node on port %d...\n", *portFlag)
	nm, err := network.NewNetworkManager(db, *portFlag)
	if err != nil {
		log.Fatalf("Network initialization failed: %v", err)
	}
	defer nm.Host.Close()

	// 4. Output local node identities
	fmt.Println("\n================= Node Details =================")
	fmt.Printf("PeerID: %s\n", nm.Host.ID())
	fmt.Println("Listen Multiaddresses:")
	for _, addr := range nm.Host.Addrs() {
		fmt.Printf("  %s/p2p/%s\n", addr, nm.Host.ID())
	}
	fmt.Println("================================================")
	fmt.Println("[Torbi] Local peer discovery started via mDNS.")
	fmt.Println("[Torbi] Type /help to see all available commands.")

	// 5. Setup Network callbacks to update CLI
	var activeChatID string
	var activePeerID string

	nm.OnMessageReceived = func(chatID string, msg *database.Message, plaintext string) {
		// Output the message cleanly.
		// If the user is currently in the active chat, print it directly.
		if chatID == activeChatID {
			fmt.Printf("\r[%s] %s: %s\n[torbi - chatting] > ", 
				time.UnixMilli(msg.Timestamp).Format("15:04:05"), 
				msg.SenderID[:8], 
				plaintext)
		} else {
			fmt.Printf("\r\n[New Message in Chat %s] %s: %s\n> ", 
				chatID[:8], 
				msg.SenderID[:8], 
				plaintext)
		}
	}

	nm.OnPeerSyncDone = func(peerID string, chatID string) {
		fmt.Printf("\r[Sync] Synchronized chat history with peer: %s\n> ", peerID[:8])
	}

	// 6. Enter interactive command line loop
	scanner := bufio.NewScanner(os.Stdin)
	for {
		prompt := "> "
		if activeChatID != "" {
			prompt = fmt.Sprintf("[torbi - chatting with %s] > ", activePeerID[:8])
		} else {
			prompt = "[torbi] > "
		}
		fmt.Print(prompt)

		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Process commands
		if strings.HasPrefix(line, "/") {
			parts := strings.SplitN(line, " ", 2)
			cmd := parts[0]
			var args string
			if len(parts) > 1 {
				args = strings.TrimSpace(parts[1])
			}

			switch cmd {
			case "/help":
				printHelp()
			case "/connect":
				if args == "" {
					fmt.Println("Usage: /connect <multiaddress>")
					continue
				}
				fmt.Printf("Connecting to %s...\n", args)
				if err := nm.ConnectToPeer(args); err != nil {
					fmt.Printf("Connection failed: %v\n", err)
				} else {
					fmt.Println("Connection established successfully.")
				}
			case "/peers":
				online := nm.GetOnlinePeers()
				fmt.Printf("--- Connected Peers (%d) ---\n", len(online))
				for _, p := range online {
					fmt.Printf("- %s\n", p)
				}
			case "/chats":
				chats, err := db.ListChats()
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					continue
				}
				fmt.Printf("--- Available Chats (%d) ---\n", len(chats))
				for _, c := range chats {
					fmt.Printf("- Chat ID: %s (Type: %s)\n", c.ID, c.Type)
				}
			case "/chat":
				if args == "" {
					fmt.Println("Usage: /chat <peerID>")
					continue
				}

				targetPeerID, err := peer.Decode(args)
				if err != nil {
					fmt.Printf("Invalid Peer ID: %v\n", err)
					continue
				}

				peerRec, err := db.GetPeer(targetPeerID.String())
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					continue
				}
				if peerRec == nil {
					fmt.Println("Error: Peer credentials not found. Connect to them first via /connect or wait for mDNS discovery.")
					continue
				}

				// Generate/retrieve chat
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
					fmt.Printf("Error: %v\n", err)
					continue
				}

				if chat == nil {
					// We have the peer's E2EE key, so we can initialize the chat session
					// Derive session key
					secret, err := torbicrypto.DeriveSharedSecret(nm.E2EEPrivKey, peerRec.E2EEPubKey)
					if err != nil {
						fmt.Printf("Key derivation failed: %v\n", err)
						continue
					}
					sessionKey, err := torbicrypto.DeriveSessionKey(secret, nil, []byte("torbi-direct-chat"))
					if err != nil {
						fmt.Printf("Session key derivation failed: %v\n", err)
						continue
					}
					chat = &database.Chat{
						ID:         chatID,
						Type:       "direct",
						SessionKey: sessionKey,
					}
					if err := db.SaveChat(chat); err != nil {
						fmt.Printf("Failed to save chat: %v\n", err)
						continue
					}
				}

				activeChatID = chatID
				activePeerID = targetPeerID.String()
				fmt.Printf("Switched to chat session with %s. Type /history to view past logs.\n", activePeerID[:8])

			case "/history":
				if activeChatID == "" {
					fmt.Println("Error: Choose a chat room first using /chat <peerID>")
					continue
				}
				msgs, err := db.GetChatMessages(activeChatID)
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					continue
				}
				fmt.Printf("--- Chat history for %s (%d messages) ---\n", activePeerID[:8], len(msgs))
				chat, _ := db.GetChat(activeChatID)
				for _, m := range msgs {
					plain, err := torbicrypto.Decrypt(chat.SessionKey, m.EncryptedBody)
					var bodyStr string
					if err != nil {
						bodyStr = fmt.Sprintf("[Decryption error: %v]", err)
					} else {
						bodyStr = string(plain)
					}
					senderName := "Self"
					if m.SenderID != nm.Host.ID().String() {
						senderName = m.SenderID[:8]
					}
					tStr := time.UnixMilli(m.Timestamp).Format("2006-01-02 15:04:05")
					fmt.Printf("[%s] [Clock: %d] %s: %s\n", tStr, m.LamportClock, senderName, bodyStr)
				}
			case "/close":
				activeChatID = ""
				activePeerID = ""
				fmt.Println("Closed active chat session.")
			case "/exit", "/quit":
				fmt.Println("Shutting down node. Goodbye!")
				return
			default:
				fmt.Printf("Unknown command: %s. Type /help to list commands.\n", cmd)
			}
		} else {
			// Sending a normal text message to active chat
			if activeChatID == "" {
				fmt.Println("Error: No active chat session. Use /chat <peerID> or type /help.")
				continue
			}

			targetPeer, err := peer.Decode(activePeerID)
			if err != nil {
				fmt.Printf("Error decoding active peer ID: %v\n", err)
				continue
			}

			if err := nm.SendChatMessage(targetPeer, activeChatID, line); err != nil {
				fmt.Printf("Failed to send message: %v\n", err)
			}
		}
	}
}

func printHelp() {
	fmt.Println("\nAvailable commands:")
	fmt.Println("  /connect <multiaddr>  - Connect to a peer manually (e.g. /connect /ip4/127.0.0.1/tcp/10001/p2p/...)")
	fmt.Println("  /peers               - List all online/connected network peers")
	fmt.Println("  /chats               - List all registered chat sessions")
	fmt.Println("  /chat <peerID>       - Open or switch to direct chat with a peer")
	fmt.Println("  /history             - View decrypted message logs for the active chat session")
	fmt.Println("  /close               - Close the active chat session (return to main prompt)")
	fmt.Println("  /exit or /quit       - Exit the messenger application")
	fmt.Println("  <message>            - Type and press Enter when inside a chat session to send a message")
	fmt.Println()
}
