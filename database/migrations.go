package database

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
}
