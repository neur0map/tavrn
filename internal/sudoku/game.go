package sudoku

import (
	"sync"
	"time"
)

// Cell represents a single cell on the shared game board.
type Cell struct {
	Value    int
	PlacedBy string // fingerprint, empty for clues
	IsClue   bool
}

// Position identifies a row/col on the board.
type Position struct {
	Row, Col int
}

// Game holds the shared multiplayer state for a single Sudoku puzzle.
type Game struct {
	mu         sync.RWMutex
	puzzle     Board      // starting state (clues only)
	solution   Board      // answer key
	board      [9][9]Cell // current state
	scores     map[string]int
	cursors    map[string]Position
	started    time.Time
	difficulty string
	onUpdate   func() // called after state changes
}

// NewGame creates a game with a freshly generated puzzle.
func NewGame(difficulty string) *Game {
	puzzle, solution := Generate(difficulty)
	g := &Game{
		puzzle:     puzzle,
		solution:   solution,
		scores:     make(map[string]int),
		cursors:    make(map[string]Position),
		started:    time.Now(),
		difficulty: difficulty,
	}
	// Initialize board from puzzle
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if puzzle[r][c] != 0 {
				g.board[r][c] = Cell{Value: puzzle[r][c], IsClue: true}
			}
		}
	}
	return g
}

// SetOnUpdate registers a callback invoked after every state change.
func (g *Game) SetOnUpdate(fn func()) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.onUpdate = fn
}

// Place puts a number on the board. Returns points earned (+1 or -1).
func (g *Game) Place(fingerprint string, row, col, value int) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	if row < 0 || row > 8 || col < 0 || col > 8 || value < 1 || value > 9 {
		return 0
	}
	if g.board[row][col].IsClue {
		return 0
	}
	g.board[row][col] = Cell{Value: value, PlacedBy: fingerprint}
	points := -1
	if g.solution[row][col] == value {
		points = 1
	}
	g.scores[fingerprint] += points
	g.notify()
	return points
}

// Clear removes a player's own placement.
func (g *Game) Clear(fingerprint string, row, col int) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if row < 0 || row > 8 || col < 0 || col > 8 {
		return false
	}
	cell := g.board[row][col]
	if cell.IsClue || cell.PlacedBy != fingerprint || cell.Value == 0 {
		return false
	}
	g.board[row][col] = Cell{}
	g.notify()
	return true
}

// Check validates the board. Costs the caller 3 points.
// Returns positions of wrong cells. Each wrong cell's placer loses 1 point.
func (g *Game) Check(fingerprint string) (wrong []Position, ok bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.scores[fingerprint] < 3 {
		return nil, false
	}
	g.scores[fingerprint] -= 3
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			cell := g.board[r][c]
			if cell.Value != 0 && !cell.IsClue && cell.Value != g.solution[r][c] {
				wrong = append(wrong, Position{Row: r, Col: c})
				if cell.PlacedBy != "" {
					g.scores[cell.PlacedBy]--
				}
			}
		}
	}
	g.notify()
	return wrong, true
}

// SetCursor updates a player's cursor position.
func (g *Game) SetCursor(fingerprint string, row, col int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.cursors[fingerprint] = Position{Row: row, Col: col}
}

// RemovePlayer removes a player's cursor (on disconnect).
func (g *Game) RemovePlayer(fingerprint string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.cursors, fingerprint)
}

// IsSolved returns true if all cells are correctly filled.
func (g *Game) IsSolved() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if g.board[r][c].Value != g.solution[r][c] {
				return false
			}
		}
	}
	return true
}

// Score returns a player's current score.
func (g *Game) Score(fingerprint string) int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.scores[fingerprint]
}

// Scores returns a copy of all scores.
func (g *Game) Scores() map[string]int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	cp := make(map[string]int, len(g.scores))
	for k, v := range g.scores {
		cp[k] = v
	}
	return cp
}

// Filled returns how many cells have values.
func (g *Game) Filled() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	n := 0
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if g.board[r][c].Value != 0 {
				n++
			}
		}
	}
	return n
}

// Difficulty returns the game difficulty.
func (g *Game) Difficulty() string {
	return g.difficulty
}

// Board returns a copy of the current board state.
func (g *Game) Board() [9][9]Cell {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.board
}

// Cursors returns a copy of cursor positions.
func (g *Game) Cursors() map[string]Position {
	g.mu.RLock()
	defer g.mu.RUnlock()
	cp := make(map[string]Position, len(g.cursors))
	for k, v := range g.cursors {
		cp[k] = v
	}
	return cp
}

// Started returns when the game started.
func (g *Game) Started() time.Time {
	return g.started
}

func (g *Game) notify() {
	if g.onUpdate != nil {
		g.onUpdate()
	}
}
