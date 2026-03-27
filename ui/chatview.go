package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"tavrn/internal/chat"
)

type ChatView struct {
	viewport viewport.Model
	input    textinput.Model
	messages []chat.Message
	width    int
	height   int
}

func NewChatView() ChatView {
	ti := textinput.New()
	ti.Placeholder = "say something..."
	ti.Focus()
	ti.CharLimit = 500
	ti.Prompt = "> "

	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(10))

	return ChatView{
		viewport: vp,
		input:    ti,
		messages: make([]chat.Message, 0),
	}
}

func (c *ChatView) SetSize(width, height int) {
	c.width = width
	c.height = height
	inputHeight := 1
	borderHeight := 2
	vpW := width - borderHeight
	vpH := height - inputHeight - borderHeight
	if vpW < 1 {
		vpW = 1
	}
	if vpH < 1 {
		vpH = 1
	}
	c.viewport.SetWidth(vpW)
	c.viewport.SetHeight(vpH)
	c.input.SetWidth(width - 4)
}

func (c *ChatView) AddMessage(msg chat.Message) {
	c.messages = append(c.messages, msg)
	c.renderMessages()
	c.viewport.GotoBottom()
}

func (c *ChatView) renderMessages() {
	var lines []string
	for _, msg := range c.messages {
		if msg.IsSystem {
			line := SystemMsgStyle.Render("* " + msg.Text)
			lines = append(lines, line)
		} else {
			// Jelly-style: colored bar | nick timestamp\n  message
			barColor := NickBarColor(msg.ColorIndex)
			bar := lipgloss.NewStyle().Foreground(barColor).Render("|")
			nick := NickStyle(msg.ColorIndex).Render(msg.Nickname)
			ts := MsgTimeStyle.Render(msg.Timestamp.Format("15:04"))
			header := fmt.Sprintf(" %s %s %s", bar, nick, ts)
			body := fmt.Sprintf(" %s %s", bar, msg.Text)
			lines = append(lines, header)
			lines = append(lines, body)
		}
	}
	c.viewport.SetContent(strings.Join(lines, "\n"))
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
	inputLine := c.input.View()

	inner := lipgloss.JoinVertical(lipgloss.Left, chatContent, inputLine)
	return ChatBorderStyle.Width(c.width).Height(c.height).Render(inner)
}

// InputValue returns current input text and clears it.
func (c *ChatView) InputValue() string {
	val := c.input.Value()
	c.input.Reset()
	return val
}
