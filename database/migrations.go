package database

import (
	"fmt"

	"github.com/rs/zerolog/log"
)

type Migration struct {
	Version int    // A sequential number (1, 2, 3...) or a timestamp
	Name    string // e.g., "add_command_column_to_history"
	UpSQL   string // The SQL to execute to apply the migration
}

// Migrations MUST be in ascending order of their Version.
var registeredMigrations = []Migration{
	{
		Version: 1,
		Name:    "initial_history_table",
		UpSQL: `
			CREATE TABLE IF NOT EXISTS history (
				id INTEGER PRIMARY KEY AUTOINCREMENT, 
				source TEXT, 
				target TEXT, 
				content TEXT, 
				timestamp INTEGER
			);
		`,
	},
	{
		Version: 2,
		Name:    "add_command_column",
		UpSQL: `
			ALTER TABLE history ADD COLUMN command TEXT DEFAULT 'PRIVMSG';
		`,
	},
	{
		Version: 3,
		Name:    "add_autojoin_table",
		UpSQL: `
			CREATE TABLE IF NOT EXISTS autojoin (
				channel TEXT PRIMARY KEY,
				UNIQUE(channel)
			);
			`,
	},
	{
		Version: 4,
		Name:    "add_extra_tables",
		UpSQL: `
			CREATE TABLE IF NOT EXISTS servers (
			id	TEXT PRIMARY KEY,
			domain TEXT,
			port INTEGER,
			ssl NOT NULL DEFAULT 0 CHECK (ssl IN (0, 1)),
			identity TEXT,
			user TEXT,

			FOREIGN KEY (identity) REFERENCES identities(id) ON DELETE SET NULL,
			FOREIGN KEY (user) REFERENCES users(id) ON DELETE CASCADE
			);

			CREATE TABLE IF NOT EXISTS identities (
			id TEXT PRIMARY KEY,
			owner TEXT,
			realname TEXT,
			nickname TEXT,
			username TEXT,
			
			FOREIGN KEY (owner) REFERENCES users(id) ON DELETE CASCADE
			);

			CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE,
			hashedpw TEXT,
			defaultidentity TEXT NOT NULL,

			FOREIGN KEY (defaultidentity) REFERENCES identities(id) ON DELETE RESTRICT 
			DEFERRABLE INITIALLY DEFERRED
			);
			`,
	},
	{
		Version: 5,
		Name:    "add_db_indexes",
		UpSQL: `
			CREATE INDEX idx_history_target_time ON history(target, timestamp DESC);
			CREATE INDEX idx_identities_owner ON identities(owner);
			CREATE INDEX idx_servers_user ON servers(user);
			CREATE INDEX idx_servers_identity ON servers(identity);
			CREATE INDEX idx_users_defaultidentity ON users(defaultidentity);
			CREATE INDEX idx_history_source_time ON history(source, timestamp DESC);
			`,
	},
	{
		Version: 6,
		Name:    "add_server_name",
		UpSQL: `
			ALTER TABLE servers ADD COLUMN name TEXT;
		`,
	},
}

// Internal function used to handle migrations during database opening
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
