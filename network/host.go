package network

import (
	"context"
	"crypto/ecdh"
	"fmt"
	"log"
	"sync"

	"torbi/database"
	torbicrypto "torbi/crypto"

	"github.com/libp2p/go-libp2p"
	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/multiformats/go-multiaddr"
)

const (
	ChatProtocolID      = "/torbi/chat/1.0.0"
	SyncProtocolID      = "/torbi/sync/1.0.0"
	HandshakeProtocolID = "/torbi/handshake/1.0.0"
	MdnsServiceTag      = "torbi-p2p"
)

// HandshakeMessage is used to exchange E2EE public keys upon connection.
type HandshakeMessage struct {
	E2EEPubKey []byte `json:"e2ee_pub_key"`
}

type NetworkManager struct {
	Host        host.Host
	DB          *database.DB
	E2EEPubKey  []byte
	E2EEPrivKey []byte
	mdnsService mdns.Service

	onlinePeers   map[peer.ID]bool
	onlinePeersMu sync.RWMutex

	// Callbacks
	OnMessageReceived   func(chatID string, msg *database.Message, plaintext string)
	OnPeerSyncDone      func(peerID string, chatID string)
	OnPeerStatusChanged func(peerID string, isOnline bool)
}

// NewNetworkManager initializes keys, creates the libp2p host, and configures transport.
func NewNetworkManager(db *database.DB, port int) (*NetworkManager, error) {
	// 1. Load or generate libp2p host private key
	libp2pPrivKey, err := loadOrGenerateNodeKey(db)
	if err != nil {
		return nil, fmt.Errorf("failed to load/generate libp2p key: %w", err)
	}

	// 2. Load or generate E2EE X25519 key pair
	pubKey, privKey, err := loadOrGenerateE2EEKey(db)
	if err != nil {
		return nil, fmt.Errorf("failed to load/generate E2EE key: %w", err)
	}

	// 3. Configure libp2p host listen addresses (TCP and QUIC)
	listenAddrs := []string{
		fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port),
		fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", port),
	}

	// 4. Create the libp2p host
	h, err := libp2p.New(
		libp2p.Identity(libp2pPrivKey),
		libp2p.ListenAddrStrings(listenAddrs...),
		libp2p.Security(noise.ID, noise.New), // Secure transports
		libp2p.EnableNATService(),
		libp2p.NATPortMap(), // UPnP port mapping
		libp2p.EnableHolePunching(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	nm := &NetworkManager{
		Host:        h,
		DB:          db,
		E2EEPubKey:  pubKey,
		E2EEPrivKey: privKey,
		onlinePeers: make(map[peer.ID]bool),
	}

	// Save self credentials into database
	selfRawPub, err := libp2pcrypto.MarshalPublicKey(h.Peerstore().PubKey(h.ID()))
	if err == nil {
		db.SavePeer(&database.Peer{
			ID:         h.ID().String(),
			PubKey:     selfRawPub,
			E2EEPubKey: pubKey,
		})
	}

	// Set stream handlers
	h.SetStreamHandler(HandshakeProtocolID, nm.handleHandshakeStream)
	h.SetStreamHandler(ChatProtocolID, nm.handleChatStream)
	h.SetStreamHandler(SyncProtocolID, nm.handleSyncStream)

	// Register network notifier to track online status and trigger handshake
	h.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(net network.Network, conn network.Conn) {
			remotePeer := conn.RemotePeer()
			nm.onlinePeersMu.Lock()
			nm.onlinePeers[remotePeer] = true
			nm.onlinePeersMu.Unlock()

			// If we initiated the connection (we are the dialer), run the handshake
			if conn.Stat().Direction == network.DirOutbound {
				go nm.initiateHandshake(remotePeer)
			}

			if nm.OnPeerStatusChanged != nil {
				nm.OnPeerStatusChanged(remotePeer.String(), true)
			}
		},
		DisconnectedF: func(net network.Network, conn network.Conn) {
			// Only mark disconnected if there are no other active connections to this peer
			remotePeer := conn.RemotePeer()
			if len(net.ConnsToPeer(remotePeer)) == 0 {
				nm.onlinePeersMu.Lock()
				delete(nm.onlinePeers, remotePeer)
				nm.onlinePeersMu.Unlock()

				if nm.OnPeerStatusChanged != nil {
					nm.OnPeerStatusChanged(remotePeer.String(), false)
				}
			}
		},
	})

	// 6. Initialize mDNS service for local discovery
	if err := nm.setupMDNS(); err != nil {
		log.Printf("Warning: Failed to start mDNS: %v\n", err)
	}

	return nm, nil
}

