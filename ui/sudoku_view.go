package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tavrn.sh/internal/chat"
	"tavrn.sh/internal/sudoku"
)

const (
	boardCols    = 37 // rendered board width in chars
	boardRows    = 19 // rendered board height in lines
	checkFlashMs = 5000
)

type SudokuView struct {
	game        *sudoku.Game
	fingerprint string
	colorIndex  int
	cursorRow   int
	cursorCol   int
	focusChat   bool // true = typing in chat, false = board
	input       textinput.Model
	messages    []chat.Message
	wrongCells  map[[2]int]time.Time // flashing wrong cells from check
	width       int
	height      int
}

func NewSudokuView(game *sudoku.Game, fingerprint string, colorIndex int) SudokuView {
	ti := textinput.New()
	ti.Placeholder = "Chat..."
	ti.CharLimit = 200
	ti.Prompt = "  > "

	return SudokuView{
		game:        game,
		fingerprint: fingerprint,
		colorIndex:  colorIndex,
		wrongCells:  make(map[[2]int]time.Time),
		input:       ti,
	}
}

func (s *SudokuView) SetSize(width, height int) {
	s.width = width
	s.height = height
}

func (s *SudokuView) AddMessage(msg chat.Message) {
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	s.messages = append(s.messages, msg)
	// Keep last 50 messages
	if len(s.messages) > 50 {
		s.messages = s.messages[len(s.messages)-50:]
	}
}

func (s *SudokuView) MarkWrong(positions []sudoku.Position) {
	now := time.Now()
	for _, p := range positions {
		s.wrongCells[[2]int{p.Row, p.Col}] = now
	}
}

func (s *SudokuView) Tick() {
	// Expire wrong cell flashes
	now := time.Now()
	for k, t := range s.wrongCells {
		if now.Sub(t) > checkFlashMs*time.Millisecond {
			delete(s.wrongCells, k)
		}
	}
}

func (s SudokuView) Update(msg tea.Msg) (SudokuView, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return s, nil
	}

	key := keyMsg.String()

	// Tab toggles focus
	if key == "tab" {
		s.focusChat = !s.focusChat
		if s.focusChat {
			s.input.Focus()
		} else {
			s.input.Blur()
		}
		return s, nil
	}

	// Chat mode
	if s.focusChat {
		switch key {
		case "enter":
			// Return value handled by app
		case "esc":
			s.focusChat = false
			s.input.Blur()
			return s, nil
		default:
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			return s, cmd
		}
		return s, nil
	}

	// Board mode
	switch key {
	case "up", "k":
		if s.cursorRow > 0 {
			s.cursorRow--
		}
		s.game.SetCursor(s.fingerprint, s.cursorRow, s.cursorCol)
	case "down", "j":
		if s.cursorRow < 8 {
			s.cursorRow++
		}
		s.game.SetCursor(s.fingerprint, s.cursorRow, s.cursorCol)
	case "left", "h":
		if s.cursorCol > 0 {
			s.cursorCol--
		}
		s.game.SetCursor(s.fingerprint, s.cursorRow, s.cursorCol)
	case "right", "l":
		if s.cursorCol < 8 {
			s.cursorCol++
		}
		s.game.SetCursor(s.fingerprint, s.cursorRow, s.cursorCol)
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		digit := int(key[0] - '0')
		s.game.Place(s.fingerprint, s.cursorRow, s.cursorCol, digit)
	case "x", "backspace", "delete":
		s.game.Clear(s.fingerprint, s.cursorRow, s.cursorCol)
	}

	return s, nil
}

// ChatInput returns the current chat input and clears it.
func (s *SudokuView) ChatInput() string {
	val := s.input.Value()
	s.input.Reset()
	return val
}

// HasChatInput returns true if there's text in the chat input.
func (s SudokuView) HasChatInput() bool {
	return s.input.Value() != ""
}

// FocusChat returns true if chat input is focused.
func (s SudokuView) FocusChat() bool {
	return s.focusChat
}

