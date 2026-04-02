package wargame

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"sync"
)

// Store manages wargame flags and user progress.
type Store struct {
	db *sql.DB
	mu sync.Mutex
}

// LeaderboardEntry represents a user on the leaderboard.
type LeaderboardEntry struct {
	Fingerprint string
	Nickname    string
	TotalLevel  int
	TotalPoints int
}

// Progress represents a user's progress in a single wargame.
type Progress struct {
	Wargame  string
	Level    int
	MaxLevel int
	Points   int
}

// New creates a wargame store using the given DB connection.
func New(db *sql.DB) *Store {
	s := &Store{db: db}
	s.migrate()
	return s
}

func (s *Store) migrate() {
	s.db.Exec(`
		CREATE TABLE IF NOT EXISTS wargame_flags (
			wargame TEXT NOT NULL,
			level   INTEGER NOT NULL,
			flag    TEXT NOT NULL,
			PRIMARY KEY (wargame, level)
		)
	`)
	s.db.Exec(`
		CREATE TABLE IF NOT EXISTS wargame_progress (
			fingerprint TEXT NOT NULL,
			wargame     TEXT NOT NULL,
			level       INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (fingerprint, wargame)
		)
	`)
	s.db.Exec(`
		CREATE TABLE IF NOT EXISTS wargame_participants (
			fingerprint TEXT PRIMARY KEY,
			signed_up   DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
}

// Signup registers a user as a wargame participant.
func (s *Store) Signup(fingerprint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.db.Exec(`INSERT OR IGNORE INTO wargame_participants (fingerprint) VALUES (?)`, fingerprint)
}

// IsParticipant checks if a user has signed up for wargames.
func (s *Store) IsParticipant(fingerprint string) bool {
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM wargame_participants WHERE fingerprint = ?`, fingerprint).Scan(&count)
	return count > 0
}

// SetFlag sets the flag for a wargame level (admin only).
func (s *Store) SetFlag(wargame string, level int, flag string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Store hashed flag so plaintext is never in DB
	hash := hashFlag(flag)
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO wargame_flags (wargame, level, flag)
		VALUES (?, ?, ?)
	`, wargame, level, hash)
	return err
}

// ListFlags returns all levels with flags for a wargame.
func (s *Store) ListFlags(wargame string) []int {
	rows, err := s.db.Query(`
		SELECT level FROM wargame_flags
		WHERE wargame = ?
		ORDER BY level ASC
	`, wargame)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var levels []int
	for rows.Next() {
		var l int
		if rows.Scan(&l) == nil {
			levels = append(levels, l)
		}
	}
	return levels
}

// MaxLevel returns the highest level with a flag for a wargame.
func (s *Store) MaxLevel(wargame string) int {
	var max int
	s.db.QueryRow(`
		SELECT COALESCE(MAX(level), 0) FROM wargame_flags
		WHERE wargame = ?
	`, wargame).Scan(&max)
	return max
}

// SubmitFlag checks a flag submission. Returns (success, newLevel, error).
func (s *Store) SubmitFlag(fingerprint, wargame, flag string) (bool, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get current level
	currentLevel := 0
	s.db.QueryRow(`
		SELECT COALESCE(level, 0) FROM wargame_progress
		WHERE fingerprint = ? AND wargame = ?
	`, fingerprint, wargame).Scan(&currentLevel)

	nextLevel := currentLevel + 1

	// Get expected flag hash for next level
	var expectedHash string
	err := s.db.QueryRow(`
		SELECT flag FROM wargame_flags
		WHERE wargame = ? AND level = ?
	`, wargame, nextLevel).Scan(&expectedHash)
	if err != nil {
		return false, currentLevel, fmt.Errorf("no flag set for %s level %d", wargame, nextLevel)
	}

	// Verify
	if hashFlag(flag) != expectedHash {
		return false, currentLevel, nil
	}

	// Update progress
	s.db.Exec(`
		INSERT INTO wargame_progress (fingerprint, wargame, level)
		VALUES (?, ?, ?)
		ON CONFLICT (fingerprint, wargame)
		DO UPDATE SET level = ?
	`, fingerprint, wargame, nextLevel, nextLevel)

	return true, nextLevel, nil
}

// UserProgress returns progress across all wargames for a user.
func (s *Store) UserProgress(fingerprint string) []Progress {
	// Get all wargames that have flags
	wargames := s.allWargames()

	var progress []Progress
	for _, wg := range wargames {
		level := 0
		s.db.QueryRow(`
			SELECT COALESCE(level, 0) FROM wargame_progress
			WHERE fingerprint = ? AND wargame = ?
		`, fingerprint, wg).Scan(&level)

		maxLevel := s.MaxLevel(wg)
		pts := triangularSum(level)
		progress = append(progress, Progress{
			Wargame:  wg,
			Level:    level,
			MaxLevel: maxLevel,
			Points:   pts,
		})
	}
	return progress
}

// UserTotalLevel returns total levels cleared across all wargames.
func (s *Store) UserTotalLevel(fingerprint string) int {
	var total int
	s.db.QueryRow(`
		SELECT COALESCE(SUM(level), 0) FROM wargame_progress
		WHERE fingerprint = ?
	`, fingerprint).Scan(&total)
	return total
}

// UserTotalPoints returns total points across all wargames.
func (s *Store) UserTotalPoints(fingerprint string) int {
	rows, err := s.db.Query(`
		SELECT level FROM wargame_progress
		WHERE fingerprint = ?
	`, fingerprint)
	if err != nil {
		return 0
	}
	defer rows.Close()
	total := 0
	for rows.Next() {
		var level int
		if rows.Scan(&level) == nil {
			total += triangularSum(level)
		}
	}
	return total
}

// Leaderboard returns top N participants by total points.
// Includes all signed-up participants, even those with 0 points.
func (s *Store) Leaderboard(limit int) []LeaderboardEntry {
	rows, err := s.db.Query(`
		SELECT p.fingerprint, COALESCE(u.nickname, p.fingerprint),
			   COALESCE(SUM(wp.level), 0) as total_level
		FROM wargame_participants p
		LEFT JOIN users u ON u.fingerprint = p.fingerprint
		LEFT JOIN wargame_progress wp ON wp.fingerprint = p.fingerprint
		GROUP BY p.fingerprint
		ORDER BY total_level DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var entries []LeaderboardEntry
	for rows.Next() {
		var e LeaderboardEntry
		if rows.Scan(&e.Fingerprint, &e.Nickname, &e.TotalLevel) == nil {
			e.TotalPoints = s.UserTotalPoints(e.Fingerprint)
			entries = append(entries, e)
		}
	}
	return entries
}

func (s *Store) allWargames() []string {
	rows, err := s.db.Query(`
		SELECT DISTINCT wargame FROM wargame_flags
		ORDER BY wargame ASC
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var wgs []string
	for rows.Next() {
		var wg string
		if rows.Scan(&wg) == nil {
			wgs = append(wgs, wg)
		}
	}
	return wgs
}

// triangularSum returns 1+2+3+...+n
func triangularSum(n int) int {
	return n * (n + 1) / 2
}

func hashFlag(flag string) string {
	h := sha256.Sum256([]byte(flag))
	return hex.EncodeToString(h[:])
}
