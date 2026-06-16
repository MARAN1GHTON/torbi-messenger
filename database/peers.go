package database

import (
	"database/sql"
	"fmt"
)

// Peer represents a registered network peer.
type Peer struct {
	ID         string // libp2p Peer ID
	PubKey     []byte // libp2p raw public key
	E2EEPubKey []byte // X25519 E2EE public key
}

// SavePeer stores or updates a peer in the database.
func (db *DB) SavePeer(p *Peer) error {
	query := `INSERT INTO peers (id, public_key, e2ee_public_key)
		VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			public_key=excluded.public_key,
			e2ee_public_key=excluded.e2ee_public_key;`
	if _, err := db.Exec(query, p.ID, p.PubKey, p.E2EEPubKey); err != nil {
		return fmt.Errorf("failed to save peer %s: %w", p.ID, err)
	}
	return nil
}

// GetPeer retrieves a peer by its ID. Returns nil, nil if the peer is not found.
func (db *DB) GetPeer(id string) (*Peer, error) {
	query := `SELECT id, public_key, e2ee_public_key FROM peers WHERE id = ?;`
	p := &Peer{}
	err := db.QueryRow(query, id).Scan(&p.ID, &p.PubKey, &p.E2EEPubKey)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query peer %s: %w", id, err)
	}
	return p, nil
}

// ListPeers lists all registered peers in the database.
func (db *DB) ListPeers() ([]*Peer, error) {
	query := `SELECT id, public_key, e2ee_public_key FROM peers;`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query peers list: %w", err)
	}
	defer rows.Close()

	var peers []*Peer
	for rows.Next() {
		p := &Peer{}
		if err := rows.Scan(&p.ID, &p.PubKey, &p.E2EEPubKey); err != nil {
			return nil, fmt.Errorf("failed to scan peer row: %w", err)
		}
		peers = append(peers, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error in ListPeers: %w", err)
	}

	return peers, nil
}
