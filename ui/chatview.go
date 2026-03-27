package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"tavrn/internal/chat"
)

// Typing dots animation frames
var typingFrames = []string{"   ", ".  ", ".. ", "..."}

type ChatView struct {
	viewport    viewport.Model
	input       textinput.Model
	messages    []chat.Message
	typingUsers map[string]time.Time // nick → last typing time
	typingFrame int
	width       int
	height      int
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
	c.messages = append(c.messages, msg)
	c.renderMessages()
	c.viewport.GotoBottom()
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
	// Re-render to update relative timestamps
	c.renderMessages()
}

func (c *ChatView) renderMessages() {
	var lines []string
	now := time.Now()

	prevNick := ""
	for i, msg := range c.messages {
		if msg.IsSystem {
			// System messages: dimmed, centered feel
			sysText := lipgloss.NewStyle().Foreground(ColorDim).Italic(true).Render(
				"    " + msg.Text)
			lines = append(lines, sysText)
			if i < len(c.messages)-1 {
				lines = append(lines, "")
			}
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
			nick := NickStyle(msg.ColorIndex).Render(msg.Nickname)
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
	if width <= 0 || len(text) <= width {
		return []string{text}
	}
	var lines []string
	words := strings.Fields(text)
	current := ""
	for _, word := range words {
		if current == "" {
			current = word
		} else if len(current)+1+len(word) <= width {
			current += " " + word
		} else {
			lines = append(lines, current)
			current = word
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

	c.input, cmd = c.input.Update(msg)
	cmds = append(cmds, cmd)

	c.viewport, cmd = c.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return c, tea.Batch(cmds...)
}

func (c ChatView) View() string {
	chatContent := c.viewport.View()

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
