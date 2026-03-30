package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"tavrn.sh/internal/chat"
	"tavrn.sh/internal/fuzzy"
	"tavrn.sh/internal/identity"
)

// Typing dots animation frames
var typingFrames = []string{"   ", ".  ", ".. ", "..."}

const (
	maxLogLines = 4
	logTTL      = 5 * time.Second
)

type sysLogEntry struct {
	text string
	at   time.Time
}

type ChatView struct {
	viewport    viewport.Model
	input       textinput.Model
	messages    []chat.Message
	sysLogs     []sysLogEntry        // timestamped, auto-expire
	typingUsers map[string]time.Time // nick → last typing time
	typingFrame int
	width       int
	height      int

	// Autocomplete popup
	mentionPopup  bool
	mentionQuery  string
	mentionNames  []string // filtered results from fuzzy.Match
	mentionCursor int      // selected index
}

func NewChatView() ChatView {
	ti := textinput.New()
	ti.Placeholder = "Type your message..."
	ti.Focus()
	ti.CharLimit = 500
	ti.Prompt = "  → > "

	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(10))

	return ChatView{
		viewport:    vp,
		input:       ti,
		messages:    make([]chat.Message, 0),
		typingUsers: make(map[string]time.Time),
	}
}

func (c *ChatView) SetSize(width, height int) {
	c.width = width
	c.height = height
	typingHeight := 1
	inputHeight := 1
	sepHeight := 1
	borderHeight := 2
	padHeight := 2
	vpW := width - borderHeight - 2
	vpH := height - inputHeight - sepHeight - typingHeight - borderHeight - padHeight
	if vpW < 1 {
		vpW = 1
	}
	if vpH < 1 {
		vpH = 1
	}
	c.viewport.SetWidth(vpW)
	c.viewport.SetHeight(vpH)
	c.input.SetWidth(width - 10)
}

func (c *ChatView) AddMessage(msg chat.Message) {
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	// Route system messages (not banners) to the log box
	if msg.IsSystem && !msg.IsBanner {
		c.addSysLog(msg.Text)
		return
	}
	c.messages = append(c.messages, msg)
	c.renderMessages()
	c.viewport.GotoBottom()
}

func (c *ChatView) addSysLog(text string) {
	c.sysLogs = append(c.sysLogs, sysLogEntry{text: text, at: time.Now()})
	if len(c.sysLogs) > maxLogLines {
		c.sysLogs = c.sysLogs[len(c.sysLogs)-maxLogLines:]
	}
}

func (c *ChatView) pruneExpiredLogs() {
	now := time.Now()
	n := 0
	for _, e := range c.sysLogs {
		if now.Sub(e.at) < logTTL {
			c.sysLogs[n] = e
			n++
		}
	}
	c.sysLogs = c.sysLogs[:n]
}

func (c *ChatView) SetTyping(nick string) {
	c.typingUsers[nick] = time.Now()
}

func (c *ChatView) ClearStaleTyping() {
	now := time.Now()
	for k, t := range c.typingUsers {
		if now.Sub(t) > 3*time.Second {
			delete(c.typingUsers, k)
		}
	}
}

func (c *ChatView) Tick() {
	c.typingFrame++
	c.ClearStaleTyping()
	c.pruneExpiredLogs()
	// Re-render to update relative timestamps
	c.renderMessages()
}

func (c ChatView) HasTypingUsers() bool {
	return len(c.typingUsers) > 0
}

func (c ChatView) HasActiveLogs() bool {
	return len(c.sysLogs) > 0
}

func (c *ChatView) renderMessages() {
	var lines []string
	now := time.Now()

	prevNick := ""
	for i, msg := range c.messages {
		if msg.IsBanner {
			// Admin banner: eye-catching, framed, word-wrapped
			bannerLine := lipgloss.NewStyle().Foreground(ColorAmber).Render("  ───")
			bannerStyle := lipgloss.NewStyle().Foreground(ColorAmber).Bold(true)
			lines = append(lines, "")
			lines = append(lines, bannerLine)
			for _, wl := range wordWrap(msg.Text, c.viewport.Width()-8) {
				lines = append(lines, bannerStyle.Render("  "+wl))
			}
			lines = append(lines, bannerLine)
			lines = append(lines, "")
			prevNick = ""
			continue
		}

		// Discord-style: group consecutive messages from same user
		sameUser := msg.Nickname == prevNick
		prevNick = msg.Nickname

		if !sameUser {
			// Add spacing before new user block (except first message)
			if i > 0 {
				lines = append(lines, "")
			}

			// Nick + timestamp header
			displayNick := msg.Nickname
			if identity.IsOwner(msg.Nickname) {
				displayNick = identity.OwnerDisplayName()
			}
			nick := NickStyle(msg.ColorIndex).Render(displayNick)
			ts := formatTimestamp(msg.Timestamp, now)
			timeStr := MsgTimeStyle.Render(ts)
			header := fmt.Sprintf("    %s  %s", nick, timeStr)
			lines = append(lines, header)
		}

		// Message body — indented under nick
		msgLines := wordWrap(msg.Text, c.viewport.Width()-8)
		for _, ml := range msgLines {
			body := "      " + ml
			lines = append(lines, body)
		}
	}
	c.viewport.SetContent(strings.Join(lines, "\n"))
}

