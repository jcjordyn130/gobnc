package database

import (
	"fmt"
	"sync"
	"time"

	"github.com/ergochat/irc-go/ircmsg"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
)

var (
	instance *DB       // Singleton instance of the DB struct
	once     sync.Once // lock to ensure that only one DB struct is open
	initErr  error     // errors that occur during Open()
)

// Internal function that actually opens the database, enables required PRAGMAs, and handles schema.
func (db *DB) init(file string) error {
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

// User facing function to actually open the database, MUST be called before Get()
// maxQLen is NOT reconfigurable after open
func Open(file string, maxQLen int) error {
	once.Do(func() {
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
		err := db.init(file)
		if err != nil {
			initErr = err
			return
		}

		// Add one to the sync group to allow for proper database teardown
		db.wg.Add(1)

		// Start goroutine for writing messages
		go db.DatabaseWriter()

		instance = &db
	})

	return initErr
}

func Get() *DB {
	if instance == nil {
		panic("uninitalized database access")
	}

	return instance
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

func (db *DB) Check() error {
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

	for _, v := range violations {
		log.Error().Msgf("Violation in table '%s' (RowID: %d). Missing parent key in '%s'.\n",
			v.Table, v.RowID, v.Parent)
	}

	// Check results
	if len(violations) == 0 {
		return nil
	} else {
		return fmt.Errorf("%d errors found", len(violations))
	}
}

func (db *DB) Optimize() error {
	log.Trace().Msg("[database] Optimizing database")
	if _, err := db.conn.Exec("PRAGMA optimize=0x10002;"); err != nil {
		log.Debug().Msgf("[database] Error optimizing database: %v", &err)
		return err
	}

	log.Trace().Msg("[database] Vacuuming database")
	if _, err := db.conn.Exec("VACUUM;"); err != nil {
		log.Debug().Msgf("[database] Error vacuuming database: %v", &err)
		return err
	}

	return nil
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
