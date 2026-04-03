package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tavrn.sh/internal/chat"
	"tavrn.sh/internal/mention"
)

type ModalType int

const (
	ModalNone ModalType = iota
	ModalHelp
	ModalNick
	ModalJoinRoom
	ModalPost
	ModalExpandNote
	ModalAdminConfirm
	ModalMention
	ModalPoll
	ModalPollVote
	ModalChangelog
	ModalGif
	ModalSubmitFlag
	ModalLeaderboard
	ModalWargameRules
)

// CloseModalMsg signals modal should close.
type CloseModalMsg struct{}

// NickChangeMsg carries the new nickname from the modal.
type NickChangeMsg struct{ Nick string }

// MentionJumpMsg signals the user wants to jump to a mentioned room.
type MentionJumpMsg struct{ Room string }

// ─────────────────────────────────────
// Help Modal
// ─────────────────────────────────────

type HelpModal struct{}

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

	b.WriteString(cat.Render("CHAT KEYS"))
	b.WriteString("\n")
	keys := []struct{ k, d string }{
		{"F1 or ?", "this help"},
		{"F2", "change nickname"},
		{"F3", "switch rooms"},
		{"F4", "view mentions"},
		{"F5", "post note"},
		{"F6", "tankard clicker"},
		{"F7", "leaderboard"},
		{"TAB", "toggle DMs"},
		{"ESC", "close modal / unfocus"},
		{"SHIFT+arrows", "scroll chat"},
	}
	for _, k := range keys {
		fmt.Fprintf(&b, "  %s  %s\n", cmd.Width(18).Render(k.k), desc.Render(k.d))
	}

	b.WriteString("\n")
	b.WriteString(cat.Render("COMMANDS"))
	b.WriteString("\n")
	cmds := []struct{ k, d string }{
		{"/poll", "create a poll"},
		{"/vote", "vote on active polls"},
		{"/endpoll", "close your poll"},
		{"/gif <search>", "search and send animated GIFs"},
		{"/submit", "submit a wargame flag"},
		{"/leaderboard", "view hacker rankings"},
		{"/dm @name", "open private DM"},
		{"/dm @name <msg>", "send a DM directly"},
	}
	for _, c := range cmds {
		fmt.Fprintf(&b, "  %s  %s\n", cmd.Width(18).Render(c.k), desc.Render(c.d))
	}

	b.WriteString("\n")
	b.WriteString(cat.Render("GALLERY KEYS"))
	b.WriteString("\n")
	gkeys := []struct{ k, d string }{
		{"P", "post note"},
		{"D", "delete your note"},
		{"TAB", "cycle selection"},
		{"click + drag", "move your notes"},
	}
	for _, k := range gkeys {
		fmt.Fprintf(&b, "  %s  %s\n", cmd.Width(18).Render(k.k), desc.Render(k.d))
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
	input textinput.Model
	err   string
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
	roomTypes   map[string]string
	currentRoom string
	cursor      int
}

func NewJoinRoomModal(rooms []string, counts []int, currentRoom string, roomTypes map[string]string) JoinRoomModal {
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
		roomTypes:   roomTypes,
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

	sectionHeader := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	greenHeader := lipgloss.NewStyle().Foreground(ColorGreen).Bold(true)
	dimSep := lipgloss.NewStyle().Foreground(ColorDimmer)

	renderRoom := func(b *strings.Builder, i int, rm string) {
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

		b.WriteString(line + "\n")
	}

	var b2 strings.Builder
	b2.WriteString(header)
	b2.WriteString("\n\n")

	// Regular rooms
	b2.WriteString(sectionHeader.Render("  ROOMS"))
	b2.WriteString("\n")
	b2.WriteString("  " + dimSep.Render(strings.Repeat("─", 32)))
	b2.WriteString("\n")
	for i, rm := range j.rooms {
		if j.roomTypes != nil && j.roomTypes[rm] == "wargame" {
			continue
		}
		renderRoom(&b2, i, rm)
	}

	// Wargame rooms
	hasWargame := false
	for _, rm := range j.rooms {
		if j.roomTypes != nil && j.roomTypes[rm] == "wargame" {
			hasWargame = true
			break
		}
	}
	if hasWargame {
		b2.WriteString("\n")
		b2.WriteString(greenHeader.Render("  WARGAMES"))
		b2.WriteString("\n")
		b2.WriteString("  " + dimSep.Render(strings.Repeat("─", 32)))
		b2.WriteString("\n")
		for i, rm := range j.rooms {
			if j.roomTypes == nil || j.roomTypes[rm] != "wargame" {
				continue
			}
			renderRoom(&b2, i, rm)
		}
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
	input textarea.Model
}

func NewPostModal() PostModal {
	ta := textarea.New()
	ta.Placeholder = "write something on the board..."
	ta.Focus()
	ta.CharLimit = 280
	ta.MaxWidth = 50
	ta.MaxHeight = 8
	ta.ShowLineNumbers = false
	return PostModal{input: ta}
}

func (p PostModal) Update(msg tea.Msg) (PostModal, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			return p, func() tea.Msg { return CloseModalMsg{} }
		case "ctrl+s":
			// Submit with ctrl+s since enter is newline in textarea
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
	modalW := 54
	headerText := " Post a Note "
	fillLen := modalW - len(headerText)
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
	b3.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render("  280 chars max. text wraps automatically."))
	b3.WriteString("\n\n")
	b3.WriteString(p.input.View())

	b3.WriteString("\n\n")
	footerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(
		strings.Repeat("╱", modalW))
	b3.WriteString(footerFill)
	b3.WriteString("\n")
	submit := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("CTRL+S")
	esc := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("ESC")
	b3.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render(
		fmt.Sprintf("  %s post  ·  %s cancel", submit, esc)))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2).
		Width(modalW + 6).
		Render(b3.String())
}

