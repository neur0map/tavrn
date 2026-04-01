package sudoku

import "testing"

// TestIsValid checks row, column, and box constraint checking.
func TestIsValid(t *testing.T) {
	var b Board

	// Place a 5 at (0,0).
	b[0][0] = 5

	// Same row - should conflict.
	if IsValid(&b, 0, 4, 5) {
		t.Error("expected conflict in the same row")
	}

	// Same column - should conflict.
	if IsValid(&b, 4, 0, 5) {
		t.Error("expected conflict in the same column")
	}

	// Same 3x3 box - should conflict.
	if IsValid(&b, 1, 1, 5) {
		t.Error("expected conflict in the same box")
	}

	// Different row, col, and box - should be fine.
	if !IsValid(&b, 4, 4, 5) {
		t.Error("expected no conflict at (4,4)")
	}

	// Different number at the same row - should be fine.
	if !IsValid(&b, 0, 4, 3) {
		t.Error("expected no conflict for different number in same row")
	}

	// Place at box boundary: (2,2) is still in box(0,0).
	b[2][2] = 7
	if IsValid(&b, 0, 1, 7) {
		t.Error("expected conflict in box for 7 at (0,1)")
	}
	// (0,3) is in box(0,1), different box - no conflict with (2,2).
	if !IsValid(&b, 0, 3, 7) {
		t.Error("expected no conflict: (0,3) is in a different box than (2,2)")
	}
}

// TestSolve verifies the solver on a known partial board.
func TestSolve(t *testing.T) {
	b := Board{
		{5, 3, 0, 0, 7, 0, 0, 0, 0},
		{6, 0, 0, 1, 9, 5, 0, 0, 0},
		{0, 9, 8, 0, 0, 0, 0, 6, 0},
		{8, 0, 0, 0, 6, 0, 0, 0, 3},
		{4, 0, 0, 8, 0, 3, 0, 0, 1},
		{7, 0, 0, 0, 2, 0, 0, 0, 6},
		{0, 6, 0, 0, 0, 0, 2, 8, 0},
		{0, 0, 0, 4, 1, 9, 0, 0, 5},
		{0, 0, 0, 0, 8, 0, 0, 7, 9},
	}

	if !Solve(&b) {
		t.Fatal("expected solver to find a solution")
	}

	// Every cell must be filled 1-9.
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if b[r][c] < 1 || b[r][c] > 9 {
				t.Fatalf("cell (%d,%d) has invalid value %d", r, c, b[r][c])
			}
		}
	}

	if err := validateBoard(&b); err != "" {
		t.Fatalf("solved board is invalid: %s", err)
	}
}

// TestSolveEmpty verifies the solver can solve a completely empty board.
func TestSolveEmpty(t *testing.T) {
	var b Board

	if !Solve(&b) {
		t.Fatal("solver should solve an empty board")
	}

	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if b[r][c] < 1 || b[r][c] > 9 {
				t.Fatalf("cell (%d,%d) has invalid value %d", r, c, b[r][c])
			}
		}
	}

	if err := validateBoard(&b); err != "" {
		t.Fatalf("solved empty board is invalid: %s", err)
	}
}

// TestGenerate generates puzzles for each difficulty and checks clue counts
// and solution validity.
func TestGenerate(t *testing.T) {
	tests := []struct {
		difficulty string
		maxClues   int // 81 - removed
	}{
		{"easy", 41},
		{"medium", 31},
		{"hard", 30},
		{"evil", 30}, // evil targets 58 removals but uniqueness may block some
	}

	for _, tc := range tests {
		t.Run(tc.difficulty, func(t *testing.T) {
			puzzle, solution := Generate(tc.difficulty)

			clues := CountFilled(&puzzle)
			if clues > tc.maxClues {
				t.Errorf("expected at most %d clues for %s, got %d", tc.maxClues, tc.difficulty, clues)
			}

			if err := validateBoard(&solution); err != "" {
				t.Fatalf("solution is invalid: %s", err)
			}

			// Every puzzle clue must match the solution.
			for r := 0; r < 9; r++ {
				for c := 0; c < 9; c++ {
					if puzzle[r][c] != 0 && puzzle[r][c] != solution[r][c] {
						t.Fatalf("clue at (%d,%d) is %d but solution has %d",
							r, c, puzzle[r][c], solution[r][c])
					}
				}
			}
		})
	}
}

// TestHasUniqueSolution verifies that generated puzzles have exactly one
// solution.
func TestHasUniqueSolution(t *testing.T) {
	for _, diff := range []string{"easy", "medium"} {
		t.Run(diff, func(t *testing.T) {
			puzzle, _ := Generate(diff)
			if !HasUniqueSolution(&puzzle) {
				t.Errorf("generated %s puzzle should have a unique solution", diff)
			}
		})
	}
}

// TestSolveSolvesGenerated generates a puzzle, solves a copy, and verifies
// it matches the original solution.
func TestSolveSolvesGenerated(t *testing.T) {
	puzzle, solution := Generate("easy")

	cp := puzzle
	if !Solve(&cp) {
		t.Fatal("solver failed on generated puzzle")
	}

	if cp != solution {
		t.Error("solved puzzle does not match the generated solution")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// validateBoard checks that a fully filled board satisfies all Sudoku rules.
// Returns an empty string on success or a description of the first violation.
func validateBoard(b *Board) string {
	// Check every row.
	for r := 0; r < 9; r++ {
		seen := [10]bool{}
		for c := 0; c < 9; c++ {
			v := b[r][c]
			if v < 1 || v > 9 {
				return "invalid value"
			}
			if seen[v] {
				return "duplicate in row"
			}
			seen[v] = true
		}
	}

	// Check every column.
	for c := 0; c < 9; c++ {
		seen := [10]bool{}
		for r := 0; r < 9; r++ {
			v := b[r][c]
			if seen[v] {
				return "duplicate in column"
			}
			seen[v] = true
		}
	}

	// Check every 3x3 box.
	for br := 0; br < 3; br++ {
		for bc := 0; bc < 3; bc++ {
			seen := [10]bool{}
			for r := br * 3; r < br*3+3; r++ {
				for c := bc * 3; c < bc*3+3; c++ {
					v := b[r][c]
					if seen[v] {
						return "duplicate in box"
					}
					seen[v] = true
				}
			}
		}
	}
	return ""
}
