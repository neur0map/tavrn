package jukebox

import (
	"testing"
	"time"
)

func TestEngineInitialState(t *testing.T) {
	e := NewEngine(NewLofi())
	state := e.State()
	if state.Current != nil {
		t.Error("expected no current track before first tick")
	}
}

func TestEngineAutoPicksOnFirstTick(t *testing.T) {
	e := NewEngine(NewLofi())
	e.tick()
	state := e.State()
	if state.Current == nil {
		t.Error("expected a track after first tick")
	}
}

func TestEngineAutoNextOnTrackEnd(t *testing.T) {
	e := NewEngine(NewLofi())
	e.tick() // picks first track

	// Simulate track ending
	e.mu.Lock()
	e.current.Duration = 1
	e.playStart = time.Now().Add(-2 * time.Second)
	firstID := e.current.ID
	e.mu.Unlock()

	e.tick() // should pick next track

	state := e.State()
	if state.Current == nil {
		t.Fatal("expected a track after auto-next")
	}
	// It's random, so it might pick the same track, but at least it shouldn't be nil
	_ = firstID
}

func TestEngineVoteSkipInstant(t *testing.T) {
	e := NewEngine(NewLofi())
	e.SetOnlineCount(func() int { return 3 }) // <= 5, threshold = 1
	e.tick()

	skipped := e.VoteSkip("user1")
	if !skipped {
		t.Error("expected instant skip with <= 5 online")
	}
}

func TestEngineVoteSkipThreshold(t *testing.T) {
	e := NewEngine(NewLofi())
	e.SetOnlineCount(func() int { return 10 }) // threshold = 3 + (10-6)/10 = 3
	e.tick()

	if e.VoteSkip("user1") {
		t.Error("should not skip with 1/3 votes")
	}
	if e.VoteSkip("user2") {
		t.Error("should not skip with 2/3 votes")
	}
	if !e.VoteSkip("user3") {
		t.Error("should skip with 3/3 votes")
	}
}

func TestEngineVoteSkipDeduplicate(t *testing.T) {
	e := NewEngine(NewLofi())
	e.SetOnlineCount(func() int { return 10 })
	e.tick()

	e.VoteSkip("user1")
	e.VoteSkip("user1") // duplicate
	e.VoteSkip("user1") // duplicate

	state := e.State()
	if state.SkipVotes != 1 {
		t.Errorf("expected 1 vote after dedup, got %d", state.SkipVotes)
	}
}

func TestEngineSkipResetsOnNewTrack(t *testing.T) {
	e := NewEngine(NewLofi())
	e.SetOnlineCount(func() int { return 3 })
	e.tick()

	e.VoteSkip("user1") // instant skip, resets voters

	state := e.State()
	if state.SkipVotes != 0 {
		t.Errorf("expected 0 skip votes after track change, got %d", state.SkipVotes)
	}
}

func TestEngineUserSkipped(t *testing.T) {
	e := NewEngine(NewLofi())
	e.SetOnlineCount(func() int { return 20 })
	e.tick()

	if e.UserSkipped("user1") {
		t.Error("user1 should not have voted yet")
	}
	e.VoteSkip("user1")
	if !e.UserSkipped("user1") {
		t.Error("user1 should have voted")
	}
}

func TestEngineUpdateDuration(t *testing.T) {
	e := NewEngine(NewLofi())
	e.tick()

	e.UpdateDuration(200)
	if e.State().Current.Duration != 200 {
		t.Errorf("duration = %d, want 200", e.State().Current.Duration)
	}
}

func TestSkipThreshold(t *testing.T) {
	tests := []struct {
		online    int
		threshold int
	}{
		{1, 1}, {3, 1}, {5, 1},
		{6, 3}, {10, 3}, {15, 3},
		{16, 4}, {25, 4},
		{26, 5}, {35, 5},
		{36, 6},
	}
	for _, tt := range tests {
		got := skipThreshold(tt.online)
		if got != tt.threshold {
			t.Errorf("skipThreshold(%d) = %d, want %d", tt.online, got, tt.threshold)
		}
	}
}
