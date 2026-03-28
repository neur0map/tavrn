package sudoku

import "math/rand/v2"

// Board is a 9x9 Sudoku grid. 0 means empty.
type Board [9][9]int

// IsValid checks if placing num at (row,col) is valid.
func IsValid(board *Board, row, col, num int) bool {
	// Check row
	for c := 0; c < 9; c++ {
		if board[row][c] == num {
			return false
		}
	}
	// Check column
	for r := 0; r < 9; r++ {
		if board[r][col] == num {
			return false
		}
	}
	// Check 3x3 box
	boxR, boxC := (row/3)*3, (col/3)*3
	for r := boxR; r < boxR+3; r++ {
		for c := boxC; c < boxC+3; c++ {
			if board[r][c] == num {
				return false
			}
		}
	}
	return true
}

// Solve fills the board using backtracking with randomized digit order.
// Returns true if solved. Mutates the board.
func Solve(board *Board) bool {
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if board[r][c] == 0 {
				digits := [9]int{1, 2, 3, 4, 5, 6, 7, 8, 9}
				rand.Shuffle(9, func(i, j int) { digits[i], digits[j] = digits[j], digits[i] })
				for _, d := range digits {
					if IsValid(board, r, c, d) {
						board[r][c] = d
						if Solve(board) {
							return true
						}
						board[r][c] = 0
					}
				}
				return false
			}
		}
	}
	return true
}

// countSolutions counts solutions up to max using backtracking.
func countSolutions(board *Board, max int) int {
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if board[r][c] == 0 {
				count := 0
				for d := 1; d <= 9; d++ {
					if IsValid(board, r, c, d) {
						board[r][c] = d
						count += countSolutions(board, max-count)
						board[r][c] = 0
						if count >= max {
							return count
						}
					}
				}
				return count
			}
		}
	}
	return 1
}

// HasUniqueSolution returns true if the board has exactly one solution.
func HasUniqueSolution(board *Board) bool {
	copy := *board
	return countSolutions(&copy, 2) == 1
}

// CountFilled returns the number of non-zero cells.
func CountFilled(board *Board) int {
	n := 0
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			if board[r][c] != 0 {
				n++
			}
		}
	}
	return n
}

// Generate creates a puzzle and its solution.
// Difficulty: "easy" (41 clues), "medium" (31 clues), "hard" (26 clues), "evil" (21 clues).
func Generate(difficulty string) (puzzle, solution Board) {
	Solve(&solution)
	puzzle = solution

	// How many cells to remove
	remove := 40
	switch difficulty {
	case "easy":
		remove = 40
	case "medium":
		remove = 50
	case "hard":
		remove = 55
	case "evil":
		remove = 58
	}

	// Randomize removal order
	cells := make([][2]int, 0, 81)
	for r := 0; r < 9; r++ {
		for c := 0; c < 9; c++ {
			cells = append(cells, [2]int{r, c})
		}
	}
	rand.Shuffle(len(cells), func(i, j int) { cells[i], cells[j] = cells[j], cells[i] })

	removed := 0
	for _, cell := range cells {
		if removed >= remove {
			break
		}
		r, c := cell[0], cell[1]
		old := puzzle[r][c]
		if old == 0 {
			continue
		}
		puzzle[r][c] = 0
		if HasUniqueSolution(&puzzle) {
			removed++
		} else {
			puzzle[r][c] = old
		}
	}
	return puzzle, solution
}
