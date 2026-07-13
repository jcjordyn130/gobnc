package database

import (
	"database/sql"
	"errors"
	"fmt"
)

type Identity struct {
	Id       string `db:"id"`
	Owner    string `db:"owner"`
	Realname string `db:"realname"`
	Nickname string `db:"nickname"`
	Username string `db:"username"`
}

func (d *DB) GetIdentityByID(id string) (*Identity, error) {
	var i Identity

	// sqlx's Get executes the query and unmarshals the single row into the struct.
	err := d.conn.Get(&i, `SELECT * FROM identities WHERE id = ?`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("identity '%s' not found", id)
		}

		// Handle any other actual database errors
		return nil, fmt.Errorf("error fetching identity from database: %w", err)
	}

	return &i, nil
}
