package store

import (
	"testing"
	"time"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	path := t.TempDir() + "/test.db"
	s, err := New(path)
	if err != nil {
		t.Fatalf("New(%q) error: %v", path, err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestMigrate(t *testing.T) {
	s := tempStore(t)
	err := s.UpsertUser("fp1", "nick1")
	if err != nil {
		t.Fatalf("UpsertUser after migrate: %v", err)
	}
}

func TestUpsertAndGetUser(t *testing.T) {
	s := tempStore(t)
	if err := s.UpsertUser("fp1", "alice"); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	u, err := s.GetUser("fp1")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if u == nil {
		t.Fatal("user is nil")
	}
	if u.Nickname != "alice" || u.Fingerprint != "fp1" {
		t.Errorf("got user %+v", u)
	}
}

func TestUpsertIncrementsVisitCount(t *testing.T) {
	s := tempStore(t)
	s.UpsertUser("fp1", "alice")
	s.UpsertUser("fp1", "alice")
	s.UpsertUser("fp1", "alice")
	u, _ := s.GetUser("fp1")
	if u.VisitCount != 3 {
		t.Errorf("visit_count = %d, want 3", u.VisitCount)
	}
}

func TestSetNickname(t *testing.T) {
	s := tempStore(t)
	s.UpsertUser("fp1", "alice")
	if err := s.SetNickname("fp1", "bob"); err != nil {
		t.Fatalf("SetNickname: %v", err)
	}
	u, _ := s.GetUser("fp1")
	if u.Nickname != "bob" {
		t.Errorf("nickname = %q, want bob", u.Nickname)
	}
}

func TestNicknameUnique(t *testing.T) {
	s := tempStore(t)
	s.UpsertUser("fp1", "alice")
	s.UpsertUser("fp2", "bob")
	err := s.SetNickname("fp2", "alice")
	if err == nil {
		t.Error("expected error for duplicate nickname")
	}
}

func TestBanAndIsBanned(t *testing.T) {
	s := tempStore(t)
	future := time.Now().Add(1 * time.Hour)
	if err := s.Ban("fp1", "spam", &future); err != nil {
		t.Fatalf("Ban: %v", err)
	}
	banned, err := s.IsBanned("fp1")
	if err != nil {
		t.Fatalf("IsBanned: %v", err)
	}
	if !banned {
		t.Error("expected fp1 to be banned")
	}
}

func TestExpiredBanNotBanned(t *testing.T) {
	s := tempStore(t)
	past := time.Now().Add(-1 * time.Hour)
	s.Ban("fp1", "spam", &past)
	banned, _ := s.IsBanned("fp1")
	if banned {
		t.Error("expired ban should not count")
	}
}

func TestPermanentBan(t *testing.T) {
	s := tempStore(t)
	s.Ban("fp1", "spam", nil)
	banned, _ := s.IsBanned("fp1")
	if !banned {
		t.Error("permanent ban should count")
	}
}

func TestUnban(t *testing.T) {
	s := tempStore(t)
	future := time.Now().Add(1 * time.Hour)
	s.Ban("fp1", "spam", &future)
	s.Unban("fp1")
	banned, _ := s.IsBanned("fp1")
	if banned {
		t.Error("expected fp1 to be unbanned")
	}
}

func TestRecordVisitorAndCount(t *testing.T) {
	s := tempStore(t)
	s.RecordVisitor("fp1")
	s.RecordVisitor("fp2")
	s.RecordVisitor("fp1") // duplicate
	count, err := s.WeeklyVisitorCount()
	if err != nil {
		t.Fatalf("WeeklyVisitorCount: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestPurgeAll(t *testing.T) {
	s := tempStore(t)
	s.UpsertUser("fp1", "alice")
	s.RecordVisitor("fp1")
	future := time.Now().Add(1 * time.Hour)
	s.Ban("fp1", "test", &future)

	if err := s.PurgeAll(); err != nil {
		t.Fatalf("PurgeAll: %v", err)
	}

	u, _ := s.GetUser("fp1")
	if u != nil {
		t.Error("user should be purged")
	}

	count, _ := s.WeeklyVisitorCount()
	if count != 0 {
		t.Errorf("visitor count = %d, want 0", count)
	}

	banned, _ := s.IsBanned("fp1")
	if !banned {
		t.Error("ban should survive purge")
	}

	// Rooms should survive purge
	rooms := s.AllRooms()
	if len(rooms) < 3 {
		t.Errorf("rooms should survive purge, got %d", len(rooms))
	}
}

func TestDefaultRoomsSeeded(t *testing.T) {
	s := tempStore(t)
	rooms := s.AllRooms()
	expected := map[string]bool{"lounge": true, "gallery": true, "games": true, "suggestions": true}
	for _, r := range rooms {
		if !expected[r] {
			t.Errorf("unexpected room: %q", r)
		}
	}
	if len(rooms) < 4 {
		t.Errorf("expected at least 3 rooms, got %d", len(rooms))
	}
}

func TestAddRoom(t *testing.T) {
	s := tempStore(t)
	if err := s.AddRoom("arena"); err != nil {
		t.Fatalf("AddRoom: %v", err)
	}
	rooms := s.AllRooms()
	found := false
	for _, r := range rooms {
		if r == "arena" {
			found = true
			break
		}
	}
	if !found {
		t.Error("arena not found in rooms")
	}
}

func TestAddRoomDuplicate(t *testing.T) {
	s := tempStore(t)
	s.AddRoom("arena")
	err := s.AddRoom("arena")
	if err != nil {
		t.Errorf("duplicate AddRoom should not error (INSERT OR IGNORE), got: %v", err)
	}
}

func TestIsRoom(t *testing.T) {
	s := tempStore(t)
	if !s.IsRoom("lounge") {
		t.Error("lounge should be a valid room")
	}
	if s.IsRoom("nonexistent") {
		t.Error("nonexistent should not be a valid room")
	}
	s.AddRoom("arena")
	if !s.IsRoom("arena") {
		t.Error("arena should be valid after AddRoom")
	}
}
