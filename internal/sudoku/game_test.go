package sudoku

import "testing"

// helper: create a game and find one clue cell and one empty cell.
func clueAndEmpty(g *Game) (clueR, clueC, emptyR, emptyC int) {
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if g.board[r][c].IsClue {
				clueR, clueC = r, c
			}
			if g.board[r][c].Value == 0 {
				emptyR, emptyC = r, c
			}
		}
	}
	return
}

func TestNewGame(t *testing.T) {
	g := NewGame("easy")

	// Board must have clues.
	clues := 0
	board := g.Board()
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if board[r][c].IsClue {
				clues++
			}
		}
	}
	if clues == 0 {
		t.Fatal("expected clues on the board")
	}

	// Scores should be empty.
	if len(g.Scores()) != 0 {
		t.Fatal("expected empty scores")
	}

	// Difficulty stored.
	if g.Difficulty() != "easy" {
		t.Fatalf("expected easy, got %s", g.Difficulty())
	}
}

func TestPlaceCorrect(t *testing.T) {
	g := NewGame("easy")

	// Find an empty cell and place the correct value.
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if g.board[r][c].Value == 0 {
				correct := g.solution[r][c]
				pts := g.Place("player1", r, c, correct)
				if pts != 1 {
					t.Fatalf("expected +1, got %d", pts)
				}
				if g.Score("player1") != 1 {
					t.Fatalf("expected score 1, got %d", g.Score("player1"))
				}
				return
			}
		}
	}
	t.Fatal("no empty cell found")
}

func TestPlaceWrong(t *testing.T) {
	g := NewGame("easy")

	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if g.board[r][c].Value == 0 {
				correct := g.solution[r][c]
				wrong := correct%9 + 1 // guaranteed != correct
				pts := g.Place("player1", r, c, wrong)
				if pts != -1 {
					t.Fatalf("expected -1, got %d", pts)
				}
				if g.Score("player1") != -1 {
					t.Fatalf("expected score -1, got %d", g.Score("player1"))
				}
				return
			}
		}
	}
	t.Fatal("no empty cell found")
}

func TestPlaceOnClue(t *testing.T) {
	g := NewGame("easy")

	clueR, clueC, _, _ := clueAndEmpty(g)
	original := g.board[clueR][clueC]
	pts := g.Place("player1", clueR, clueC, 5)
	if pts != 0 {
		t.Fatalf("expected 0 on clue, got %d", pts)
	}
	// Cell must be unchanged.
	if g.board[clueR][clueC] != original {
		t.Fatal("clue cell was modified")
	}
}

func TestClearOwnPlacement(t *testing.T) {
	g := NewGame("easy")

	_, _, emptyR, emptyC := clueAndEmpty(g)
	// Place a wrong number (correct placements are locked and can't be cleared)
	wrongVal := g.solution[emptyR][emptyC]%9 + 1
	if wrongVal == g.solution[emptyR][emptyC] {
		wrongVal = wrongVal%9 + 1
	}
	g.Place("player1", emptyR, emptyC, wrongVal)

	ok := g.Clear("player1", emptyR, emptyC)
	if !ok {
		t.Fatal("expected clear to succeed")
	}
	if g.board[emptyR][emptyC].Value != 0 {
		t.Fatal("cell not cleared")
	}
}

func TestCorrectPlacementIsLocked(t *testing.T) {
	g := NewGame("easy")

	_, _, emptyR, emptyC := clueAndEmpty(g)
	g.Place("player1", emptyR, emptyC, g.solution[emptyR][emptyC])

	// Can't clear a locked cell
	if g.Clear("player1", emptyR, emptyC) {
		t.Fatal("should not be able to clear locked cell")
	}
	// Can't overwrite a locked cell
	pts := g.Place("player1", emptyR, emptyC, g.solution[emptyR][emptyC]%9+1)
	if pts != 0 {
		t.Fatal("should not score on locked cell")
	}
}

func TestClearOtherPlacement(t *testing.T) {
	g := NewGame("easy")

	_, _, emptyR, emptyC := clueAndEmpty(g)
	g.Place("player1", emptyR, emptyC, g.solution[emptyR][emptyC])

	ok := g.Clear("player2", emptyR, emptyC)
	if ok {
		t.Fatal("should not clear another player's placement")
	}
	if g.board[emptyR][emptyC].Value == 0 {
		t.Fatal("cell was wrongly cleared")
	}
}

func TestCheckCost(t *testing.T) {
	g := NewGame("easy")

	// Give player 5 points by placing correct values.
	placed := 0
	for r := 0; r < 9 && placed < 5; r++ {
		for c := 0; c < 9 && placed < 5; c++ {
			if g.board[r][c].Value == 0 {
				g.Place("checker", r, c, g.solution[r][c])
				placed++
			}
		}
	}
	if g.Score("checker") < 3 {
		t.Fatalf("setup failed: score %d", g.Score("checker"))
	}

	scoreBefore := g.Score("checker")
	wrong, ok := g.Check("checker")
	if !ok {
		t.Fatal("check should succeed with enough points")
	}
	scoreAfter := g.Score("checker")
	if scoreAfter != scoreBefore-3 {
		t.Fatalf("expected score %d, got %d", scoreBefore-3, scoreAfter)
	}
	// All placements are correct, so no wrong positions.
	if len(wrong) != 0 {
		t.Fatalf("expected 0 wrong, got %d", len(wrong))
	}
}

func TestCheckInsufficientPoints(t *testing.T) {
	g := NewGame("easy")

	// Player has 0 points, check should fail.
	_, ok := g.Check("broke")
	if ok {
		t.Fatal("check should fail with insufficient points")
	}
}

func TestCheckPenalizesWrongPlacers(t *testing.T) {
	g := NewGame("easy")

	// Place a wrong value by player "badguy".
	var wrongR, wrongC int
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if g.board[r][c].Value == 0 {
				correct := g.solution[r][c]
				wrong := correct%9 + 1
				g.Place("badguy", r, c, wrong)
				wrongR, wrongC = r, c
				goto done
			}
		}
	}
done:

	// Give "checker" enough points (place 4 correct).
	placed := 0
	for r := 0; r < 9 && placed < 4; r++ {
		for c := 0; c < 9 && placed < 4; c++ {
			if g.board[r][c].Value == 0 {
				g.Place("checker", r, c, g.solution[r][c])
				placed++
			}
		}
	}
	if g.Score("checker") < 3 {
		t.Fatalf("setup failed: checker score %d", g.Score("checker"))
	}

	badBefore := g.Score("badguy")
	wrong, ok := g.Check("checker")
	if !ok {
		t.Fatal("check should succeed")
	}

	// Exactly one wrong position at (wrongR, wrongC).
	found := false
	for _, p := range wrong {
		if p.Row == wrongR && p.Col == wrongC {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected wrong position (%d,%d) in results", wrongR, wrongC)
	}

	// Badguy should have lost 1 more point.
	if g.Score("badguy") != badBefore-1 {
		t.Fatalf("expected badguy score %d, got %d", badBefore-1, g.Score("badguy"))
	}
}

func TestIsSolved(t *testing.T) {
	g := NewGame("easy")

	if g.IsSolved() {
		t.Fatal("should not be solved at start")
	}

	// Fill every empty cell with the correct value.
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if g.board[r][c].Value == 0 {
				g.Place("solver", r, c, g.solution[r][c])
			}
		}
	}

	if !g.IsSolved() {
		t.Fatal("should be solved after filling all cells correctly")
	}
}
