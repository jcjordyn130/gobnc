package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

// AsyncStreamQuery is a generic helper that executes a query and streams the results.
// T represents the struct type you are returning (e.g., ChatMessage).
func AsyncStreamQuery[T any](
	ctx context.Context,
	db *sql.DB,
	query string,
	scanner func(*sql.Rows) (T, error),
	args ...any,
) (<-chan T, <-chan error) {

	msgChan := make(chan T)
	errChan := make(chan error, 1)

	go func() {
		defer close(msgChan)
		defer close(errChan)

		// 1. Execute the query
		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			log.Debug().Msgf("[database] Query execution failed: %v", err)
			errChan <- err
			return
		}
		defer rows.Close()

		rowCount := 0

		// 2. Iterate and scan
		for rows.Next() {
			rowCount++
			// Delegate the actual scanning to the caller's specific struct
			item, err := scanner(rows)
			if err != nil {
				log.Debug().Msgf("[database] Error scanning generic row: %v", err)
				continue
			}

			// 3. Safe handoff
			select {
			case msgChan <- item:
			case <-ctx.Done():
				log.Debug().Msgf("[database] Context cancelled! Aborting stream.")
				errChan <- ctx.Err()
				return
			}
		}

		log.Debug().Msgf("[database] Finished iterating. Total rows processed: %d", rowCount)
		if err := rows.Err(); err != nil {
			log.Debug().Msgf("[database] Error during row iteration: %v", err)
			errChan <- err
		}
	}()

	return msgChan, errChan
}

func (db *DB) AsyncGetDirectMessages(ctx context.Context, myNick string, limit int) (<-chan ChatMessage, <-chan error) {
	query := `
	SELECT id, source, target, content, timestamp, COALESCE(command, 'PRIVMSG') 
	FROM (
		SELECT id, source, target, content, timestamp, command 
		FROM history 
		WHERE target NOT LIKE '#%' 
		  AND target NOT LIKE '&%'
		  AND (source = ? COLLATE NOCASE OR target = ? COLLATE NOCASE)
		ORDER BY timestamp DESC, id DESC LIMIT ?
	)
	ORDER BY timestamp ASC, id ASC`

	// Define how to map a single row into a ChatMessage
	scanner := func(rows *sql.Rows) (ChatMessage, error) {
		var msg ChatMessage
		err := rows.Scan(&msg.id, &msg.Source, &msg.Target, &msg.Content, &msg.Timestamp, &msg.Command)
		return msg, err
	}

	// Call the generic helper
	return AsyncStreamQuery(ctx, db.conn.DB, query, scanner, myNick, myNick, limit)
}

// AsyncSearchMessages streams history records that match dynamic filters.
func (db *DB) AsyncSearchMessages(ctx context.Context, filters map[string]string, limit int) (<-chan ChatMessage, <-chan error) {
	validColumns := map[string]bool{
		"source":  true,
		"target":  true,
		"content": true,
	}

	var conditions []string
	var args []any

	// 1. Build the dynamic WHERE clause
	for col, searchTerm := range filters {
		if !validColumns[col] {
			// If a column is invalid, we return a closed/errored channel immediately
			errChan := make(chan error, 1)
			errChan <- fmt.Errorf("invalid search column: %s", col)
			close(errChan)

			msgChan := make(chan ChatMessage)
			close(msgChan)
			return msgChan, errChan
		}

		// Note: For exact channel matching, it is actually safer to use exact matches (=)
		// rather than LIKE, so "#channel" doesn't accidentally pull history for "#channel2".
		// But if you prefer partial matching, you can keep the LIKE and "%" logic here.
		conditions = append(conditions, fmt.Sprintf("%s = ?", col))
		args = append(args, searchTerm)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	// 2. Construct the safe, ordered sub-query
	// We use fmt.Sprintf just to inject the dynamic WHERE clause safely.
	// The actual user inputs are still parameterized via args...
	query := fmt.Sprintf(`
		SELECT id, source, target, content, timestamp, command 
		FROM (
			SELECT id, source, target, content, timestamp, COALESCE(command, 'PRIVMSG') as command 
			FROM history 
			%s
			ORDER BY timestamp DESC, id DESC LIMIT ?
		)
		ORDER BY timestamp ASC, id ASC
	`, whereClause)

	// Append the limit to align with the final '?'
	args = append(args, limit)

	// 3. Define the scanner
	scanner := func(rows *sql.Rows) (ChatMessage, error) {
		var msg ChatMessage
		err := rows.Scan(&msg.id, &msg.Source, &msg.Target, &msg.Content, &msg.Timestamp, &msg.Command)
		return msg, err
	}

	// 4. Fire the generic runner
	return AsyncStreamQuery(ctx, db.conn.DB, query, scanner, args...)
}
