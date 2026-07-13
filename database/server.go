package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type Server struct {
	Name     string `db:"name"`
	Id       string `db:"id"`
	Domain   string `db:"domain"`
	Port     int    `db:"port"`
	Ssl      bool   `db:"ssl"`
	Identity string `db:"identity"`
	User     string `db:"user"`
}

func (d *DB) GetServerByID(id string) (*Server, error) {
	var s Server

	// sqlx's Get executes the query and unmarshals the single row into the struct.
	err := d.conn.Get(&s, `SELECT * FROM servers WHERE id = ?`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("server '%s' not found", id)
		}

		// Handle any other actual database errors
		return nil, fmt.Errorf("error fetching server from database: %w", err)
	}

	return &s, nil
}

func (d *DB) GetAllServers() ([]Server, error) {
	// We can initialize an empty slice of users directly.
	// Ensure you use []User, not []*User, unless your requirements specifically demand pointers here.
	var servers []Server

	// Select executes a query, iterates over rows, and unmarshals them into the slice.
	// It automatically closes the rows and handles iteration errors for you.
	err := d.conn.Select(&servers, `SELECT * from servers`)
	if err != nil {
		return nil, fmt.Errorf("failed to query servers: %w", err)
	}

	return servers, nil
}

func (d *DB) AddServer(s Server) error {
	// Safety check to ensure an ID is present before inserting
	if s.Id == "" {
		return fmt.Errorf("cannot insert server: missing server ID")
	}

	// The SQL Query using named parameters.
	// sqlx automatically matches the :named_parameters (e.g., :domain)
	// to the `db` tags on your Server struct (e.g., `db:"domain"`).
	query := `
		INSERT INTO servers (name, id, domain, port, ssl, identity, user)
		VALUES (:name, :id, :domain, :port, :ssl, :identity, :user)`

	// Execute the insert using the struct directly
	_, err := d.conn.NamedExec(query, s)
	if err != nil {
		return fmt.Errorf("failed to insert server: %w", err)
	}

	return nil
}

func (d *DB) RemoveServer(s Server) error {
	log.Debug().Msgf("Removing server %s", s.Id)
	result, err := d.conn.Exec(`DELETE FROM servers WHERE id = ?`, s.Id)
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
		return fmt.Errorf("server '%s' does not exist", s.Id)
	}

	return nil
}

func (d *DB) UpdateServer(s Server) error {
	// 1. Safety Check: Never execute an update without a WHERE clause target
	if s.Id == "" {
		return fmt.Errorf("cannot update server: missing server ID")
	}

	// 2. The SQL Query
	// sqlx will automatically match the :named_parameters to the `db` tags
	// on your User struct (e.g., :username maps to `db:"username"`).
	query := `
		UPDATE server
		SET
			name = :name,
			domain = :domain,
			port = :port,
			ssl = :ssl,
			identity = :identity,
			user = :user
		WHERE id = :id`

	// 3. Execute the update using the struct
	result, err := d.conn.NamedExec(query, s)
	if err != nil {
		return fmt.Errorf("failed to execute structure update: %w", err)
	}

	// 4. Verify the update actually happened
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("update failed: no server found with id %s", s.Id)
	}

	return nil
}

func (d *DB) NewServer(u User) Server {
	// BOOM!
	if u.Id == "" || u.Defaultidentity == "" {
		panic("u.Id or u.Defaultidentity blank in NewServer")
	}

	log.Debug().Msgf("Creating new server for %s", u.Username)

	var s Server

	// Generate id
	s.Id = uuid.New().String()
	s.User = u.Id
	s.Identity = u.Defaultidentity

	log.Debug().Msgf("UUID for new server: %s", s.Id)

	return s
}
