package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/ergochat/irc-go/ircmsg"
	_ "modernc.org/sqlite"
)

// Update your session struct
type DB struct {
	// ... previous fields (ServerConn, Clients, Upstream) ...

	conn     *sql.DB
	LogQueue chan ircmsg.Message // Channel for async database writes
}

type ChatMessage struct {
	id        int
	Source    string
	Target    string
	Content   string
	Timestamp int64
	Command   string
}

func NewDB(file string, maxQLen int) (*DB, error) {
	db := DB{}

	// Check for blank database string, the database itself doesn't check for it.
	if file == "" {
		return nil, fmt.Errorf("database file path is empty")
	}

	log.Printf("[database] Using %d for max queued messages", maxQLen)
	db.LogQueue = make(chan ircmsg.Message, maxQLen)

	// Open DB and init tables
	err := db.Init(file)
	if err != nil {
		return nil, err
	}

	// Start goroutine for writing messages
	go db.DatabaseWriter()

	return &db, nil
}

func (db *DB) Init(file string) error {
	// Open DB connection
	log.Printf("[database] Using file %s", file)
	conn, err := sql.Open("sqlite", file)
	if err != nil {
		panic(err)
	}
	db.conn = conn

	// Verify it works
	if err := conn.Ping(); err != nil {
		panic(err)
	}

	// Ensure DB is not accessed concurrently
	conn.SetMaxOpenConns(10)

	// Enable WAL for increased performance
	if _, err := conn.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		log.Printf("[database] Error enabling WAL mode for DB: %v", &err)
	}

	// WAL means we don't need hard syncing for the database
	if _, err := conn.Exec("PRAGMA synchronous = NORMAL;"); err != nil {
		log.Printf("[database] Error enabling NORMAL sync mode for DB: %v", &err)
	}

	// Init DB schema
	log.Printf("[database] Creating schema")
	err = db.runMigrations()
	if err != nil {
		panic(err)
	}

	return nil
}

// We return a read-only channel (<-chan) for messages, and an error channel.
// TODO: change history support to send history for a single channel
// TODO: get rid of
func (db *DB) AsyncGetMessages(ctx context.Context, limit int) (<-chan ChatMessage, <-chan error) {
	// Unbuffered channel so we only hold one message in RAM at a time
	msgChan := make(chan ChatMessage)

	// Buffered error channel so the goroutine can exit immediately after an error
	errChan := make(chan error, 1)

	// Spin up the worker goroutine
	go func() {
		// Ensure channels are closed when the goroutine exits
		defer close(msgChan)
		defer close(errChan)

		// Grab raw rows from the table
		query := `SELECT * FROM history ORDER BY timestamp ASC LIMIT ?`
		rows, err := db.conn.Query(query, limit)
		if err != nil {
			errChan <- err
			return
		}
		defer rows.Close()

		// Ship raw rows off to caller
		for rows.Next() {
			var msg ChatMessage
			if err := rows.Scan(&msg.id, &msg.Source, &msg.Target, &msg.Content, &msg.Timestamp); err != nil {
				log.Printf("[database] Error scanning row: %v", err)
				continue
			}

			// The critical safety block
			select {
			case msgChan <- msg:
				// Message was successfully handed off to the network writer
			case <-ctx.Done():
				// The downstream client disconnected! Abort the database read.
				errChan <- ctx.Err()
				return
			}
		}

		// Check for errors mid-stream
		if err := rows.Err(); err != nil {
			errChan <- err
		}
	}()

	// Pass channels back to caller
	return msgChan, errChan
}

