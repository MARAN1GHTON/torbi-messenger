package database

import (
	"database/sql"
	"fmt"
	"strings"
)

// Message represents a record in the messages table.
type Message struct {
	ID             string // Unique message ID (UUID)
	ChatID         string // Chat ID relation
	SenderID       string // Peer ID of the sender
	EncryptedBody  []byte // E2EE body ciphertext
	Timestamp      int64  // Unix timestamp (milliseconds)
	LamportClock   int64  // Lamport clock value at the time of creation
}

// SaveMessage stores or updates a message in the database.
func (db *DB) SaveMessage(m *Message) error {
	query := `INSERT INTO messages (id, chat_id, sender_id, encrypted_body, timestamp, lamport_clock)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			chat_id=excluded.chat_id,
			sender_id=excluded.sender_id,
			encrypted_body=excluded.encrypted_body,
			timestamp=excluded.timestamp,
			lamport_clock=excluded.lamport_clock;`
	if _, err := db.Exec(query, m.ID, m.ChatID, m.SenderID, m.EncryptedBody, m.Timestamp, m.LamportClock); err != nil {
		return fmt.Errorf("failed to save message %s: %w", m.ID, err)
	}
	return nil
}

// GetMessage retrieves a message by its ID. Returns nil, nil if not found.
func (db *DB) GetMessage(id string) (*Message, error) {
	query := `SELECT id, chat_id, sender_id, encrypted_body, timestamp, lamport_clock FROM messages WHERE id = ?;`
	m := &Message{}
	err := db.QueryRow(query, id).Scan(&m.ID, &m.ChatID, &m.SenderID, &m.EncryptedBody, &m.Timestamp, &m.LamportClock)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query message %s: %w", id, err)
	}
	return m, nil
}

// GetChatMessages retrieves all messages for a given chat, sorted by Lamport clock.
func (db *DB) GetChatMessages(chatID string) ([]*Message, error) {
	query := `SELECT id, chat_id, sender_id, encrypted_body, timestamp, lamport_clock FROM messages 
		WHERE chat_id = ? 
		ORDER BY lamport_clock ASC, timestamp ASC;`
	rows, err := db.Query(query, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages for chat %s: %w", chatID, err)
	}
	defer rows.Close()

	var messages []*Message
	for rows.Next() {
		m := &Message{}
		if err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.EncryptedBody, &m.Timestamp, &m.LamportClock); err != nil {
			return nil, fmt.Errorf("failed to scan message row: %w", err)
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error in GetChatMessages: %w", err)
	}
	return messages, nil
}

// GetMaxClock returns the maximum Lamport clock value in a chat.
func (db *DB) GetMaxClock(chatID string) (int64, error) {
	query := `SELECT COALESCE(MAX(lamport_clock), 0) FROM messages WHERE chat_id = ?;`
	var maxClock int64
	if err := db.QueryRow(query, chatID).Scan(&maxClock); err != nil {
		return 0, fmt.Errorf("failed to query max clock for chat %s: %w", chatID, err)
	}
	return maxClock, nil
}

// GetStateVector returns the maximum Lamport clock for each sender in a specific chat.
// This serves as the version vector representing what this peer has already received.
func (db *DB) GetStateVector(chatID string) (map[string]int64, error) {
	query := `SELECT sender_id, MAX(lamport_clock) FROM messages WHERE chat_id = ? GROUP BY sender_id;`
	rows, err := db.Query(query, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to query state vector for chat %s: %w", chatID, err)
	}
	defer rows.Close()

	vector := make(map[string]int64)
	for rows.Next() {
		var senderID string
		var maxClock int64
		if err := rows.Scan(&senderID, &maxClock); err != nil {
			return nil, fmt.Errorf("failed to scan state vector row: %w", err)
		}
		vector[senderID] = maxClock
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error in GetStateVector: %w", err)
	}
	return vector, nil
}

// GetMessagesSince filters messages in a chat that have a higher Lamport clock
// than what is represented in the provided stateVector for each sender.
func (db *DB) GetMessagesSince(chatID string, stateVector map[string]int64) ([]*Message, error) {
	allMsgs, err := db.GetChatMessages(chatID)
	if err != nil {
		return nil, err
	}

	var delta []*Message
	for _, m := range allMsgs {
		limitClock, ok := stateVector[m.SenderID]
		// If we don't have records for this sender, or the message clock is higher than the known limit
		if !ok || m.LamportClock > limitClock {
			delta = append(delta, m)
		}
	}
	return delta, nil
}

// GetMessageIDs returns all message IDs present in a chat room.
func (db *DB) GetMessageIDs(chatID string) ([]string, error) {
	query := `SELECT id FROM messages WHERE chat_id = ?;`
	rows, err := db.Query(query, chatID)
	if err != nil {
		return nil, fmt.Errorf("failed to query message IDs for chat %s: %w", chatID, err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan message ID row: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error in GetMessageIDs: %w", err)
	}
	return ids, nil
}

// GetMessagesExcept returns messages in a chat whose IDs are not in the excluded list.
// It optimizes performance by first finding which message IDs are missing in Go,
// and then fetching only the missing records via SQL (chunked to prevent SQLite parameter limits).
func (db *DB) GetMessagesExcept(chatID string, excludedIDs []string) ([]*Message, error) {
	// 1. Fetch all local message IDs for this chat
	localIDs, err := db.GetMessageIDs(chatID)
	if err != nil {
		return nil, err
	}

	// Create a map for fast lookup of excluded IDs
	excludedMap := make(map[string]bool)
	for _, id := range excludedIDs {
		excludedMap[id] = true
	}

	// 2. Identify missing IDs
	var missingIDs []string
	for _, id := range localIDs {
		if !excludedMap[id] {
			missingIDs = append(missingIDs, id)
		}
	}

	if len(missingIDs) == 0 {
		return nil, nil // No missing messages
	}

	// 3. Query the missing records in chunks to respect SQLite's parameter limits (999)
	const chunkSize = 900
	var missingMessages []*Message

	for i := 0; i < len(missingIDs); i += chunkSize {
		end := i + chunkSize
		if end > len(missingIDs) {
			end = len(missingIDs)
		}
		chunk := missingIDs[i:end]

		// Construct query placeholder: SELECT ... WHERE id IN (?, ?, ...)
		placeholders := make([]string, len(chunk))
		args := make([]interface{}, len(chunk))
		for j, id := range chunk {
			placeholders[j] = "?"
			args[j] = id
		}

		query := fmt.Sprintf(`SELECT id, chat_id, sender_id, encrypted_body, timestamp, lamport_clock 
			FROM messages 
			WHERE id IN (%s) 
			ORDER BY lamport_clock ASC, timestamp ASC;`, strings.Join(placeholders, ","))

		rows, err := db.Query(query, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to query missing messages chunk: %w", err)
		}

		for rows.Next() {
			m := &Message{}
			if err := rows.Scan(&m.ID, &m.ChatID, &m.SenderID, &m.EncryptedBody, &m.Timestamp, &m.LamportClock); err != nil {
				rows.Close()
				return nil, fmt.Errorf("failed to scan missing message row: %w", err)
			}
			missingMessages = append(missingMessages, m)
		}
		rows.Close()
	}

	return missingMessages, nil
}
