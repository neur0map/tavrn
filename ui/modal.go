package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type ModalType int

const (
	ModalNone ModalType = iota
	ModalHelp
	ModalNick
	ModalJoinRoom
	ModalPost
)

// CloseModalMsg signals modal should close.
type CloseModalMsg struct{}

// NickChangeMsg carries the new nickname from the modal.
type NickChangeMsg struct{ Nick string }

// ─────────────────────────────────────
// Help Modal
// ─────────────────────────────────────

type HelpModal struct {
	width  int
	height int
}

func NewHelpModal() HelpModal {
	return HelpModal{}
}

func (h HelpModal) View(width, height int) string {
	// Header with ╱╱╱ fill
	headerText := " Help "
	fillLen := 40 - len(headerText)
	if fillLen < 4 {
		fillLen = 4
	}
	leftFill := strings.Repeat("╱", fillLen/2)
	rightFill := strings.Repeat("╱", fillLen-fillLen/2)

	headerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(leftFill)
	headerTitle := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(headerText)
	headerFillR := lipgloss.NewStyle().Foreground(ColorBorder).Render(rightFill)
	header := headerFill + headerTitle + headerFillR

	// Sections
	cat := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	cmd := lipgloss.NewStyle().Foreground(ColorCommand).Bold(true)
	desc := lipgloss.NewStyle().Foreground(ColorDesc)

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")

	b.WriteString(cat.Render("CHAT"))
	b.WriteString("\n")
	cmds := []struct{ c, d string }{
		{"/join ROOM", "switch rooms"},
		{"/nick NAME", "change your handle"},
		{"/who", "see who's around"},
		{"/post MSG", "sticky note (#gallery)"},
		{"/help", "show this card"},
	}
	for _, c := range cmds {
		b.WriteString(fmt.Sprintf("  %s  %s\n", cmd.Width(18).Render(c.c), desc.Render(c.d)))
	}

	b.WriteString("\n")
	b.WriteString(cat.Render("KEYS"))
	b.WriteString("\n")
	keys := []struct{ k, d string }{
		{"F1 or ?", "this help"},
		{"F2", "change nickname"},
		{"F3", "switch rooms"},
		{"F4", "post note"},
		{"ESC", "close modal"},
		{"SHIFT+arrows", "scroll chat"},
	}
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("  %s  %s\n", cmd.Width(18).Render(k.k), desc.Render(k.d)))
	}

	b.WriteString("\n")
	b.WriteString(cat.Render("GALLERY KEYS"))
	b.WriteString("\n")
	gkeys := []struct{ k, d string }{
		{"P", "post note"},
		{"J", "switch rooms"},
		{"H", "help"},
		{"D", "delete your note"},
		{"TAB", "cycle selection"},
		{"click + drag", "move your notes"},
	}
	for _, k := range gkeys {
		b.WriteString(fmt.Sprintf("  %s  %s\n", cmd.Width(18).Render(k.k), desc.Render(k.d)))
	}

	b.WriteString("\n")
	b.WriteString(cat.Render("INFO"))
	b.WriteString("\n")
	b.WriteString(desc.Italic(true).Render(
		"  All data purged every Sunday 23:59 UTC.\n"))
	b.WriteString(desc.Italic(true).Render(
		"  Nothing is permanent. Draw while you can."))

	// Footer
	b.WriteString("\n\n")
	footerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(
		strings.Repeat("╱", 40))
	b.WriteString(footerFill)
	b.WriteString("\n")
	esc := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("ESC")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render(
		fmt.Sprintf("  %s close", esc)))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2).
		Render(b.String())
}

// ─────────────────────────────────────
// Nick Modal
// ─────────────────────────────────────

type NickModal struct {
	input  textinput.Model
	width  int
	height int
	err    string
}

func NewNickModal(currentNick string) NickModal {
	ti := textinput.New()
	ti.Placeholder = "enter new nickname..."
	ti.Focus()
	ti.CharLimit = 20
	ti.Prompt = "> "
	ti.SetValue(currentNick)

	return NickModal{input: ti}
}

func (n NickModal) Update(msg tea.Msg) (NickModal, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return n, func() tea.Msg { return CloseModalMsg{} }
		case "enter":
			val := strings.TrimSpace(n.input.Value())
			if len(val) >= 2 && len(val) <= 20 {
				return n, func() tea.Msg { return NickChangeMsg{Nick: val} }
			}
			n.err = "nickname must be 2-20 characters"
			return n, nil
		}
	}

	var cmd tea.Cmd
	n.input, cmd = n.input.Update(msg)
	n.err = "" // clear error on typing
	return n, cmd
}

func (n NickModal) View(width, height int) string {
	// Header
	headerText := " Change Nickname "
	fillLen := 36 - len(headerText)
	if fillLen < 4 {
		fillLen = 4
	}
	leftFill := strings.Repeat("╱", fillLen/2)
	rightFill := strings.Repeat("╱", fillLen-fillLen/2)

	headerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(leftFill)
	headerTitle := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(headerText)
	headerFillR := lipgloss.NewStyle().Foreground(ColorBorder).Render(rightFill)
	header := headerFill + headerTitle + headerFillR

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render("  Enter a new nickname (2-20 chars)"))
	b.WriteString("\n\n")
	b.WriteString("  " + n.input.View())

	if n.err != "" {
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("131")).Render("  " + n.err))
	}

	// Footer
	b.WriteString("\n\n")
	footerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(
		strings.Repeat("╱", 36))
	b.WriteString(footerFill)
	b.WriteString("\n")
	enter := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("ENTER")
	esc := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("ESC")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render(
		fmt.Sprintf("  %s confirm  ·  %s cancel", enter, esc)))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2).
		Render(b.String())
}

