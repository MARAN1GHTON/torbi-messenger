package database

import (
	"database/sql"
	"fmt"
	"net/url"

	_ "github.com/mutecomm/go-sqlcipher/v4"
)

// DB wraps a sql.DB connection.
type DB struct {
	*sql.DB
}

// InitDB opens the database file using SQLCipher with the provided password.
func InitDB(dbPath, password string) (*DB, error) {
	key := url.QueryEscape(password)
	dsn := fmt.Sprintf("file:%s?_pragma_key=%s&_pragma_cipher_page_size=4096", dbPath, key)

	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)

	// Verify password by attempting to read from sqlite_master
	var count int
	err = sqlDB.QueryRow("SELECT count(*) FROM sqlite_master").Scan(&count)
	if err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("invalid database password or corrupt database: %w", err)
	}

	db := &DB{sqlDB}
	if err := db.createSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create database schema: %w", err)
	}

	return db, nil
}

// createSchema sets up the tables needed for peers, chats, and messages.
func (db *DB) createSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS peers (
			id TEXT PRIMARY KEY,
			public_key BLOB,
			e2ee_public_key BLOB
		);`,
		`CREATE TABLE IF NOT EXISTS chats (
			id TEXT PRIMARY KEY,
			type TEXT,
			session_key BLOB
		);`,
		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			chat_id TEXT,
			sender_id TEXT,
			encrypted_body BLOB,
			timestamp INTEGER,
			lamport_clock INTEGER,
			FOREIGN KEY(chat_id) REFERENCES chats(id)
		);`,
		`CREATE TABLE IF NOT EXISTS config (
			key TEXT PRIMARY KEY,
			value BLOB
		);`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed executing schema query %q: %w", query, err)
		}
	}
	return nil
}

// GetConfig retrieves a configuration value by key.
func (db *DB) GetConfig(key string) ([]byte, error) {
	var val []byte
	err := db.QueryRow("SELECT value FROM config WHERE key = ?", key).Scan(&val)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get config for key %s: %w", key, err)
	}
	return val, nil
}

// SaveConfig saves or updates a configuration key-value pair.
func (db *DB) SaveConfig(key string, val []byte) error {
	query := `INSERT INTO config (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value;`
	if _, err := db.Exec(query, key, val); err != nil {
		return fmt.Errorf("failed to save config for key %s: %w", key, err)
	}
	return nil
}
