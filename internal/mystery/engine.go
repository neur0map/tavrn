package mystery

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"tavrn.sh/internal/identity"
)

// Clue is a single piece of evidence returned to the chat.
type Clue struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Result is what Check returns when a message triggers something.
type Result struct {
	Clues      []Clue
	Solved     bool
	Killer     string
	Confession []string
	Sender     string
}

type caseData struct {
	Triggers   map[string][]Clue `json:"triggers"`
	Confession []string          `json:"confession"`
}

// Engine drives the hidden murder mystery in the tavern lounge.
type Engine struct {
	triggers   map[string][]Clue
	found      map[string]bool
	solution   string // SHA256 hex of killer name
	solved     bool
	solvedBy   string
	confession []string
	fakeNames  []string
	killerName string // fake handle for the killer
	lastSender string
	lastClueAt time.Time
	mu         sync.Mutex
}

const (
	fakeNameCount  = 30
	clueCooldown   = 5 * time.Second
	maxCluesPerMsg = 2
)

// New loads a mystery case from caseDir and returns a ready Engine.
func New(caseDir string) (*Engine, error) {
	raw, err := os.ReadFile(filepath.Join(caseDir, "triggers.json"))
	if err != nil {
		return nil, fmt.Errorf("mystery: read triggers: %w", err)
	}

	var cd caseData
	if err := json.Unmarshal(raw, &cd); err != nil {
		return nil, fmt.Errorf("mystery: parse triggers: %w", err)
	}

	sol, err := os.ReadFile(filepath.Join(caseDir, "answer.sha256"))
	if err != nil {
		return nil, fmt.Errorf("mystery: read answer: %w", err)
	}

	e := &Engine{
		triggers:   cd.Triggers,
		found:      make(map[string]bool),
		solution:   strings.TrimSpace(string(sol)),
		confession: cd.Confession,
	}
	e.generateFakeNames()

	log.Printf("mystery: loaded case (%d triggers)", len(e.triggers))
	return e, nil
}

// generateFakeNames populates the fake name pool. The first name becomes
// the killer's handle.
func (e *Engine) generateFakeNames() {
	seen := make(map[string]bool)
	e.fakeNames = make([]string, 0, fakeNameCount)
	for len(e.fakeNames) < fakeNameCount {
		n := identity.RandomNickname()
		if seen[n] {
			continue
		}
		seen[n] = true
		e.fakeNames = append(e.fakeNames, n)
	}
	e.killerName = e.fakeNames[0]
}

// IsActive returns true if the engine is loaded and has triggers.
func (e *Engine) IsActive() bool {
	return e != nil && len(e.triggers) > 0
}

// Reset clears all progress so the case can be replayed.
func (e *Engine) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.found = make(map[string]bool)
	e.solved = false
	e.solvedBy = ""
	e.lastSender = ""
	e.lastClueAt = time.Time{}
	e.generateFakeNames()

	log.Printf("mystery: case reset")
}

// Check examines a chat message for trigger keywords or a solve attempt.
// Returns nil when nothing matches or the engine is inactive.
func (e *Engine) Check(text string) *Result {
	if e == nil {
		return nil
	}

	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	lower := strings.ToLower(trimmed)

	// Check for solve first.
	if !e.solved && e.checkSolve(lower) {
		e.solved = true
		log.Printf("mystery: case solved")
		return &Result{
			Solved:     true,
			Killer:     e.killerName,
			Confession: e.confession,
		}
	}

	// Already solved — no more clues.
	if e.solved {
		return nil
	}

	// Cooldown check.
	if !e.lastClueAt.IsZero() && time.Since(e.lastClueAt) < clueCooldown {
		return nil
	}

	// Split into words and strip punctuation.
	words := strings.Fields(lower)
	for i, w := range words {
		words[i] = stripPunct(w)
	}

	// Collect matching clues.
	var clues []Clue
	for _, w := range words {
		if len(clues) >= maxCluesPerMsg {
			break
		}
		pool, ok := e.triggers[w]
		if !ok {
			continue
		}
		for _, c := range pool {
			if e.found[c.ID] {
				continue
			}
			clues = append(clues, c)
			e.found[c.ID] = true
			if len(clues) >= maxCluesPerMsg {
				break
			}
		}
	}

	if len(clues) == 0 {
		return nil
	}

	e.lastClueAt = time.Now()
	sender := e.pickSender()

	return &Result{
		Clues:  clues,
		Sender: sender,
	}
}

// checkSolve hashes individual words, consecutive pairs, and triples from
// the input and compares against the stored solution hash.
func (e *Engine) checkSolve(lower string) bool {
	words := strings.Fields(lower)
	cleaned := make([]string, len(words))
	for i, w := range words {
		cleaned[i] = stripPunct(w)
	}

	// Individual words.
	for _, w := range cleaned {
		if sha256Hex(w) == e.solution {
			return true
		}
	}

	// Consecutive pairs.
	for i := 0; i+1 < len(cleaned); i++ {
		pair := cleaned[i] + " " + cleaned[i+1]
		if sha256Hex(pair) == e.solution {
			return true
		}
	}

	// Consecutive triples.
	for i := 0; i+2 < len(cleaned); i++ {
		triple := cleaned[i] + " " + cleaned[i+1] + " " + cleaned[i+2]
		if sha256Hex(triple) == e.solution {
			return true
		}
	}

	return false
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}

// stripPunct removes leading and trailing punctuation from a word.
func stripPunct(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		return unicode.IsPunct(r)
	})
}

// pickSender chooses a random fake name that isn't the killer or the last sender.
func (e *Engine) pickSender() string {
	for i := 0; i < 10; i++ {
		n := e.fakeNames[rand.Intn(len(e.fakeNames))]
		if n != e.killerName && n != e.lastSender {
			e.lastSender = n
			return n
		}
	}
	// Fallback: just use any non-killer name.
	e.lastSender = e.fakeNames[1]
	return e.lastSender
}
