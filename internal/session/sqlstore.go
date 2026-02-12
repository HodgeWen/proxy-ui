package session

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// sqlStore implements the scs.Store interface for session persistence
// using standard database/sql. Compatible with pure-Go SQLite (glebarez/sqlite).
// Schema: token TEXT PRIMARY KEY, data BLOB, expiry REAL (julianday).
type sqlStore struct {
	db        *sql.DB
	tableName string
	stopCleanup chan bool
}

// NewSQLStore returns a new sqlStore for the given *sql.DB.
// Uses "sessions" table. Starts background cleanup every 5 minutes.
func NewSQLStore(db *sql.DB) *sqlStore {
	s := &sqlStore{db: db, tableName: "sessions"}
	s.stopCleanup = make(chan bool)
	go s.startCleanup(5 * time.Minute)
	return s
}

// Find returns the data for a given session token.
func (s *sqlStore) Find(token string) (b []byte, found bool, err error) {
	stmt := fmt.Sprintf("SELECT data FROM %s WHERE token = ? AND julianday('now') < expiry", s.tableName)
	row := s.db.QueryRow(stmt, token)
	err = row.Scan(&b)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

// Commit adds or updates session token and data with expiry.
func (s *sqlStore) Commit(token string, b []byte, expiry time.Time) error {
	stmt := fmt.Sprintf("REPLACE INTO %s (token, data, expiry) VALUES (?, ?, julianday(?))", s.tableName)
	_, err := s.db.Exec(stmt, token, b, expiry.UTC().Format("2006-01-02T15:04:05.999"))
	return err
}

// Delete removes a session token.
func (s *sqlStore) Delete(token string) error {
	stmt := fmt.Sprintf("DELETE FROM %s WHERE token = ?", s.tableName)
	_, err := s.db.Exec(stmt, token)
	return err
}

func (s *sqlStore) startCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			_ = s.deleteExpired()
		case <-s.stopCleanup:
			ticker.Stop()
			return
		}
	}
}

func (s *sqlStore) deleteExpired() error {
	stmt := fmt.Sprintf("DELETE FROM %s WHERE expiry < julianday('now')", s.tableName)
	_, err := s.db.Exec(stmt)
	if err != nil {
		log.Printf("[session] cleanup expired: %v", err)
	}
	return err
}
