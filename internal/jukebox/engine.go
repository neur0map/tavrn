package jukebox

import (
	"context"
	"math/rand/v2"
	"sort"
	"sync"
	"time"
)

type Phase int

const (
	PhaseIdle    Phase = iota
	PhasePlaying
)

type Request struct {
	Track     Track
	Count     int       // how many users requested this
	Votes     int       // how many votes for next
	Voters    map[string]bool
	FirstTime time.Time
}

type EngineState struct {
	Phase     Phase
	Current   *Track
	Position  time.Duration
	Requests  []Request // sorted by votes desc, then count desc
	Listeners int
}

type OnStateChange func()

type Engine struct {
	mu            sync.RWMutex
	phase         Phase
	current       *Track
	playStart     time.Time
	requestPool   map[string]*Request // keyed by Track.ID
	userVoted     map[string]string   // fingerprint -> trackID they voted for
	backends      []MusicBackend
	onStateChange OnStateChange
	listeners     int
}

func NewEngine(backends []MusicBackend) *Engine {
	return &Engine{
		phase:       PhaseIdle,
		requestPool: make(map[string]*Request),
		userVoted:   make(map[string]string),
		backends:    backends,
	}
}

func (e *Engine) SetOnStateChange(fn OnStateChange) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onStateChange = fn
}

func (e *Engine) SetListeners(n int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.listeners = n
}

func (e *Engine) Backends() []MusicBackend {
	var enabled []MusicBackend
	for _, b := range e.backends {
		if b.Enabled() {
			enabled = append(enabled, b)
		}
	}
	return enabled
}

func (e *Engine) State() EngineState {
	e.mu.RLock()
	defer e.mu.RUnlock()

	state := EngineState{
		Phase:     e.phase,
		Current:   e.current,
		Listeners: e.listeners,
	}

	if e.current != nil {
		state.Position = time.Since(e.playStart)
	}

	// Build sorted requests: votes desc, then count desc, then earliest first
	reqs := make([]Request, 0, len(e.requestPool))
	for _, r := range e.requestPool {
		reqs = append(reqs, Request{
			Track:     r.Track,
			Count:     r.Count,
			Votes:     r.Votes,
			FirstTime: r.FirstTime,
		})
	}
	sort.Slice(reqs, func(i, j int) bool {
		if reqs[i].Votes != reqs[j].Votes {
			return reqs[i].Votes > reqs[j].Votes
		}
		if reqs[i].Count != reqs[j].Count {
			return reqs[i].Count > reqs[j].Count
		}
		return reqs[i].FirstTime.Before(reqs[j].FirstTime)
	})
	state.Requests = reqs

	return state
}

// UserVotedFor returns the track ID the user voted for, or empty string.
func (e *Engine) UserVotedFor(userFP string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.userVoted[userFP]
}

// AddRequest adds a track to the request pool.
func (e *Engine) AddRequest(userFP string, track Track) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.phase == PhaseIdle {
		return
	}

	if existing, ok := e.requestPool[track.ID]; ok {
		existing.Count++
	} else {
		e.requestPool[track.ID] = &Request{
			Track:     track,
			Count:     1,
			Voters:    make(map[string]bool),
			FirstTime: time.Now(),
		}
	}

	e.notifyChange()
}

// Vote casts a vote for a track. Users vote by fingerprint (pub key).
// Returns true if the vote was accepted.
func (e *Engine) Vote(userFP string, trackID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.phase == PhaseIdle {
		return false
	}

	// Check track exists in request pool
	req, ok := e.requestPool[trackID]
	if !ok {
		return false
	}

	// If user already voted for something, remove that vote first
	if oldTrackID, voted := e.userVoted[userFP]; voted {
		if oldTrackID == trackID {
			return false // already voted for this track
		}
		// Switch vote: remove from old track
		if oldReq, ok := e.requestPool[oldTrackID]; ok {
			delete(oldReq.Voters, userFP)
			oldReq.Votes = len(oldReq.Voters)
		}
	}

	// Cast vote
	req.Voters[userFP] = true
	req.Votes = len(req.Voters)
	e.userVoted[userFP] = trackID

	e.notifyChange()
	return true
}

func (e *Engine) StartPlaying(track Track) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.current = &track
	e.playStart = time.Now()
	e.phase = PhasePlaying

	e.notifyChange()
}

func (e *Engine) FinishTrack() *Track {
	e.mu.Lock()
	defer e.mu.Unlock()

	winner := e.pickWinnerLocked()

	// Reset for next round
	e.requestPool = make(map[string]*Request)
	e.userVoted = make(map[string]string)

	if winner != nil {
		e.current = winner
		e.playStart = time.Now()
		e.phase = PhasePlaying
	} else {
		e.phase = PhaseIdle
		e.current = nil
	}

	e.notifyChange()
	return winner
}

func (e *Engine) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.tick()
		}
	}
}

func (e *Engine) tick() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.current == nil || e.phase == PhaseIdle {
		e.tryAutoPlay()
		return
	}

	elapsed := time.Since(e.playStart)
	duration := time.Duration(e.current.Duration) * time.Second
	if duration == 0 {
		return
	}

	progress := elapsed.Seconds() / duration.Seconds()

	if progress >= 1.0 {
		// Track ended — pick winner
		winner := e.pickWinnerLocked()
		e.requestPool = make(map[string]*Request)
		e.userVoted = make(map[string]string)

		if winner != nil {
			e.current = winner
			e.playStart = time.Now()
			e.phase = PhasePlaying
		} else {
			e.current = nil
			e.phase = PhaseIdle
			e.tryAutoPlay()
		}
		e.notifyChange()
	}
}

func (e *Engine) tryAutoPlay() {
	for _, b := range e.backends {
		if !b.Enabled() {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		tracks, err := b.Search(ctx, "popular", 20)
		cancel()
		if err != nil || len(tracks) == 0 {
			continue
		}
		pick := tracks[rand.IntN(len(tracks))]
		e.current = &pick
		e.playStart = time.Now()
		e.phase = PhasePlaying
		e.notifyChange()
		return
	}
}

// pickWinnerLocked returns the track with the most votes.
// Ties broken by request count, then random.
func (e *Engine) pickWinnerLocked() *Track {
	if len(e.requestPool) == 0 {
		return nil
	}

	type candidate struct {
		track Track
		votes int
		count int
	}
	var candidates []candidate
	for _, r := range e.requestPool {
		candidates = append(candidates, candidate{
			track: r.Track,
			votes: r.Votes,
			count: r.Count,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].votes != candidates[j].votes {
			return candidates[i].votes > candidates[j].votes
		}
		return candidates[i].count > candidates[j].count
	})

	maxVotes := candidates[0].votes
	maxCount := candidates[0].count
	var tied []candidate
	for _, c := range candidates {
		if c.votes == maxVotes && c.count == maxCount {
			tied = append(tied, c)
		}
	}

	winner := tied[rand.IntN(len(tied))]
	return &winner.track
}

func (e *Engine) notifyChange() {
	if e.onStateChange != nil {
		e.onStateChange()
	}
}