// ConnectToPeer manually connects to a multiaddress.
func (nm *NetworkManager) ConnectToPeer(multiaddrStr string) error {
	ma, err := multiaddr.NewMultiaddr(multiaddrStr)
	if err != nil {
		return fmt.Errorf("invalid multiaddress: %w", err)
	}

	info, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		return fmt.Errorf("invalid peer multiaddress: %w", err)
	}

	ctx := context.Background()
	if err := nm.Host.Connect(ctx, *info); err != nil {
		return fmt.Errorf("failed to connect to peer: %w", err)
	}

	return nil
}

// GetOnlinePeers returns the list of currently connected peer IDs.
func (nm *NetworkManager) GetOnlinePeers() []peer.ID {
	nm.onlinePeersMu.RLock()
	defer nm.onlinePeersMu.RUnlock()

	peers := make([]peer.ID, 0, len(nm.onlinePeers))
	for pID := range nm.onlinePeers {
		peers = append(peers, pID)
	}
	return peers
}

// setupMDNS configures local peer discovery.
func (nm *NetworkManager) setupMDNS() error {
	ser := mdns.NewMdnsService(nm.Host, MdnsServiceTag, &mdnsNotifer{nm: nm})
	nm.mdnsService = ser
	return ser.Start()
}

// mdnsNotifer implements mdns.Notifier interface.
type mdnsNotifer struct {
	nm *NetworkManager
}

func (m *mdnsNotifer) HandlePeerFound(pi peer.AddrInfo) {
	// Skip self
	if pi.ID == m.nm.Host.ID() {
		return
	}
	// Try to connect to discovered peer
	go func() {
		ctx := context.Background()
		if err := m.nm.Host.Connect(ctx, pi); err != nil {
			// Silently fail, could be temporary network issues
			return
		}
	}()
}

// Helpers for Key Persistence

func loadOrGenerateNodeKey(db *database.DB) (libp2pcrypto.PrivKey, error) {
	privBytes, err := db.GetConfig("libp2p_priv_key")
	if err != nil {
		return nil, err
	}
	if len(privBytes) > 0 {
		return libp2pcrypto.UnmarshalPrivateKey(privBytes)
	}

	priv, _, err := libp2pcrypto.GenerateKeyPair(libp2pcrypto.Ed25519, -1)
	if err != nil {
		return nil, err
	}
	data, err := libp2pcrypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, err
	}
	if err := db.SaveConfig("libp2p_priv_key", data); err != nil {
		return nil, err
	}
	return priv, nil
}

func loadOrGenerateE2EEKey(db *database.DB) ([]byte, []byte, error) {
	privBytes, err := db.GetConfig("e2ee_priv_key")
	if err != nil {
		return nil, nil, err
	}
	if len(privBytes) > 0 {
		priv, err := ecdh.X25519().NewPrivateKey(privBytes)
		if err != nil {
			return regenerateE2EEKey(db)
		}
		return priv.PublicKey().Bytes(), privBytes, nil
	}
	return regenerateE2EEKey(db)
}

func regenerateE2EEKey(db *database.DB) ([]byte, []byte, error) {
	pub, priv, err := torbicrypto.GenerateX25519KeyPair()
	if err != nil {
		return nil, nil, err
	}
	if err := db.SaveConfig("e2ee_priv_key", priv); err != nil {
		return nil, nil, err
	}
	return pub, priv, nil
}