// ─────────────────────────────────────
// Expand Note Modal
// ─────────────────────────────────────

type ExpandNoteModal struct {
	Text     string
	Nickname string
	ColorIdx int
	IsOwn    bool
	NoteID   int
}

func NewExpandNoteModal(text, nick string, colorIdx int, isOwn bool, noteID int) ExpandNoteModal {
	return ExpandNoteModal{
		Text:     text,
		Nickname: nick,
		ColorIdx: colorIdx,
		IsOwn:    isOwn,
		NoteID:   noteID,
	}
}

func (e ExpandNoteModal) View(width, height int) string {
	modalW := 56
	headerText := " Note "
	fillLen := modalW - len(headerText)
	if fillLen < 4 {
		fillLen = 4
	}
	leftFill := strings.Repeat("╱", fillLen/2)
	rightFill := strings.Repeat("╱", fillLen-fillLen/2)

	borderColor := NickColors[e.ColorIdx%len(NickColors)]
	headerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(leftFill)
	headerTitle := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(headerText)
	headerFillR := lipgloss.NewStyle().Foreground(ColorBorder).Render(rightFill)

	var b4 strings.Builder
	b4.WriteString(headerFill + headerTitle + headerFillR)
	b4.WriteString("\n\n")

	// Nick header
	nick := e.Nickname
	if e.IsOwn {
		nick = "~" + nick
	}
	nickStyled := lipgloss.NewStyle().Foreground(borderColor).Bold(true).Render(nick)
	b4.WriteString("  " + nickStyled)
	b4.WriteString("\n\n")

	// Full text with word wrapping — wider for readability
	wrapWidth := modalW - 6
	wrapped := wordWrap(e.Text, wrapWidth)
	for _, line := range wrapped {
		b4.WriteString("  " + line + "\n")
	}

	// Footer
	b4.WriteString("\n")
	footerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(
		strings.Repeat("╱", modalW))
	b4.WriteString(footerFill)
	b4.WriteString("\n")

	esc := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("ESC")
	footer := lipgloss.NewStyle().Foreground(ColorDim).Render(
		fmt.Sprintf("  %s close", esc))

	if e.IsOwn {
		del := lipgloss.NewStyle().Foreground(lipgloss.Color("131")).Bold(true).Render("D")
		footer += lipgloss.NewStyle().Foreground(ColorDim).Render(
			fmt.Sprintf("  ·  %s delete", del))
	}
	b4.WriteString(footer)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Render(b4.String())
}

// ─────────────────────────────────────
// Admin Confirm Modal
// ─────────────────────────────────────

type AdminConfirmMsg struct{ Action string }

type AdminConfirmModal struct {
	Action  string // "purge", "ban <fp>", etc
	Message string
}

func NewAdminConfirmModal(action, message string) AdminConfirmModal {
	return AdminConfirmModal{Action: action, Message: message}
}

func (a AdminConfirmModal) Update(msg tea.Msg) (AdminConfirmModal, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "y":
			action := a.Action
			return a, func() tea.Msg { return AdminConfirmMsg{Action: action} }
		case "n", "esc":
			return a, func() tea.Msg { return CloseModalMsg{} }
		}
	}
	return a, nil
}

