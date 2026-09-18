package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// forwardingEntry is one row of the `forwardings` table (source,
// destination columns only), with display names resolved via a LEFT JOIN
// against `users` — either side may have no matching user (an external
// forward destination, or a source not otherwise in the directory), so
// SourceName/DestinationName come back empty rather than the join failing.
type forwardingEntry struct {
	Source          string `json:"source"`
	Destination     string `json:"destination"`
	SourceName      string `json:"sourceName"`
	DestinationName string `json:"destinationName"`
}

var errDirectoryDisabled = fmt.Errorf("사용자 조회 기능이 설정되어 있지 않습니다 (MAIL_MONITOR_DB_DSN)")

// listForwardings returns every row of the `forwardings` table, sorted by
// source then destination. Returns an empty slice (no error) when db is
// nil, matching listUsers' behavior for the optional-directory case.
func listForwardings(db *sql.DB) ([]forwardingEntry, error) {
	if db == nil {
		return []forwardingEntry{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := db.QueryContext(ctx, `
		SELECT f.source, f.destination, COALESCE(us.name, ''), COALESCE(ud.name, '')
		FROM forwardings f
		LEFT JOIN users us ON us.email = f.source
		LEFT JOIN users ud ON ud.email = f.destination
		ORDER BY f.source, f.destination`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []forwardingEntry{}
	for rows.Next() {
		var e forwardingEntry
		if err := rows.Scan(&e.Source, &e.Destination, &e.SourceName, &e.DestinationName); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// addForwarding inserts a (source, destination) pair, unless that exact
// pair already exists — a source having several destinations (fan-out
// forwarding) is legitimate and not treated as a duplicate.
func addForwarding(db *sql.DB, source, destination string) (alreadyExists bool, err error) {
	if db == nil {
		return false, errDirectoryDisabled
	}
	if !emailPatternRe.MatchString(source) {
		return false, fmt.Errorf("발신(source) 주소 형식이 올바르지 않습니다: %s", source)
	}
	if !emailPatternRe.MatchString(destination) {
		return false, fmt.Errorf("수신(destination) 주소 형식이 올바르지 않습니다: %s", destination)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var count int
	if err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM forwardings WHERE source = ? AND destination = ?", source, destination,
	).Scan(&count); err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}

	_, err = db.ExecContext(ctx, "INSERT INTO forwardings (source, destination) VALUES (?, ?)", source, destination)
	return false, err
}

// deleteForwarding removes one exact (source, destination) pair. Returns
// found=false if no such row existed.
func deleteForwarding(db *sql.DB, source, destination string) (found bool, err error) {
	if db == nil {
		return false, errDirectoryDisabled
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := db.ExecContext(ctx, "DELETE FROM forwardings WHERE source = ? AND destination = ?", source, destination)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