func formatTimestamp(t time.Time, now time.Time) string {
	diff := now.Sub(t)

	switch {
	case diff < 10*time.Second:
		return "just now"
	case diff < time.Minute:
		return fmt.Sprintf("%ds ago", int(diff.Seconds()))
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", mins)
	case diff < 24*time.Hour:
		return t.Format("15:04")
	default:
		return t.Format("Jan 02 15:04")
	}
}

func wordWrap(text string, width int) []string {
	if width <= 0 || lipgloss.Width(text) <= width {
		return []string{text}
	}
	var lines []string
	words := strings.Fields(text)
	current := ""
	currentW := 0
	for _, word := range words {
		wordW := lipgloss.Width(word)
		if current == "" {
			current = word
			currentW = wordW
		} else if currentW+1+wordW <= width {
			current += " " + word
			currentW += 1 + wordW
		} else {
			lines = append(lines, current)
			current = word
			currentW = wordW
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func (c ChatView) Update(msg tea.Msg) (ChatView, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	// Handle mention popup keys first
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok && c.mentionPopup {
		switch keyMsg.String() {
		case "up":
			if c.mentionCursor > 0 {
				c.mentionCursor--
			} else if len(c.mentionNames) > 0 {
				c.mentionCursor = len(c.mentionNames) - 1
			}
			return c, nil
		case "down":
			if c.mentionCursor < len(c.mentionNames)-1 {
				c.mentionCursor++
			} else {
				c.mentionCursor = 0
			}
			return c, nil
		case "tab", "enter":
			if len(c.mentionNames) > 0 {
				c.completeMention(c.mentionNames[c.mentionCursor])
				return c, nil
			}
		case "esc":
			c.mentionPopup = false
			c.mentionNames = nil
			c.mentionCursor = 0
			return c, nil
		}
	}

	// Route scroll keys to viewport
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "pgup", "pgdown", "home", "end":
			c.viewport, cmd = c.viewport.Update(msg)
			cmds = append(cmds, cmd)
			return c, tea.Batch(cmds...)
		case "shift+up", "up":
			c.viewport.ScrollUp(3)
			return c, nil
		case "shift+down", "down":
			c.viewport.ScrollDown(3)
			return c, nil
		}
	}

	// Route mouse wheel to viewport
	if m, ok := msg.(tea.MouseWheelMsg); ok {
		switch m.Button {
		case tea.MouseWheelUp:
			c.viewport.ScrollUp(3)
		case tea.MouseWheelDown:
			c.viewport.ScrollDown(3)
		}
		return c, nil
	}

	c.input, cmd = c.input.Update(msg)
	cmds = append(cmds, cmd)

	// Still pass non-key messages (like window resize) to viewport
	if _, ok := msg.(tea.KeyPressMsg); !ok {
		c.viewport, cmd = c.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return c, tea.Batch(cmds...)
}

func (c ChatView) View() string {
	chatContent := c.viewport.View()

	// Overlay log box on bottom-right of chat content
	if len(c.sysLogs) > 0 {
		chatContent = c.overlayLogBox(chatContent)
	}

	// Typing indicator
	typingLine := c.renderTypingIndicator()

	// Separator
	sepWidth := c.width - 6
	if sepWidth < 1 {
		sepWidth = 1
	}
	sep := lipgloss.NewStyle().Foreground(ColorDimmer).
		Render("  " + strings.Repeat("─", sepWidth))

	// Input
	inputLine := c.input.View()

	inner := lipgloss.JoinVertical(lipgloss.Left,
		chatContent,
		typingLine,
		sep,
		inputLine,
	)
	return ChatBorderStyle.Width(c.width).Height(c.height).Padding(1, 0).Render(inner)
}

func (c ChatView) overlayLogBox(base string) string {
	logW := c.viewport.Width() / 3
	if logW < 20 {
		logW = 20
	}
	if logW > 40 {
		logW = 40
	}

	dim := lipgloss.NewStyle().Foreground(ColorDimmer).Italic(true)
	var logLines []string
	for _, e := range c.sysLogs {
		logLines = append(logLines, dim.Render(truncateWidth(e.text, logW-2)))
	}
	logContent := strings.Join(logLines, "\n")

	box := lipgloss.NewStyle().
		Foreground(ColorDimmer).
		Width(logW).
		Padding(0, 1).
		Render(logContent)

	baseLines := strings.Split(base, "\n")
	boxLines := strings.Split(box, "\n")

	// Position at bottom-right
	startY := len(baseLines) - len(boxLines)
	startX := c.viewport.Width() - logW
	if startY < 0 {
		startY = 0
	}
	if startX < 0 {
		startX = 0
	}

	for i, bLine := range boxLines {
		row := startY + i
		if row >= len(baseLines) {
			break
		}
		baseLine := baseLines[row]
		baseRunes := []rune(stripAnsi(baseLine))

		var b strings.Builder
		if startX > 0 && startX <= len(baseRunes) {
			b.WriteString(string(baseRunes[:startX]))
		} else if startX > len(baseRunes) {
			b.WriteString(string(baseRunes))
			b.WriteString(strings.Repeat(" ", startX-len(baseRunes)))
		}
		b.WriteString(bLine)
		baseLines[row] = b.String()
	}

	return strings.Join(baseLines, "\n")
}

func (c ChatView) renderTypingIndicator() string {
	c.ClearStaleTyping()

	if len(c.typingUsers) == 0 {
		return "  " // keep the line height consistent
	}

	var names []string
	for nick := range c.typingUsers {
		names = append(names, nick)
	}

	dots := typingFrames[c.typingFrame%len(typingFrames)]
	var text string
	switch len(names) {
	case 1:
		text = fmt.Sprintf("%s is typing%s", names[0], dots)
	case 2:
		text = fmt.Sprintf("%s and %s are typing%s", names[0], names[1], dots)
	default:
		text = fmt.Sprintf("%d people are typing%s", len(names), dots)
	}

	return TypingStyle.Render("    " + text)
}

// InputValue returns current input text and clears it.
func (c *ChatView) InputValue() string {
	val := c.input.Value()
	c.input.Reset()
	return val
}

// HasInput returns true if the user has typed something.
func (c *ChatView) HasInput() bool {
	return c.input.Value() != ""
}

// detectMentionTrigger checks if the cursor is inside an @query.
// Returns (query, true) if active, ("", false) otherwise.
func (c *ChatView) detectMentionTrigger() (string, bool) {
	val := c.input.Value()
	pos := c.input.Position()
	if pos == 0 {
		return "", false
	}

	// Walk backwards from cursor to find @
	text := val[:pos]
	atIdx := -1
	for i := len(text) - 1; i >= 0; i-- {
		if text[i] == ' ' || text[i] == '\n' {
			break
		}
		if text[i] == '@' {
			atIdx = i
			break
		}
	}
	if atIdx < 0 {
		return "", false
	}
	// @ must be at start or after whitespace
	if atIdx > 0 && text[atIdx-1] != ' ' {
		return "", false
	}
	query := text[atIdx+1:]
	return query, true
}

// completeMention replaces @query with @fullname in the input.
func (c *ChatView) completeMention(name string) {
	val := c.input.Value()
	pos := c.input.Position()
	text := val[:pos]

	// Find the @ that started this mention
	atIdx := strings.LastIndex(text, "@")
	if atIdx < 0 {
		return
	}

	replacement := "@" + name + " "
	newVal := val[:atIdx] + replacement + val[pos:]
	c.input.SetValue(newVal)
	c.input.SetCursor(atIdx + len(replacement))
	c.mentionPopup = false
	c.mentionQuery = ""
	c.mentionNames = nil
	c.mentionCursor = 0
}

// UpdateMentionPopup refreshes the autocomplete popup state.
// onlineNames is the list of users currently in the room.
func (c *ChatView) UpdateMentionPopup(onlineNames []string) {
	query, active := c.detectMentionTrigger()
	if !active {
		c.mentionPopup = false
		c.mentionQuery = ""
		c.mentionNames = nil
		c.mentionCursor = 0
		return
	}
	c.mentionPopup = true
	c.mentionQuery = query
	c.mentionNames = fuzzy.Match(query, onlineNames)
	// Reset cursor if out of bounds
	if c.mentionCursor >= len(c.mentionNames) {
		c.mentionCursor = 0
	}
}

// MentionPopupActive returns true if the autocomplete popup is showing with results.
func (c *ChatView) MentionPopupActive() bool {
	return c.mentionPopup && len(c.mentionNames) > 0
}
