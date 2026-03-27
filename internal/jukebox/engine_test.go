package jukebox

import (
	"testing"
	"time"
)

func TestEngineInitialState(t *testing.T) {
	e := NewEngine(nil)
	state := e.State()
	if state.Phase != PhaseIdle {
		t.Errorf("expected PhaseIdle, got %v", state.Phase)
	}
	if state.Current != nil {
		t.Errorf("expected no current track")
	}
}

func TestEngineAddRequest(t *testing.T) {
	e := NewEngine(nil)
	e.mu.Lock()
	e.phase = PhasePlaying
	e.mu.Unlock()

	track := Track{ID: "1", Title: "Test", Artist: "Artist", Source: "jamendo"}
	e.AddRequest("user1", track)

	state := e.State()
	if len(state.Requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(state.Requests))
	}
	if state.Requests[0].Count != 1 {
		t.Errorf("expected count 1, got %d", state.Requests[0].Count)
	}
}

func TestEngineAddRequestDuplicate(t *testing.T) {
	e := NewEngine(nil)
	e.mu.Lock()
	e.phase = PhasePlaying
	e.mu.Unlock()

	track := Track{ID: "1", Title: "Test", Artist: "Artist", Source: "jamendo"}
	e.AddRequest("user1", track)
	e.AddRequest("user2", track)

	state := e.State()
	if len(state.Requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(state.Requests))
	}
	if state.Requests[0].Count != 2 {
		t.Errorf("expected count 2, got %d", state.Requests[0].Count)
	}
}

func TestEngineAddRequestRejectsIdle(t *testing.T) {
	e := NewEngine(nil)
	// Phase is idle by default
	track := Track{ID: "1", Title: "Test"}
	e.AddRequest("user1", track)

	state := e.State()
	if len(state.Requests) != 0 {
		t.Errorf("expected 0 requests in idle phase, got %d", len(state.Requests))
	}
}

func TestEngineVote(t *testing.T) {
	e := NewEngine(nil)
	track := Track{ID: "1", Title: "Test", Artist: "Artist", Source: "jamendo"}

	e.mu.Lock()
	e.phase = PhasePlaying
	e.requestPool["1"] = &Request{Track: track, Count: 1, Voters: make(map[string]bool)}
	e.mu.Unlock()

	ok := e.Vote("user1", "1")
	if !ok {
		t.Error("expected vote to succeed")
	}

	// Same user, same track — should fail
	ok = e.Vote("user1", "1")
	if ok {
		t.Error("expected duplicate vote to fail")
	}

	// Check vote counted
	state := e.State()
	if state.Requests[0].Votes != 1 {
		t.Errorf("expected 1 vote, got %d", state.Requests[0].Votes)
	}
}

func TestEngineVoteSwitching(t *testing.T) {
	e := NewEngine(nil)
	trackA := Track{ID: "a", Title: "A"}
	trackB := Track{ID: "b", Title: "B"}

	e.mu.Lock()
	e.phase = PhasePlaying
	e.requestPool["a"] = &Request{Track: trackA, Count: 1, Voters: make(map[string]bool)}
	e.requestPool["b"] = &Request{Track: trackB, Count: 1, Voters: make(map[string]bool)}
	e.mu.Unlock()

	e.Vote("user1", "a")
	if e.UserVotedFor("user1") != "a" {
		t.Error("expected user1 voted for a")
	}

	// Switch vote to b
	ok := e.Vote("user1", "b")
	if !ok {
		t.Error("expected vote switch to succeed")
	}
	if e.UserVotedFor("user1") != "b" {
		t.Error("expected user1 voted for b after switch")
	}

	state := e.State()
	for _, r := range state.Requests {
		if r.Track.ID == "a" && r.Votes != 0 {
			t.Errorf("expected 0 votes on a after switch, got %d", r.Votes)
		}
		if r.Track.ID == "b" && r.Votes != 1 {
			t.Errorf("expected 1 vote on b after switch, got %d", r.Votes)
		}
	}
}

func TestEngineVoteNotInPool(t *testing.T) {
	e := NewEngine(nil)
	e.mu.Lock()
	e.phase = PhasePlaying
	e.mu.Unlock()

	ok := e.Vote("user1", "nonexistent")
	if ok {
		t.Error("expected vote for nonexistent track to fail")
	}
}

func TestEnginePickWinner(t *testing.T) {
	e := NewEngine(nil)

	e.mu.Lock()
	e.phase = PhasePlaying
	e.requestPool = map[string]*Request{
		"a": {Track: Track{ID: "a", Title: "A"}, Count: 1, Votes: 1, Voters: map[string]bool{"u1": true}},
		"b": {Track: Track{ID: "b", Title: "B"}, Count: 1, Votes: 2, Voters: map[string]bool{"u2": true, "u3": true}},
	}
	e.mu.Unlock()

	winner := e.FinishTrack()
	if winner == nil {
		t.Fatal("expected a winner")
	}
	if winner.ID != "b" {
		t.Errorf("expected winner 'b' (2 votes), got '%s'", winner.ID)
	}
}

func TestEngineTickTrackEnds(t *testing.T) {
	e := NewEngine(nil)

	track := Track{ID: "1", Title: "Test", Duration: 100}
	e.mu.Lock()
	e.current = &track
	e.playStart = time.Now().Add(-101 * time.Second)
	e.phase = PhasePlaying
	e.mu.Unlock()

	e.tick()

	state := e.State()
	// No backends, no requests — should go idle
	if state.Phase != PhaseIdle {
		t.Errorf("expected PhaseIdle after track ends with no requests/backends, got %v", state.Phase)
	}
}

func TestEngineTickWithWinner(t *testing.T) {
	e := NewEngine(nil)

	track := Track{ID: "1", Title: "Current", Duration: 100}
	nextTrack := Track{ID: "2", Title: "Next"}

	e.mu.Lock()
	e.current = &track
	e.playStart = time.Now().Add(-101 * time.Second)
	e.phase = PhasePlaying
	e.requestPool = map[string]*Request{
		"2": {Track: nextTrack, Count: 1, Votes: 1, Voters: map[string]bool{"u1": true}},
	}
	e.mu.Unlock()

	e.tick()

	state := e.State()
	if state.Phase != PhasePlaying {
		t.Errorf("expected PhasePlaying after winner picked, got %v", state.Phase)
	}
	if state.Current == nil || state.Current.ID != "2" {
		t.Error("expected current track to be the winner")
	}
}

func TestEngineFinishTrack(t *testing.T) {
	e := NewEngine(nil)
	track := Track{ID: "1", Title: "Test"}
	nextTrack := Track{ID: "2", Title: "Next"}

	e.mu.Lock()
	e.current = &track
	e.phase = PhasePlaying
	e.requestPool = map[string]*Request{
		"2": {Track: nextTrack, Count: 1, Votes: 1, Voters: map[string]bool{"u1": true}},
	}
	e.mu.Unlock()

	winner := e.FinishTrack()
	if winner == nil || winner.ID != "2" {
		t.Errorf("expected winner '2', got %v", winner)
	}

	state := e.State()
	if state.Phase != PhasePlaying {
		t.Errorf("expected PhasePlaying after finish, got %v", state.Phase)
	}
	if state.Current == nil || state.Current.ID != "2" {
		t.Error("expected current track to be the winner")
	}
}
