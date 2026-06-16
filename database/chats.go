package database

import (
	"database/sql"
	"fmt"
)

// Chat represents an active chat session (direct or group).
type Chat struct {
	ID         string // Unique chat ID
	Type       string // E.g., "direct"
	SessionKey []byte // E2EE session key for the chat
}

// SaveChat stores or updates a chat session in the database.
func (db *DB) SaveChat(c *Chat) error {
	query := `INSERT INTO chats (id, type, session_key)
		VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			type=excluded.type,
			session_key=excluded.session_key;`
	if _, err := db.Exec(query, c.ID, c.Type, c.SessionKey); err != nil {
		return fmt.Errorf("failed to save chat %s: %w", c.ID, err)
	}
	return nil
}

// GetChat retrieves a chat session by its ID. Returns nil, nil if not found.
func (db *DB) GetChat(id string) (*Chat, error) {
	query := `SELECT id, type, session_key FROM chats WHERE id = ?;`
	c := &Chat{}
	err := db.QueryRow(query, id).Scan(&c.ID, &c.Type, &c.SessionKey)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query chat %s: %w", id, err)
	}
	return c, nil
}

// ListChats lists all active chats in the database.
func (db *DB) ListChats() ([]*Chat, error) {
	query := `SELECT id, type, session_key FROM chats;`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query chats list: %w", err)
	}
	defer rows.Close()

	var chats []*Chat
	for rows.Next() {
		c := &Chat{}
		if err := rows.Scan(&c.ID, &c.Type, &c.SessionKey); err != nil {
			return nil, fmt.Errorf("failed to scan chat row: %w", err)
		}
		chats = append(chats, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error in ListChats: %w", err)
	}

	return chats, nil
}
