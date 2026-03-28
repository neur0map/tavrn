package jukebox

import (
	"testing"
	"time"
)

func newTestEngine() *Engine {
	return NewEngineWithCatalog(NewCatalog())
}

func TestEngineInitialState(t *testing.T) {
	e := newTestEngine()
	state := e.State()
	if state.Current != nil {
		t.Error("expected no current track before first tick")
	}
}

func TestEngineAutoPicksOnFirstTick(t *testing.T) {
	e := newTestEngine()
	e.tick()
	state := e.State()
	if state.Current == nil {
		t.Error("expected a track after first tick")
	}
}

func TestEngineAutoNextOnTrackEnd(t *testing.T) {
	e := newTestEngine()
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
	e := newTestEngine()
	e.tick()

	state := e.State()
	if state.Current == nil {
		t.Fatal("expected a track")
	}
	if state.Current.Duration != 0 {
		t.Errorf("expected duration 0, got %d", state.Current.Duration)
	}

	firstID := state.Current.ID
	e.tick()
	if e.State().Current.ID != firstID {
		t.Error("should not change track while duration is unknown")
	}
}

func TestEngineUpdateDuration(t *testing.T) {
	e := newTestEngine()
	e.tick()

	e.UpdateDuration(200)
	if e.State().Current.Duration != 200 {
		t.Errorf("duration = %d, want 200", e.State().Current.Duration)
	}
}

func TestEngineListeners(t *testing.T) {
	e := newTestEngine()
	e.SetOnlineCount(func() int { return 7 })
	state := e.State()
	if state.Listeners != 7 {
		t.Errorf("listeners = %d, want 7", state.Listeners)
	}
}

func TestEngineTrackChangeCallback(t *testing.T) {
	e := newTestEngine()
	called := false
	e.SetOnTrackChange(func(track Track) {
		called = true
	})
	e.tick()
	if !called {
		t.Error("expected onTrackChange to be called on first tick")
	}
}

func TestEngineDefaultGenreIsLofi(t *testing.T) {
	e := newTestEngine()
	state := e.State()
	if state.ActiveGenre.String() != "Lofi" {
		t.Errorf("default genre = %s, want Lofi", state.ActiveGenre)
	}
}

func TestEngineSetGenrePending(t *testing.T) {
	e := newTestEngine()
	e.SetGenre(Genre(1)) // Jazz
	state := e.State()
	if state.PendingGenre.String() != "Jazz" {
		t.Errorf("pending genre = %s, want Jazz", state.PendingGenre)
	}
	if state.ActiveGenre.String() != "Lofi" {
		t.Errorf("active genre should still be Lofi before track change, got %s", state.ActiveGenre)
	}
}

func TestEngineGenreSwitchOnNextTrack(t *testing.T) {
	e := newTestEngine()
	e.tick()

	e.SetGenre(Genre(1)) // Jazz

	e.mu.Lock()
	e.current.Duration = 1
	e.playStart = time.Now().Add(-2 * time.Second)
	e.mu.Unlock()

	e.tick()

	state := e.State()
	if state.ActiveGenre.String() != "Jazz" {
		t.Errorf("active genre = %s, want Jazz after track end", state.ActiveGenre)
	}
	if state.Current == nil {
		t.Fatal("expected a track after genre switch")
	}
}
