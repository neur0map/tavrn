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
	e.tick()

	e.mu.Lock()
	e.current.Duration = 1
	e.playStart = time.Now().Add(-2 * time.Second)
	e.mu.Unlock()

	e.tick()

	state := e.State()
	if state.Current == nil {
		t.Fatal("expected a track after auto-next")
	}
}

func TestEngineWaitsForDuration(t *testing.T) {
	e := NewEngine(NewLofi())
	e.tick() // picks first track, duration = 0

	state := e.State()
	if state.Current == nil {
		t.Fatal("expected a track")
	}
	if state.Current.Duration != 0 {
		t.Errorf("expected duration 0, got %d", state.Current.Duration)
	}

	// Tick again — should NOT pick a new track (waiting for ffprobe)
	firstID := state.Current.ID
	e.tick()
	if e.State().Current.ID != firstID {
		t.Error("should not change track while duration is unknown")
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

func TestEngineListeners(t *testing.T) {
	e := NewEngine(NewLofi())
	e.SetOnlineCount(func() int { return 7 })
	state := e.State()
	if state.Listeners != 7 {
		t.Errorf("listeners = %d, want 7", state.Listeners)
	}
}

func TestEngineTrackChangeCallback(t *testing.T) {
	e := NewEngine(NewLofi())
	called := false
	e.SetOnTrackChange(func(track Track) {
		called = true
	})
	e.tick()
	if !called {
		t.Error("expected onTrackChange to be called on first tick")
	}
}
