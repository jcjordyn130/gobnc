package database

import (
	"fmt"

	"github.com/rs/zerolog/log"
)

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
