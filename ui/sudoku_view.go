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
	// Board with 4-char wide cells: │ XX │ XX │ XX ║ XX │ ...
	// 9 cells × 4 chars + 4 separators (│ at edges and ║ at box boundaries) = 40 chars
	renderedBoardW = 40
	checkFlashMs   = 5000
)

type SudokuView struct {
	game        *sudoku.Game
	fingerprint string
	nickname    string
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

func NewSudokuView(game *sudoku.Game, fingerprint, nickname string, colorIndex int) SudokuView {
	ti := textinput.New()
	ti.Placeholder = "Chat..."
	ti.CharLimit = 200
	ti.Prompt = "  > "

	return SudokuView{
		game:        game,
		fingerprint: fingerprint,
		nickname:    nickname,
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

	if key == "tab" {
		s.focusChat = !s.focusChat
		if s.focusChat {
			s.input.Focus()
		} else {
			s.input.Blur()
		}
		return s, nil
	}

	if s.focusChat {
		switch key {
		case "enter":
			// handled by app
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

func (s *SudokuView) ChatInput() string {
	val := s.input.Value()
	s.input.Reset()
	return val
}

func (s SudokuView) HasChatInput() bool {
	return s.input.Value() != ""
}

func (s SudokuView) FocusChat() bool {
	return s.focusChat
}

func (s SudokuView) View() string {
	board := s.game.Board()
	cursors := s.game.Cursors()

	// Board takes left side, chat takes the rest
	chatW := s.width - renderedBoardW - 8
	if chatW < 20 {
		chatW = 20
	}

	boardView := s.renderBoard(board, cursors)
	chatView := s.renderChat(chatW)

	// Join side by side with a vertical separator
	boardLines := strings.Split(boardView, "\n")
	chatLines := strings.Split(chatView, "\n")

	// Target height for the content area (leave room for status + help)
	contentH := s.height - 7
	if contentH < 13 {
		contentH = 13
	}

	// Pad both to content height
	for len(boardLines) < contentH {
		boardLines = append(boardLines, strings.Repeat(" ", renderedBoardW))
	}
	for len(chatLines) < contentH {
		chatLines = append(chatLines, "")
	}

	sep := lipgloss.NewStyle().Foreground(ColorDimmer)
	var combined strings.Builder
	for i := 0; i < contentH; i++ {
		bl := boardLines[i]
		cl := ""
		if i < len(chatLines) {
			cl = chatLines[i]
		}
		combined.WriteString(bl)
		combined.WriteString(sep.Render("  │ "))
		combined.WriteString(cl)
		combined.WriteString("\n")
	}

	// Score line — use nickname
	dim := lipgloss.NewStyle().Foreground(ColorDim)
	hl := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)

	myScore := s.game.Score(s.fingerprint)
	scoreLine := "  " + NickStyle(s.colorIndex).Render(s.nickname) +
		dim.Render(":") + hl.Render(fmt.Sprintf("%d", myScore)) +
		dim.Render(fmt.Sprintf("  · %s · %d/81",
			strings.ToUpper(s.game.Difficulty()[:1])+s.game.Difficulty()[1:],
			s.game.Filled()))

	// Help line
	k := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)
	d := lipgloss.NewStyle().Foreground(ColorDim)
	helpLine := "  " + k.Render("←→↑↓") + d.Render(" move  ") +
		k.Render("1-9") + d.Render(" place  ") +
		k.Render("x") + d.Render(" clear  ") +
		k.Render("C") + d.Render(" check(3)  ") +
		k.Render("Tab") + d.Render(" chat  ") +
		k.Render("ESC") + d.Render(" back")

	inner := combined.String() + "\n" + scoreLine + "\n" + helpLine

	return ChatBorderStyle.Width(s.width).Height(s.height).Padding(1, 1).Render(inner)
}

func (s SudokuView) renderBoard(board [9][9]sudoku.Cell, cursors map[string]sudoku.Position) string {
	clue := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	empty := lipgloss.NewStyle().Foreground(ColorDimmer)
	wrong := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	cursorBg := lipgloss.NewStyle().Background(ColorBorder).Foreground(lipgloss.Color("255")).Bold(true)
	grid := lipgloss.NewStyle().Foreground(ColorDimmer)
	boxGrid := lipgloss.NewStyle().Foreground(ColorBorder)

	var b strings.Builder

	// Top border
	b.WriteString(grid.Render("╔════════════╦════════════╦════════════╗"))
	b.WriteString("\n")

	for r := 0; r < 9; r++ {
		b.WriteString(grid.Render("║"))
		for c := 0; c < 9; c++ {
			cell := board[r][c]
			_, isWrong := s.wrongCells[[2]int{r, c}]
			isCursor := r == s.cursorRow && c == s.cursorCol

			var cellStr string
			if cell.Value == 0 {
				cellStr = " · "
			} else {
				cellStr = fmt.Sprintf(" %d ", cell.Value)
			}

			// Style the cell content
			var styled string
			switch {
			case isCursor:
				styled = cursorBg.Render(cellStr)
			case isWrong:
				styled = wrong.Render(cellStr)
			case cell.IsClue:
				styled = clue.Render(cellStr)
			case cell.Value == 0:
				styled = empty.Render(cellStr)
			default:
				styled = NickStyle(s.colorIndex).Render(cellStr)
			}

			b.WriteString(styled)

			// Column separator
			if c == 2 || c == 5 {
				b.WriteString(boxGrid.Render("║"))
			} else if c < 8 {
				b.WriteString(grid.Render("│"))
			}
		}
		b.WriteString(grid.Render("║"))
		b.WriteString("\n")

		// Row separators
		if r == 2 || r == 5 {
			b.WriteString(boxGrid.Render("╠════════════╬════════════╬════════════╣"))
			b.WriteString("\n")
		} else if r < 8 {
			b.WriteString(grid.Render("║────────────║────────────║────────────║"))
			b.WriteString("\n")
		}
	}

	// Bottom border
	b.WriteString(grid.Render("╚════════════╩════════════╩════════════╝"))

	return b.String()
}

func (s SudokuView) renderChat(width int) string {
	header := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	dim := lipgloss.NewStyle().Foreground(ColorDim)
	dimmer := lipgloss.NewStyle().Foreground(ColorDimmer)

	var b strings.Builder
	b.WriteString(header.Render("GAME CHAT"))
	b.WriteString("\n")
	b.WriteString(dimmer.Render(strings.Repeat("─", width)))
	b.WriteString("\n")

	// Chat area height
	chatHeight := s.height - 10
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
			b.WriteString(dim.Render(truncateWidth(msg.Text, width)))
			b.WriteString("\n")
			lines++
			continue
		}
		nick := NickStyle(msg.ColorIndex).Render(truncateWidth(msg.Nickname, 15))
		text := truncateWidth(msg.Text, width-17)
		b.WriteString(fmt.Sprintf("%s %s\n", nick, dim.Render(text)))
		lines++
	}

	for lines < chatHeight {
		b.WriteString("\n")
		lines++
	}

	b.WriteString(dimmer.Render(strings.Repeat("─", width)))
	b.WriteString("\n")
	if s.focusChat {
		b.WriteString(s.input.View())
	} else {
		b.WriteString(dim.Render("  Tab to chat..."))
	}

	return b.String()
}
