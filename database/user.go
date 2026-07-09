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
	Id              string `db:"id"`
	Username        string `db:"username"`
	HashedPW        string `db:"hashedpw"`
	Defaultidentity string `db:"defaultidentity"`
}

var redactedVal string = "%%REDACTED%%"

func (d *DB) GetUserByUsername(username string) (*User, error) {
	var u User

	// sqlx's Get executes the query and unmarshals the single row into the struct.
	err := d.conn.Get(&u, `SELECT id, username, hashedpw, defaultidentity FROM users WHERE username = ?`, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user '%s' not found", username)
		}

		// Handle any other actual database errors
		return nil, fmt.Errorf("error fetching user from database: %w", err)
	}

	return &u, nil
}

func (d *DB) GetAllUsers() ([]User, error) {
	// We can initialize an empty slice of users directly.
	// Ensure you use []User, not []*User, unless your requirements specifically demand pointers here.
	var users []User

	// Select executes a query, iterates over rows, and unmarshals them into the slice.
	// It automatically closes the rows and handles iteration errors for you.
	err := d.conn.Select(&users, `SELECT id, username, defaultidentity FROM users`)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}

	// Callers don't need the raw hashed password
	for i := range users {
		users[i].HashedPW = redactedVal
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
		user.Id, user.Username, user.HashedPW, user.Defaultidentity,
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
	result, err := d.conn.Exec(`DELETE FROM users WHERE username = ?`)
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

func (d *DB) UpdateUser(u User) error {
	// 1. Safety Check: Never execute an update without a WHERE clause target
	if u.Id == "" {
		return fmt.Errorf("cannot update user: missing user ID")
	}

	// 2. The SQL Query
	// sqlx will automatically match the :named_parameters to the `db` tags
	// on your User struct (e.g., :username maps to `db:"username"`).
	query := `
		UPDATE users 
		SET 
			username = :username, 
			hashedpw = :hashedpw, 
			defaultidentity = :defaultidentity
		WHERE id = :id`

	// 3. Execute the update using the struct
	result, err := d.conn.NamedExec(query, u)
	if err != nil {
		return fmt.Errorf("failed to execute structure update: %w", err)
	}

	// 4. Verify the update actually happened
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("update failed: no user found with id %s", u.Id)
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

	u.HashedPW = string(hashedBytes)

	return nil
}

func (u *User) VerifyPassword(password string) (bool, error) {
	// Check for password redaction first
	if u.HashedPW == redactedVal {
		return false, fmt.Errorf("Password Hash for '%s' has been redacted...", u.Username)
	}

	// 2. Compare password against stored hash
	err := bcrypt.CompareHashAndPassword([]byte(u.HashedPW), []byte(password))
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
