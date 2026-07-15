package database

import (
	"database/sql"
	"sync"

	"github.com/ergochat/irc-go/ircmsg"
	"github.com/jmoiron/sqlx"
)

// Update your session struct
type DB struct {
	// ... previous fields (ServerConn, Clients, Upstream) ...

	conn     *sqlx.DB
	LogQueue chan ircmsg.Message // Channel for async database writes
	wg       sync.WaitGroup
}

type FKViolation struct {
	Table  string
	RowID  int64
	Parent string
	Fkid   sql.NullString
}