func (s SudokuView) View() string {
	board := s.game.Board()
	cursors := s.game.Cursors()
	scores := s.game.Scores()

	// Calculate widths
	chatW := s.width - boardCols - 6
	if chatW < 15 {
		chatW = 15
	}

	// Render board
	boardView := s.renderBoard(board, cursors)

	// Render chat panel
	chatView := s.renderChat(chatW)

	// Join board and chat horizontally
	gap := "  "
	boardLines := strings.Split(boardView, "\n")
	chatLines := strings.Split(chatView, "\n")

	// Pad to same height
	maxLines := len(boardLines)
	if len(chatLines) > maxLines {
		maxLines = len(chatLines)
	}
	for len(boardLines) < maxLines {
		boardLines = append(boardLines, strings.Repeat(" ", boardCols))
	}
	for len(chatLines) < maxLines {
		chatLines = append(chatLines, strings.Repeat(" ", chatW))
	}

	var combined strings.Builder
	for i := range maxLines {
		combined.WriteString(boardLines[i])
		combined.WriteString(gap)
		combined.WriteString(chatLines[i])
		if i < maxLines-1 {
			combined.WriteString("\n")
		}
	}

	// Status bar
	var status strings.Builder
	dim := lipgloss.NewStyle().Foreground(ColorDim)
	highlight := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)

	// Scores
	for fp, sc := range scores {
		name := fp
		if len(name) > 12 {
			name = name[:12]
		}
		status.WriteString(dim.Render(fmt.Sprintf("%s:", name)))
		status.WriteString(highlight.Render(fmt.Sprintf("%d", sc)))
		status.WriteString(dim.Render("  "))
	}
	status.WriteString(dim.Render(fmt.Sprintf("· %s · %d/81",
		strings.Title(s.game.Difficulty()), s.game.Filled())))

	// Help line
	var help strings.Builder
	k := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)
	d := lipgloss.NewStyle().Foreground(ColorDim)
	help.WriteString(k.Render("←→↑↓"))
	help.WriteString(d.Render(" move  "))
	help.WriteString(k.Render("1-9"))
	help.WriteString(d.Render(" place  "))
	help.WriteString(k.Render("x"))
	help.WriteString(d.Render(" clear  "))
	help.WriteString(k.Render("C"))
	help.WriteString(d.Render(fmt.Sprintf(" check(%d)  ", 3)))
	help.WriteString(k.Render("Tab"))
	help.WriteString(d.Render(" chat  "))
	help.WriteString(k.Render("ESC"))
	help.WriteString(d.Render(" back"))

	inner := combined.String() + "\n\n" +
		"  " + status.String() + "\n" +
		"  " + help.String()

	return ChatBorderStyle.Width(s.width).Height(s.height).Padding(1, 1).Render(inner)
}

func (s SudokuView) renderBoard(board [9][9]sudoku.Cell, cursors map[string]sudoku.Position) string {
	clue := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	empty := lipgloss.NewStyle().Foreground(ColorDimmer)
	wrong := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	cursorStyle := lipgloss.NewStyle().Background(ColorBorder).Foreground(lipgloss.Color("255")).Bold(true)
	gridLine := lipgloss.NewStyle().Foreground(ColorDimmer)

	var b strings.Builder

	// Top border
	b.WriteString(gridLine.Render("┌───────┬───────┬───────┐"))
	b.WriteString("\n")

	for r := 0; r < 9; r++ {
		b.WriteString(gridLine.Render("│"))
		for c := 0; c < 9; c++ {
			cell := board[r][c]
			isWrong := false
			if _, ok := s.wrongCells[[2]int{r, c}]; ok {
				isWrong = true
			}
			isCursor := r == s.cursorRow && c == s.cursorCol

			var cellStr string
			if cell.Value == 0 {
				cellStr = "·"
			} else {
				cellStr = fmt.Sprintf("%d", cell.Value)
			}

			// Apply style
			var styled string
			switch {
			case isCursor:
				styled = cursorStyle.Render(" " + cellStr + " ")
			case isWrong:
				styled = wrong.Render(" " + cellStr + " ")
			case cell.IsClue:
				styled = clue.Render(" " + cellStr + " ")
			case cell.Value == 0:
				styled = empty.Render(" " + cellStr + " ")
			default:
				// Player placement — use their color
				ci := s.colorIndex // default to own color
				styled = NickStyle(ci).Render(" " + cellStr + " ")
			}

			b.WriteString(styled)

			if c == 2 || c == 5 {
				b.WriteString(gridLine.Render("│"))
			}
		}
		b.WriteString(gridLine.Render("│"))
		b.WriteString("\n")

		// Row separators
		if r == 2 || r == 5 {
			b.WriteString(gridLine.Render("├───────┼───────┼───────┤"))
			b.WriteString("\n")
		}
	}

	// Bottom border
	b.WriteString(gridLine.Render("└───────┴───────┴───────┘"))

	return b.String()
}

func (s SudokuView) renderChat(width int) string {
	header := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	dim := lipgloss.NewStyle().Foreground(ColorDim)
	dimmer := lipgloss.NewStyle().Foreground(ColorDimmer)

	var b strings.Builder
	b.WriteString(header.Render("GAME CHAT"))
	b.WriteString("\n")
	b.WriteString(dimmer.Render(strings.Repeat("─", width-2)))
	b.WriteString("\n")

	// Show recent messages (fit in available height)
	chatHeight := s.height - 8 // leave room for header, input, status
	if chatHeight < 3 {
		chatHeight = 3
	}

	start := 0
	if len(s.messages) > chatHeight {
		start = len(s.messages) - chatHeight
	}

	lines := 0
	for i := start; i < len(s.messages); i++ {
		if lines >= chatHeight {
			break
		}
		msg := s.messages[i]
		if msg.IsSystem {
			b.WriteString(dim.Render(truncateWidth(msg.Text, width-2)))
			b.WriteString("\n")
			lines++
			continue
		}
		nick := NickStyle(msg.ColorIndex).Render(truncateWidth(msg.Nickname, 15))
		text := truncateWidth(msg.Text, width-18)
		b.WriteString(fmt.Sprintf("%s %s", nick, dim.Render(text)))
		b.WriteString("\n")
		lines++
	}

	// Pad remaining space
	for lines < chatHeight {
		b.WriteString("\n")
		lines++
	}

	// Separator and input
	b.WriteString(dimmer.Render(strings.Repeat("─", width-2)))
	b.WriteString("\n")
	if s.focusChat {
		b.WriteString(s.input.View())
	} else {
		b.WriteString(dim.Render("  Tab to chat..."))
	}

	return b.String()
}
