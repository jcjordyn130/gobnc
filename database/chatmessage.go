package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rs/zerolog/log"
)

type ChatMessage struct {
	id        int
	Source    string
	Target    string
	Content   string
	Timestamp int64
	Command   string
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