func (a AdminConfirmModal) View(width, height int) string {
	warn := lipgloss.NewStyle().Foreground(lipgloss.Color("131")).Bold(true)
	dim := lipgloss.NewStyle().Foreground(ColorDim)

	var b5 strings.Builder

	headerText := " ADMIN "
	fillLen := 40 - len(headerText)
	leftFill := strings.Repeat("╱", fillLen/2)
	rightFill := strings.Repeat("╱", fillLen-fillLen/2)
	b5.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("131")).Render(leftFill))
	b5.WriteString(warn.Render(headerText))
	b5.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("131")).Render(rightFill))
	b5.WriteString("\n\n")

	b5.WriteString("  " + warn.Render(a.Message))
	b5.WriteString("\n\n")
	b5.WriteString("  " + dim.Render("This action cannot be undone."))

	b5.WriteString("\n\n")
	b5.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("131")).Render(
		strings.Repeat("╱", 40)))
	b5.WriteString("\n")

	y := lipgloss.NewStyle().Foreground(lipgloss.Color("108")).Bold(true).Render("Y")
	n := lipgloss.NewStyle().Foreground(lipgloss.Color("131")).Bold(true).Render("N")
	b5.WriteString(dim.Render(fmt.Sprintf("  %s confirm  ·  %s cancel", y, n)))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("131")).
		Padding(1, 2).
		Render(b5.String())
}

// ─────────────────────────────────────
// Mention Modal
// ─────────────────────────────────────

type MentionModal struct {
	mentions []mention.Mention
	contexts [][]chat.Message // context messages per mention (3 before)
	current  int
}

func NewMentionModal(mentions []mention.Mention, contexts [][]chat.Message) MentionModal {
	return MentionModal{
		mentions: mentions,
		contexts: contexts,
		current:  0,
	}
}

func (m MentionModal) Update(msg tea.Msg) (MentionModal, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			return m, func() tea.Msg { return CloseModalMsg{} }
		case "enter":
			if len(m.mentions) > 0 {
				room := m.mentions[m.current].Room
				return m, func() tea.Msg { return MentionJumpMsg{Room: room} }
			}
		case "left":
			if m.current > 0 {
				m.current--
			}
		case "right":
			if m.current < len(m.mentions)-1 {
				m.current++
			}
		}
	}
	return m, nil
}

// Current returns the index of the currently viewed mention.
func (m MentionModal) Current() int {
	return m.current
}

func (m MentionModal) View(width, height int) string {
	if len(m.mentions) == 0 {
		return ""
	}

	modalW := 52
	cur := m.mentions[m.current]

	// Header
	headerText := fmt.Sprintf(" @mention from %s in #%s ", cur.Author, cur.Room)
	counter := fmt.Sprintf(" %d/%d ", m.current+1, len(m.mentions))
	fillLen := modalW - len(headerText) - len(counter)
	if fillLen < 2 {
		fillLen = 2
	}
	leftFill := strings.Repeat("╱", fillLen/2)
	rightFill := strings.Repeat("╱", fillLen-fillLen/2)

	headerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(leftFill)
	headerTitle := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(headerText)
	counterText := lipgloss.NewStyle().Foreground(ColorDim).Render(counter)
	headerFillR := lipgloss.NewStyle().Foreground(ColorBorder).Render(rightFill)
	header := headerFill + headerTitle + counterText + headerFillR

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")

	dim := lipgloss.NewStyle().Foreground(ColorDim)
	wrapWidth := modalW - 6

	// Context messages (dimmed)
	if m.current < len(m.contexts) {
		for _, ctx := range m.contexts[m.current] {
			nick := lipgloss.NewStyle().Foreground(NickColors[ctx.ColorIndex%len(NickColors)]).
				Render(ctx.Nickname)
			ts := dim.Render(formatTimestamp(ctx.Timestamp, time.Now()))
			fmt.Fprintf(&b, "  %s  %s\n", nick, ts)
			wrapped := wordWrap(ctx.Text, wrapWidth)
			for _, line := range wrapped {
				b.WriteString(dim.Render("      "+line) + "\n")
			}
			b.WriteString("\n")
		}
	}

	// The mentioned message (highlighted)
	highlight := lipgloss.NewStyle().Foreground(ColorAmber).Bold(true)
	nickColor := NickColors[cur.ColorIndex%len(NickColors)]
	nick := lipgloss.NewStyle().Foreground(nickColor).Bold(true).Render(cur.Author)
	ts := dim.Render(formatTimestamp(cur.Timestamp, time.Now()))
	fmt.Fprintf(&b, "▸ %s  %s\n", nick, ts)
	wrapped := wordWrap(cur.Text, wrapWidth)
	for _, line := range wrapped {
		b.WriteString(highlight.Render("▸     "+line) + "\n")
	}

	// Footer
	b.WriteString("\n")
	footerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(
		strings.Repeat("╱", modalW))
	b.WriteString(footerFill)
	b.WriteString("\n")

	arrows := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("←→")
	enter := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("ENTER")
	esc := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("ESC")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render(
		fmt.Sprintf("  %s prev/next  ·  %s jump  ·  %s close", arrows, enter, esc)))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2).
		Render(b.String())
}