func (db *DB) GetMessages(source string, limit int) ([]ChatMessage, error) {
	// 1. Execute the query
	// We order by id DESC to get the newest messages, and limit the count.
	var rows *sql.Rows
	var err error

	if source == "" {
		query := `SELECT * FROM history ORDER BY id DESC LIMIT ?`
		rows, err = db.conn.Query(query, limit)
		if err != nil {
			return nil, fmt.Errorf("failed to query history: %w", err)
		}
	} else {
		query := `SELECT * FROM history WHERE source = ? ORDER BY id DESC LIMIT ?`
		rows, err = db.conn.Query(query, source, limit)
		if err != nil {
			return nil, fmt.Errorf("failed to query history: %w", err)
		}
	}

	// ALWAYS defer rows.Close() immediately after checking the error
	defer rows.Close()

	var msgs []ChatMessage

	// 2. Iterate through the results
	for rows.Next() {
		var msg ChatMessage
		// Scan copies the columns from the current row into the variables provided.
		if err := rows.Scan(&msg.id, &msg.Source, &msg.Target, &msg.Content, &msg.Timestamp); err != nil {
			log.Printf("Error scanning row: %v\n", err)
			continue // Skip bad rows rather than crashing
		}
		msgs = append(msgs, msg)
	}

	// 3. Check for errors encountered during iteration
	// This is a crucial step often missed in Go database tutorials!
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating history rows: %w", err)
	}

	return msgs, nil
}

// Dedicated goroutine for writing to SQLite
func (db *DB) DatabaseWriter() {
	for {
		// Wait for at least one message to arrive
		msg, ok := <-db.LogQueue
		if !ok {
			log.Printf("[database] Writer quitting due to closed channel")
			return // Channel closed, exit goroutine
		}

		// Start a transaction
		tx, err := db.conn.Begin()
		if err != nil {
			log.Printf("[database] Error starting transaction: %v", err)
			continue
		}

		stmt, err := tx.Prepare("INSERT INTO history (source, target, content, timestamp, command) VALUES (?, ?, ?, ?, ?)")
		if err != nil {
			log.Printf("[database] Error preparing statement: %v", err)
			tx.Rollback()
			continue
		}

		// Helper to safely extract params without crashing the bouncer
		writeMsg := func(m ircmsg.Message) {
			target := ""
			content := ""
			if len(m.Params) > 0 {
				target = m.Params[0]
			}
			if len(m.Params) > 1 {
				content = m.Params[1]
			}

			// FIX 2: Pass m.Command into the Exec statement
			stmt.Exec(m.Nick(), target, content, time.Now().Unix(), m.Command)
		}

		writeMsg(msg)
		// Drain whatever else is currently sitting in the queue without blocking
		// Label is here so we can break out of the outer loop
	drainLoop:
		for i := 0; i < 100; i++ { // Max 100 messages per transaction batch
			select {
			case nextMsg := <-db.LogQueue:
				writeMsg(nextMsg)
			default:
				// Queue is empty for now, break out and commit what we have
				break drainLoop
			}
		}

		stmt.Close()
		err = tx.Commit()
		if err != nil {
			log.Printf("[database] Error committing batch: %v", err)
		}
	}
}

func (db *DB) runMigrations() error {
	// 1. Ensure the migration tracking table exists
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	// 2. Find the current database version
	var currentVersion int
	err = db.conn.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&currentVersion)
	if err != nil {
		// If the table is empty, Scan returns an error or NULL. We treat it as version 0.
		currentVersion = 0
	}

	log.Printf("[database] Current schema version: %d", currentVersion)

	// 3. Iterate through registered migrations
	for _, migration := range registeredMigrations {
		if migration.Version > currentVersion {
			log.Printf("[database] Applying migration %d: %s", migration.Version, migration.Name)

			// Start a transaction for this specific migration
			tx, err := db.conn.Begin()
			if err != nil {
				return fmt.Errorf("failed to begin transaction for migration %d: %w", migration.Version, err)
			}

			// Execute the migration SQL
			if _, err := tx.Exec(migration.UpSQL); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to execute migration %d (%s): %w", migration.Version, migration.Name, err)
			}

			// Update the schema tracking table
			if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", migration.Version); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to update migration version to %d: %w", migration.Version, err)
			}

			// Commit the transaction
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
			}

			log.Printf("[database] Successfully applied migration %d", migration.Version)
		}
	}

	return nil
}
