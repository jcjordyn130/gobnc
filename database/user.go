package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Id              string
	Username        string
	hashedpw        string
	Defaultidentity string
}

func (d *DB) GetUserByUsername(username string) (*User, error) {
	var u User

	// QueryRow executes a query that is expected to return at most one row.
	err := d.conn.QueryRow(`
		SELECT id, username, hashedpw, defaultidentity 
		FROM users 
		WHERE username = ?
	`, username).Scan(&u.Id, &u.Username, &u.hashedpw, &u.Defaultidentity)

	if err != nil {
		// Specifically check if the error is because the user doesn't exist
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user '%s' not found", username)
		}
		// Handle any other actual database errors (connection lost, bad syntax, etc.)
		return nil, fmt.Errorf("error fetching user from database: %w", err)
	}

	return &u, nil
}

func (d *DB) GetAllUsers() ([]User, error) {
	// 1. Use Query instead of QueryRow to fetch multiple records
	rows, err := d.conn.Query(`SELECT id, username, defaultidentity FROM users`)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}

	// CRITICAL: Always defer closing the rows to prevent connection leaks!
	defer rows.Close()

	var users []User

	// 2. Iterate through the rows
	for rows.Next() {
		var u User

		// This is here so it is obvious if this function is being misused
		u.hashedpw = "%%DELETED%%"

		// Scan the current row into our struct variables
		if err := rows.Scan(&u.Id, &u.Username, &u.Defaultidentity); err != nil {
			return nil, fmt.Errorf("error scanning user row: %w", err)
		}
		users = append(users, u)
	}

	// 3. Check for errors that might have occurred during iteration
	// (e.g., the network connection dropped halfway through fetching the list)
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over user rows: %w", err)
	}

	return users, nil
}

func (d *DB) AddUser(user User) error {
	if user.Id == "" {
		panic("null user.id")
	}

	// 2. Start a transaction because of the circular foreign keys
	log.Debug().Msg("Starting database transaction for AddUser")
	tx, err := d.conn.Begin() // Replace d.db with your actual *sql.DB variable
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Defer a rollback in case anything panics or errors out
	defer tx.Rollback()

	// Create ID for identity
	user.Defaultidentity = uuid.New().String()
	log.Debug().Msgf("Using identity ID %s for default for user %s", user.Defaultidentity, user.Username)

	// 3. Insert the User
	log.Debug().Msg("Inserting User")
	_, err = tx.Exec(`
		INSERT INTO users (id, username, hashedpw, defaultidentity) 
		VALUES (?, ?, ?, ?)`,
		user.Id, user.Username, user.hashedpw, user.Defaultidentity,
	)
	if err != nil {
		return fmt.Errorf("failed to insert user: %w", err)
	}

	// 4. Insert the Default Identity
	log.Debug().Msg("Inserting Default Identity")
	_, err = tx.Exec(`
		INSERT INTO identities (id, owner, realname, nickname, username) 
		VALUES (?, ?, ?, ?, ?)`,
		user.Defaultidentity, user.Id, user.Username, user.Username, user.Username,
	)
	if err != nil {
		return fmt.Errorf("failed to insert default identity: %w", err)
	}

	// 5. Commit the transaction (this is when the DEFERRABLE constraints are checked!)
	log.Debug().Msg("Comitting transaction for AddUser")
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info().Msgf("Successfully added user %s to database", user.Username)
	return nil
}

func (d *DB) RemoveUser(username string) error {
	log.Debug().Msgf("Removing user %s", username)
	result, err := d.conn.Exec(`DELETE FROM users WHERE username == '?'`)
	if err != nil {
		return fmt.Errorf("failed to remove user: %w", err)
	}

	// 2. Check how many rows were actually deleted
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error checking affected rows: %w", err)
	}

	// 3. If zero rows were affected, the user didn't exist
	if rowsAffected == 0 {
		return fmt.Errorf("user '%s' does not exist", username)
	}

	return nil
}

func (d *DB) NewUser(username string) User {
	log.Debug().Msgf("Creating new user for %s", username)

	var u User

	// Generate id
	u.Id = uuid.New().String()
	u.Username = username

	log.Debug().Msgf("UUID for user %s: %s", u.Username, u.Id)

	return u
}

func (u *User) SetPassword(password string) error {
	log.Debug().Msgf("Hashing password for %s", u.Username)

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	u.hashedpw = string(hashedBytes)

	return nil
}

func (u *User) VerifyPassword(password string) (bool, error) {
	// 2. Compare password against stored hash
	err := bcrypt.CompareHashAndPassword([]byte(u.hashedpw), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			log.Debug().Msgf("Password verify for %s failed", u.Username)
			return false, nil // Invalid password
		}
		return false, err
	}

	log.Debug().Msgf("Password verify for %s succeeded", u.Username)
	return true, nil // Authentication successful
}
