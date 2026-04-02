package wargame

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", t.TempDir()+"/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return New(db)
}

func TestSetAndSubmitFlag(t *testing.T) {
	s := tempStore(t)
	s.SetFlag("bandit", 1, "password123")

	ok, level, err := s.SubmitFlag("fp1", "bandit", "password123")
	if err != nil {
		t.Fatalf("SubmitFlag error: %v", err)
	}
	if !ok || level != 1 {
		t.Errorf("expected ok=true level=1, got ok=%v level=%d", ok, level)
	}
}

func TestWrongFlag(t *testing.T) {
	s := tempStore(t)
	s.SetFlag("bandit", 1, "password123")

	ok, level, _ := s.SubmitFlag("fp1", "bandit", "wrong")
	if ok {
		t.Error("expected wrong flag to fail")
	}
	if level != 0 {
		t.Errorf("level should still be 0, got %d", level)
	}
}

func TestProgressiveSubmission(t *testing.T) {
	s := tempStore(t)
	s.SetFlag("bandit", 1, "flag1")
	s.SetFlag("bandit", 2, "flag2")
	s.SetFlag("bandit", 3, "flag3")

	// Can't skip to level 2
	ok, _, _ := s.SubmitFlag("fp1", "bandit", "flag2")
	if ok {
		t.Error("shouldn't be able to skip levels")
	}

	// Clear level 1
	ok, level, _ := s.SubmitFlag("fp1", "bandit", "flag1")
	if !ok || level != 1 {
		t.Error("level 1 should pass")
	}

	// Now clear level 2
	ok, level, _ = s.SubmitFlag("fp1", "bandit", "flag2")
	if !ok || level != 2 {
		t.Error("level 2 should pass")
	}
}

func TestTriangularPoints(t *testing.T) {
	tests := []struct {
		n    int
		want int
	}{
		{0, 0},
		{1, 1},
		{5, 15},
		{10, 55},
	}
	for _, tt := range tests {
		got := triangularSum(tt.n)
		if got != tt.want {
			t.Errorf("triangularSum(%d) = %d, want %d", tt.n, got, tt.want)
		}
	}
}

func TestUserTotalLevel(t *testing.T) {
	s := tempStore(t)
	s.SetFlag("bandit", 1, "b1")
	s.SetFlag("natas", 1, "n1")
	s.SetFlag("natas", 2, "n2")

	s.SubmitFlag("fp1", "bandit", "b1")
	s.SubmitFlag("fp1", "natas", "n1")
	s.SubmitFlag("fp1", "natas", "n2")

	total := s.UserTotalLevel("fp1")
	if total != 3 {
		t.Errorf("total level = %d, want 3", total)
	}
}

func TestUserTotalPoints(t *testing.T) {
	s := tempStore(t)
	s.SetFlag("bandit", 1, "b1")
	s.SetFlag("bandit", 2, "b2")
	s.SetFlag("natas", 1, "n1")

	s.SubmitFlag("fp1", "bandit", "b1")
	s.SubmitFlag("fp1", "bandit", "b2")
	s.SubmitFlag("fp1", "natas", "n1")

	// bandit level 2 = 1+2 = 3 pts, natas level 1 = 1 pt, total = 4
	pts := s.UserTotalPoints("fp1")
	if pts != 4 {
		t.Errorf("total points = %d, want 4", pts)
	}
}

func TestLeaderboard(t *testing.T) {
	s := tempStore(t)
	// Need users table for nickname join
	s.db.Exec(`CREATE TABLE IF NOT EXISTS users (fingerprint TEXT PRIMARY KEY, nickname TEXT)`)
	s.db.Exec(`INSERT INTO users VALUES ('fp1', 'alice')`)
	s.db.Exec(`INSERT INTO users VALUES ('fp2', 'bob')`)

	s.Signup("fp1")
	s.Signup("fp2")

	s.SetFlag("bandit", 1, "b1")
	s.SetFlag("bandit", 2, "b2")
	s.SetFlag("bandit", 3, "b3")

	s.SubmitFlag("fp1", "bandit", "b1")
	s.SubmitFlag("fp1", "bandit", "b2")
	s.SubmitFlag("fp1", "bandit", "b3")
	s.SubmitFlag("fp2", "bandit", "b1")

	lb := s.Leaderboard(10)
	if len(lb) != 2 {
		t.Fatalf("leaderboard entries = %d, want 2", len(lb))
	}
	if lb[0].Nickname != "alice" || lb[0].TotalLevel != 3 {
		t.Errorf("first = %+v", lb[0])
	}
	if lb[1].Nickname != "bob" || lb[1].TotalLevel != 1 {
		t.Errorf("second = %+v", lb[1])
	}
}

func TestMaxLevel(t *testing.T) {
	s := tempStore(t)
	s.SetFlag("bandit", 1, "a")
	s.SetFlag("bandit", 5, "b")
	s.SetFlag("bandit", 10, "c")

	max := s.MaxLevel("bandit")
	if max != 10 {
		t.Errorf("max = %d, want 10", max)
	}
}

func TestFlagHashed(t *testing.T) {
	s := tempStore(t)
	s.SetFlag("bandit", 1, "secret")

	// Verify the stored value is a hash, not plaintext
	var stored string
	s.db.QueryRow(`SELECT flag FROM wargame_flags WHERE wargame='bandit' AND level=1`).Scan(&stored)
	if stored == "secret" {
		t.Error("flag should be hashed, not stored as plaintext")
	}
	if len(stored) != 64 { // SHA256 hex
		t.Errorf("hash length = %d, want 64", len(stored))
	}
}
