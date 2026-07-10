package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/ergochat/irc-go/ircmsg"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
)

// Update your session struct
type DB struct {
	// ... previous fields (ServerConn, Clients, Upstream) ...

	conn     *sqlx.DB
	LogQueue chan ircmsg.Message // Channel for async database writes
	wg       sync.WaitGroup
}

type ChatMessage struct {
	id        int
	Source    string
	Target    string
	Content   string
	Timestamp int64
	Command   string
}

type FKViolation struct {
	Table  string
	RowID  int64
	Parent string
	Fkid   sql.NullString
}

func NewDB(file string, maxQLen int) (*DB, error) {
	db := DB{}

	// Check for blank database string, the database itself doesn't check for it.
	if file == "" {
		log.Warn().Msg("database file path is empty, using *IN MEMORY* database")
		log.Warn().Msg("Please configure the bouncer!")
		file = ":memory:"
	}

	log.Debug().Msgf("[database] Using %d for max queued messages", maxQLen)
	db.LogQueue = make(chan ircmsg.Message, maxQLen)

	// Open DB and init tables
	err := db.Init(file)
	if err != nil {
		return nil, err
	}

	// Add one to the sync group to allow for proper database teardown
	db.wg.Add(1)

	// Start goroutine for writing messages
	go db.DatabaseWriter()

	return &db, nil
}

func (db *DB) Check() error {
	// Check to make sure we even HAVE a connec tion first.
	if db.conn == nil {
		return fmt.Errorf("database is not open yet!")
	}

	// Run integrity check first
	log.Debug().Msg("[database] Running integry check!")
	result, err := db.conn.Query("PRAGMA integrity_check;")
	if err != nil {
		return err
	}
	defer result.Close()

	for result.Next() {
		var res string
		if err := result.Scan(&res); err != nil {
			return err
		}

		if res != "ok" {
			log.Error().Msgf("[database] Corrupt row found: %s", res)
		}
	}

	if err := result.Err(); err != nil {
		return err
	}

	// Now check foreign keys
	log.Debug().Msg("[database] Running foreign key check!")
	var violations []FKViolation

	// Use Select to fetch and bind all rows automatically
	err = db.conn.Select(&violations, "PRAGMA foreign_key_check;")
	if err != nil {
		return err
	}

	// Check results
	if len(violations) == 0 {
		return nil
	}

	for _, v := range violations {
		log.Error().Msgf("Violation in table '%s' (RowID: %d). Missing parent key in '%s'.\n",
			v.Table, v.RowID, v.Parent)
	}

	return nil
}

func (db *DB) Init(file string) error {
	// Open DB connection
	log.Debug().Msgf("[database] Using file %s", file)
	conn, err := sqlx.Open("sqlite", file)
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
	log.Trace().Msg("[database] Enabling WAL mode")
	if _, err := conn.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		log.Debug().Msgf("[database] Error enabling WAL mode for DB: %v", &err)
	}

	// WAL means we don't need hard syncing for the database
	log.Trace().Msg("[database] Enabling NORMAL sync mode")
	if _, err := conn.Exec("PRAGMA synchronous = NORMAL;"); err != nil {
		log.Debug().Msgf("[database] Error enabling NORMAL sync mode for DB: %v", &err)
	}

	// Enable foreign key constraints
	log.Trace().Msg("[database] Enabling foreign key constraints")
	if _, err := conn.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		log.Debug().Msgf("[database] Error enabling foreign key constraints")
		return err
	}

	// Init DB schema
	log.Debug().Msg("[database] Creating schema")
	err = db.runMigrations()
	if err != nil {
		panic(err)
	}

	return nil
}

func (db *DB) AddAutoJoinChan(channel string) error {
	// Basic sanity checking
	if channel == "" {
		return fmt.Errorf("channel name cannot be empty")
	}
	// The query returns 1 if found, 0 if not
	query := "SELECT EXISTS(SELECT * FROM autojoin WHERE channel = ?)"

	var exists bool
	err := db.conn.QueryRow(query, channel).Scan(&exists)
	if err != nil {
		return fmt.Errorf("error checking autojoin channel: %w", err)
	}

	if !exists {
		log.Debug().Msgf("[database] Adding autojoin channel: %s", channel)
		_, err := db.conn.Exec("INSERT INTO autojoin (channel) VALUES (?)", channel)
		if err != nil {
			return fmt.Errorf("error adding autojoin channel: %w", err)
		}
	} else {
		return fmt.Errorf("channel %s already exists in autojoin list", channel)
	}

	return nil
}

func (db *DB) RemoveAutoJoinChan(channel string) error {
	// Basic sanity checking
	if channel == "" {
		return fmt.Errorf("channel name cannot be empty")
	}

	log.Debug().Msgf("[database] Removing autojoin channel: %s", channel)
	result, err := db.conn.Exec("DELETE FROM autojoin WHERE channel = ?", channel)
	if err != nil {
		return fmt.Errorf("error removing autojoin channel: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking rows affected when removing autojoin channel: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("channel %s not found in autojoin list", channel)
	}

	return nil
}

func (db *DB) GetAutoJoinChans() ([]string, error) {
	log.Debug().Msg("[database] Retrieving autojoin channels")
	query := "SELECT channel FROM autojoin"
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying autojoin channels: %w", err)
	}
	defer rows.Close()

	var channels []string
	for rows.Next() {
		var channel string
		if err := rows.Scan(&channel); err != nil {
			return nil, fmt.Errorf("error scanning autojoin channel: %w", err)
		}
		channels = append(channels, channel)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating autojoin channels: %w", err)
	}
	return channels, nil
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
				log.Debug().Msgf("[database] Error scanning row: %v", err)
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
			log.Debug().Msgf("[database] Error scanning row: %v", err)
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
	defer db.wg.Done() // Signal that this goroutine is done when it exits

	for {
		// Wait for at least one message to arrive
		msg, ok := <-db.LogQueue
		if !ok {
			log.Debug().Msg("[database] Writer quitting due to closed channel")
			return // Channel closed, exit goroutine
		}

		// Start a transaction
		tx, err := db.conn.Begin()
		if err != nil {
			log.Debug().Msgf("[database] Error starting transaction: %v", err)
			continue
		}

		stmt, err := tx.Prepare("INSERT INTO history (source, target, content, timestamp, command) VALUES (?, ?, ?, ?, ?)")
		if err != nil {
			log.Debug().Msgf("[database] Error preparing statement: %v", err)
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
			log.Debug().Msgf("[database] Error committing batch: %v", err)
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

	log.Debug().Msgf("[database] Current schema version: %d", currentVersion)

	// 3. Iterate through registered migrations
	for _, migration := range registeredMigrations {
		if migration.Version > currentVersion {
			log.Debug().Msgf("[database] Applying migration %d: %s", migration.Version, migration.Name)

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

			log.Debug().Msgf("[database] Successfully applied migration %d", migration.Version)
		}
	}

	return nil
}

func (db *DB) Close() error {
	// Close the log queue channel to signal the writer goroutine to exit
	// This causes ok = false when the channel is pulled from
	close(db.LogQueue)

	// Wait for the writer goroutine to finish
	db.wg.Wait()

	// Close the database connection
	if err := db.conn.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}

	log.Debug().Msg("[database] Database closed successfully")
	return nil
}
