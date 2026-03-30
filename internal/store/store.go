package store

import (
	"database/sql"
	"errors"
	"fmt"
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
	CREATE TABLE IF NOT EXISTS gallery_notes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		x           INTEGER DEFAULT 0,
		y           INTEGER DEFAULT 0,
		text        TEXT NOT NULL,
		fingerprint TEXT NOT NULL,
		nickname    TEXT NOT NULL,
		color_index INTEGER DEFAULT 0,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS rooms (
		name       TEXT PRIMARY KEY,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS owners (
		fingerprint TEXT PRIMARY KEY,
		nickname    TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS banner (
		id         INTEGER PRIMARY KEY CHECK (id = 1),
		text       TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	return s.seedRooms()
}

func (s *Store) seedRooms() error {
	for _, name := range []string{"lounge", "gallery", "games", "suggestions"} {
		s.db.Exec(`INSERT OR IGNORE INTO rooms (name) VALUES (?)`, name)
	}
	return nil
}

// AddOwner registers a permanent owner nickname that survives purges.
func (s *Store) AddOwner(fingerprint, nickname string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT OR REPLACE INTO owners (fingerprint, nickname) VALUES (?, ?)`,
		fingerprint, nickname)
	return err
}

// restoreOwners re-creates owner user rows after a purge.
func (s *Store) restoreOwners() {
	rows, err := s.db.Query(`SELECT fingerprint, nickname FROM owners`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var fp, nick string
		if err := rows.Scan(&fp, &nick); err != nil {
			continue
		}
		s.db.Exec(`INSERT OR IGNORE INTO users (fingerprint, nickname) VALUES (?, ?)`, fp, nick)
	}
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

// FingerprintByNickname looks up a user's fingerprint by their nickname.
func (s *Store) FingerprintByNickname(nickname string) (string, error) {
	row := s.db.QueryRow(`SELECT fingerprint FROM users WHERE nickname = ?`, nickname)
	var fp string
	if err := row.Scan(&fp); err != nil {
		return "", err
	}
	return fp, nil
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

// RecentActivityCounts returns the number of non-system messages per room
// within the last N minutes.
func (s *Store) RecentActivityCounts(minutes int) map[string]int {
	rows, err := s.db.Query(`
		SELECT room, COUNT(*) FROM chat_messages
		WHERE created_at > datetime('now', ? || ' minutes')
		AND is_system = 0
		GROUP BY room
	`, fmt.Sprintf("-%d", minutes))
	if err != nil {
		return nil
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var room string
		var count int
		if err := rows.Scan(&room, &count); err != nil {
			continue
		}
		counts[room] = count
	}
	return counts
}

// ── Gallery Notes ──

type NoteRow struct {
	ID          int
	X, Y        int
	Text        string
	Fingerprint string
	Nickname    string
	ColorIndex  int
	CreatedAt   time.Time
}

func (s *Store) CreateNote(x, y int, text, fingerprint, nickname string, colorIndex int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`
		INSERT INTO gallery_notes (x, y, text, fingerprint, nickname, color_index)
		VALUES (?, ?, ?, ?, ?, ?)
	`, x, y, text, fingerprint, nickname, colorIndex)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return int(id), nil
}

func (s *Store) MoveNote(id, x, y int, fingerprint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE gallery_notes SET x = ?, y = ? WHERE id = ? AND fingerprint = ?`,
		x, y, id, fingerprint)
	return err
}

func (s *Store) DeleteNote(id int, fingerprint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM gallery_notes WHERE id = ? AND fingerprint = ?`,
		id, fingerprint)
	return err
}

func (s *Store) AllNotes() ([]NoteRow, error) {
	rows, err := s.db.Query(`
		SELECT id, x, y, text, fingerprint, nickname, color_index, created_at
		FROM gallery_notes ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var notes []NoteRow
	for rows.Next() {
		var n NoteRow
		var ts string
		if err := rows.Scan(&n.ID, &n.X, &n.Y, &n.Text, &n.Fingerprint, &n.Nickname, &n.ColorIndex, &ts); err != nil {
			continue
		}
		n.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", ts)
		notes = append(notes, n)
	}
	return notes, nil
}

func (s *Store) ClearGallery() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM gallery_notes`)
	return err
}

// ── Rooms ──

func (s *Store) AddRoom(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT OR IGNORE INTO rooms (name) VALUES (?)`, name)
	return err
}

func (s *Store) AllRooms() []string {
	rows, err := s.db.Query(`SELECT name FROM rooms ORDER BY created_at ASC`)
	if err != nil {
		return []string{"lounge", "gallery", "games", "suggestions"}
	}
	defer rows.Close()
	var rooms []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		rooms = append(rooms, name)
	}
	if len(rooms) == 0 {
		return []string{"lounge", "gallery", "games", "suggestions"}
	}
	return rooms
}

func (s *Store) RenameRoom(oldName, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Rename the room
	if _, err := tx.Exec(`UPDATE rooms SET name = ? WHERE name = ?`, newName, oldName); err != nil {
		return err
	}
	// Update chat history to point to new room name
	tx.Exec(`UPDATE chat_messages SET room = ? WHERE room = ?`, newName, oldName)
	return tx.Commit()
}

func (s *Store) DeleteRoom(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.Exec(`DELETE FROM rooms WHERE name = ?`, name)
	tx.Exec(`DELETE FROM chat_messages WHERE room = ?`, name)
	return tx.Commit()
}

func (s *Store) IsRoom(name string) bool {
	row := s.db.QueryRow(`SELECT COUNT(*) FROM rooms WHERE name = ?`, name)
	var count int
	if err := row.Scan(&count); err != nil {
		return false
	}
	return count > 0
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
	tx.Exec(`DELETE FROM gallery_notes`)
	tx.Exec(`DELETE FROM banner`)
	if err := tx.Commit(); err != nil {
		return err
	}
	s.restoreOwners()
	return nil
}

func (s *Store) SetBanner(text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO banner (id, text) VALUES (1, ?) ON CONFLICT(id) DO UPDATE SET text = ?, created_at = CURRENT_TIMESTAMP`, text, text)
	return err
}

func (s *Store) GetBanner() string {
	row := s.db.QueryRow(`SELECT text FROM banner WHERE id = 1`)
	var text string
	if err := row.Scan(&text); err != nil {
		return ""
	}
	return text
}

func (s *Store) ClearBanner() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.Exec(`DELETE FROM banner`)
}