// ─────────────────────────────────────
// Join Room Modal
// ─────────────────────────────────────

// JoinRoomMsg carries the selected room name.
type JoinRoomMsg struct{ Room string }

type JoinRoomModal struct {
	rooms       []string
	counts      []int
	currentRoom string
	cursor      int
}

func NewJoinRoomModal(rooms []string, counts []int, currentRoom string) JoinRoomModal {
	cursor := 0
	for i, r := range rooms {
		if r == currentRoom {
			cursor = i
			break
		}
	}
	return JoinRoomModal{
		rooms:       rooms,
		counts:      counts,
		currentRoom: currentRoom,
		cursor:      cursor,
	}
}

func (j JoinRoomModal) Update(msg tea.Msg) (JoinRoomModal, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			return j, func() tea.Msg { return CloseModalMsg{} }
		case "enter":
			selected := j.rooms[j.cursor]
			return j, func() tea.Msg { return JoinRoomMsg{Room: selected} }
		case "up", "k":
			j.cursor--
			if j.cursor < 0 {
				j.cursor = len(j.rooms) - 1
			}
		case "down", "j":
			j.cursor++
			if j.cursor >= len(j.rooms) {
				j.cursor = 0
			}
		}
	}
	return j, nil
}

func (j JoinRoomModal) View(width, height int) string {
	headerText := " Join Room "
	fillLen := 36 - len(headerText)
	if fillLen < 4 {
		fillLen = 4
	}
	leftFill := strings.Repeat("╱", fillLen/2)
	rightFill := strings.Repeat("╱", fillLen-fillLen/2)

	headerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(leftFill)
	headerTitle := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(headerText)
	headerFillR := lipgloss.NewStyle().Foreground(ColorBorder).Render(rightFill)
	header := headerFill + headerTitle + headerFillR

	var b2 strings.Builder
	b2.WriteString(header)
	b2.WriteString("\n\n")

	for i, rm := range j.rooms {
		count := 0
		if i < len(j.counts) {
			count = j.counts[i]
		}

		isCurrent := rm == j.currentRoom
		isSelected := i == j.cursor

		name := fmt.Sprintf("#%s", rm)
		countStr := fmt.Sprintf("  %d online", count)

		var line string
		if isSelected {
			indicator := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(" ▸ ")
			roomStyle := lipgloss.NewStyle().Foreground(ColorAmber).Bold(true)
			countStyle := lipgloss.NewStyle().Foreground(ColorDim)
			line = indicator + roomStyle.Render(name) + countStyle.Render(countStr)
		} else {
			roomStyle := lipgloss.NewStyle().Foreground(ColorSand)
			countStyle := lipgloss.NewStyle().Foreground(ColorDimmer)
			line = "   " + roomStyle.Render(name) + countStyle.Render(countStr)
		}

		if isCurrent {
			tag := lipgloss.NewStyle().Foreground(ColorDim).Render("  (here)")
			line += tag
		}

		b2.WriteString(line)
		b2.WriteString("\n")
	}

	b2.WriteString("\n")
	footerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(
		strings.Repeat("╱", 36))
	b2.WriteString(footerFill)
	b2.WriteString("\n")
	arrows := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("↑↓")
	enterK := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("ENTER")
	escK := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("ESC")
	b2.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render(
		fmt.Sprintf("  %s navigate  ·  %s join  ·  %s cancel", arrows, enterK, escK)))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2).
		Render(b2.String())
}

// ─────────────────────────────────────
// Post Note Modal
// ─────────────────────────────────────

type PostNoteMsg struct{ Text string }

type PostModal struct {
	input textinput.Model
}

func NewPostModal() PostModal {
	ti := textinput.New()
	ti.Placeholder = "write something on the board..."
	ti.Focus()
	ti.CharLimit = 60
	ti.Prompt = "> "
	return PostModal{input: ti}
}

func (p PostModal) Update(msg tea.Msg) (PostModal, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			return p, func() tea.Msg { return CloseModalMsg{} }
		case "enter":
			val := strings.TrimSpace(p.input.Value())
			if val != "" {
				return p, func() tea.Msg { return PostNoteMsg{Text: val} }
			}
			return p, nil
		}
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, cmd
}

func (p PostModal) View(width, height int) string {
	headerText := " Post a Note "
	fillLen := 36 - len(headerText)
	if fillLen < 4 {
		fillLen = 4
	}
	leftFill := strings.Repeat("╱", fillLen/2)
	rightFill := strings.Repeat("╱", fillLen-fillLen/2)

	headerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(leftFill)
	headerTitle := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(headerText)
	headerFillR := lipgloss.NewStyle().Foreground(ColorBorder).Render(rightFill)
	header := headerFill + headerTitle + headerFillR

	var b3 strings.Builder
	b3.WriteString(header)
	b3.WriteString("\n\n")
	b3.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render("  Leave a note on the board (60 chars max)"))
	b3.WriteString("\n\n")
	b3.WriteString("  " + p.input.View())

	b3.WriteString("\n\n")
	footerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(
		strings.Repeat("╱", 36))
	b3.WriteString(footerFill)
	b3.WriteString("\n")
	enter := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("ENTER")
	esc := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("ESC")
	b3.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render(
		fmt.Sprintf("  %s post  ·  %s cancel", enter, esc)))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2).
		Render(b3.String())
}
