package store

import (
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type User struct {
	Fingerprint string
	Nickname    string
	Flair       bool
	VisitCount  int
	LastSeen    time.Time
}

type Store struct {
	db *sql.DB
	mu sync.Mutex
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		fingerprint TEXT PRIMARY KEY,
		nickname    TEXT UNIQUE,
		flair       INTEGER DEFAULT 0,
		visit_count INTEGER DEFAULT 1,
		last_seen   DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS bans (
		fingerprint TEXT PRIMARY KEY,
		reason      TEXT,
		banned_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at  DATETIME
	);
	CREATE TABLE IF NOT EXISTS weekly_visitors (
		fingerprint TEXT PRIMARY KEY,
		first_seen  DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS chat_messages (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		room        TEXT NOT NULL,
		fingerprint TEXT,
		nickname    TEXT,
		color_index INTEGER DEFAULT 0,
		text        TEXT NOT NULL,
		is_system   INTEGER DEFAULT 0,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_chat_room_created ON chat_messages(room, created_at);
	`
	_, err := s.db.Exec(schema)
	return err
}

func (s *Store) UpsertUser(fingerprint, nickname string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO users (fingerprint, nickname, visit_count, last_seen)
		VALUES (?, ?, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(fingerprint) DO UPDATE SET
			visit_count = visit_count + 1,
			last_seen = CURRENT_TIMESTAMP
	`, fingerprint, nickname)
	return err
}

func (s *Store) GetUser(fingerprint string) (*User, error) {
	row := s.db.QueryRow(
		`SELECT fingerprint, nickname, flair, visit_count, last_seen FROM users WHERE fingerprint = ?`,
		fingerprint,
	)
	u := &User{}
	var lastSeen string
	var flairInt int
	err := row.Scan(&u.Fingerprint, &u.Nickname, &flairInt, &u.VisitCount, &lastSeen)
	u.Flair = flairInt != 0
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.LastSeen, _ = time.Parse("2006-01-02 15:04:05", lastSeen)
	return u, nil
}

func (s *Store) SetNickname(fingerprint, nickname string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`UPDATE users SET nickname = ? WHERE fingerprint = ?`, nickname, fingerprint)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			return errors.New("nickname already taken")
		}
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (s *Store) IsBanned(fingerprint string) (bool, error) {
	s.mu.Lock()
	s.db.Exec(`DELETE FROM bans WHERE expires_at IS NOT NULL AND expires_at < CURRENT_TIMESTAMP`)
	s.mu.Unlock()

	row := s.db.QueryRow(`SELECT COUNT(*) FROM bans WHERE fingerprint = ?`, fingerprint)
	var count int
	if err := row.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) Ban(fingerprint, reason string, expiresAt *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var exp interface{}
	if expiresAt != nil {
		exp = expiresAt.UTC().Format("2006-01-02 15:04:05")
	}
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO bans (fingerprint, reason, banned_at, expires_at)
		VALUES (?, ?, CURRENT_TIMESTAMP, ?)
	`, fingerprint, reason, exp)
	return err
}

func (s *Store) Unban(fingerprint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM bans WHERE fingerprint = ?`, fingerprint)
	return err
}

func (s *Store) RecordVisitor(fingerprint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT OR IGNORE INTO weekly_visitors (fingerprint) VALUES (?)`, fingerprint)
	return err
}

func (s *Store) WeeklyVisitorCount() (int, error) {
	row := s.db.QueryRow(`SELECT COUNT(*) FROM weekly_visitors`)
	var count int
	err := row.Scan(&count)
	return count, err
}

type ChatRow struct {
	Room        string
	Fingerprint string
	Nickname    string
	ColorIndex  int
	Text        string
	IsSystem    bool
	CreatedAt   time.Time
}

func (s *Store) SaveMessage(room, fingerprint, nickname string, colorIndex int, text string, isSystem bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sys := 0
	if isSystem {
		sys = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO chat_messages (room, fingerprint, nickname, color_index, text, is_system)
		VALUES (?, ?, ?, ?, ?, ?)
	`, room, fingerprint, nickname, colorIndex, text, sys)
	return err
}

func (s *Store) RecentMessages(room string, limit int) ([]ChatRow, error) {
	rows, err := s.db.Query(`
		SELECT room, fingerprint, nickname, color_index, text, is_system, created_at
		FROM chat_messages
		WHERE room = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, room, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []ChatRow
	for rows.Next() {
		var m ChatRow
		var sys int
		var ts string
		if err := rows.Scan(&m.Room, &m.Fingerprint, &m.Nickname, &m.ColorIndex, &m.Text, &sys, &ts); err != nil {
			continue
		}
		m.IsSystem = sys != 0
		m.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", ts)
		msgs = append(msgs, m)
	}
	// Reverse so oldest first
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

func (s *Store) PurgeAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.Exec(`DELETE FROM users`)
	tx.Exec(`DELETE FROM weekly_visitors`)
	tx.Exec(`DELETE FROM chat_messages`)
	return tx.Commit()
}
