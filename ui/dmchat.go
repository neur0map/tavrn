package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"tavrn.sh/internal/dm"
)

type DMSendMsg struct {
	ToFP   string
	ToNick string
	Text   string
}

type DMBackToInboxMsg struct{}

type DMChat struct {
	peerFP        string
	peerNick      string
	ownFP         string
	ownNick       string
	ownColorIndex int
	viewport      viewport.Model
	input         textinput.Model
	messages      []dm.DirectMessage
	width, height int
}

func NewDMChat(peerFP, peerNick, ownFP, ownNick string, ownColorIndex int) DMChat {
	vp := viewport.New(viewport.WithWidth(80), viewport.WithHeight(10))
	ti := textinput.New()
	ti.Focus()
	ti.Placeholder = fmt.Sprintf("Message %s...", peerNick)
	ti.CharLimit = 500
	return DMChat{
		peerFP:        peerFP,
		peerNick:      peerNick,
		ownFP:         ownFP,
		ownNick:       ownNick,
		ownColorIndex: ownColorIndex,
		viewport:      vp,
		input:         ti,
	}
}

func (d *DMChat) SetSize(w, h int) {
	d.width = w
	d.height = h
	headerH := 3
	inputH := 2
	vpH := h - headerH - inputH
	if vpH < 2 {
		vpH = 2
	}
	d.viewport.SetWidth(w)
	d.viewport.SetHeight(vpH)
	d.input.SetWidth(w - 6)
	d.renderMessages()
}

func (d *DMChat) SetMessages(msgs []dm.DirectMessage) {
	d.messages = msgs
	d.renderMessages()
}

func (d *DMChat) AddMessage(msg dm.DirectMessage) {
	d.messages = append(d.messages, msg)
	d.renderMessages()
}

func (d *DMChat) renderMessages() {
	now := time.Now()
	wrapW := d.width - 10
	if wrapW < 20 {
		wrapW = 20
	}

	var lines []string
	prevFrom := ""

	for i, m := range d.messages {
		isOwn := m.FromFP == d.ownFP
		sameUser := m.FromFP == prevFrom
		prevFrom = m.FromFP

		if !sameUser {
			// Blank line between different speakers (not before first)
			if i > 0 {
				lines = append(lines, "")
			}

			// Nick + timestamp header
			var nickStyle lipgloss.Style
			if isOwn {
				nickStyle = NickStyle(d.ownColorIndex)
			} else {
				nickStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
			}
			nick := nickStyle.Render(m.FromNick)
			ts := dmFormatTime(m.CreatedAt, now)
			timeStr := lipgloss.NewStyle().Foreground(ColorDimmer).Render(ts)
			lines = append(lines, fmt.Sprintf("    %s  %s", nick, timeStr))
		}

		// Message text with word wrap
		wrapped := wordWrap(m.Text, wrapW)
		textStyle := lipgloss.NewStyle().Foreground(ColorSand)
		for _, wl := range wrapped {
			lines = append(lines, "      "+textStyle.Render(wl))
		}
	}

	// Pad with empty lines if few messages so content doesn't stick to top
	if len(lines) == 0 {
		dim := lipgloss.NewStyle().Foreground(ColorDimmer)
		lines = append(lines, "")
		lines = append(lines, dim.Render("    No messages yet. Say something."))
	}

	content := strings.Join(lines, "\n")
	d.viewport.SetContent(content)
	d.viewport.GotoBottom()
}

func (d DMChat) Update(msg tea.Msg) (DMChat, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			return d, func() tea.Msg { return DMBackToInboxMsg{} }
		case "enter":
			text := strings.TrimSpace(d.input.Value())
			if text != "" {
				d.input.Reset()
				return d, func() tea.Msg {
					return DMSendMsg{
						ToFP:   d.peerFP,
						ToNick: d.peerNick,
						Text:   text,
					}
				}
			}
			return d, nil
		case "shift+up", "pgup":
			d.viewport.ScrollUp(3)
			return d, nil
		case "shift+down", "pgdown":
			d.viewport.ScrollDown(3)
			return d, nil
		}
	}
	var cmd tea.Cmd
	d.input, cmd = d.input.Update(msg)
	return d, cmd
}

func (d DMChat) View() string {
	accent := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	dimmer := lipgloss.NewStyle().Foreground(ColorDimmer)
	highlight := lipgloss.NewStyle().Foreground(ColorHighlight)

	contentW := d.width - 4
	if contentW < 20 {
		contentW = 20
	}

	// Header
	var header strings.Builder
	header.WriteString("\n")
	header.WriteString("  " + accent.Render("DM  ") + highlight.Render(d.peerNick))
	header.WriteString("\n")
	header.WriteString("  " + dimmer.Render(strings.Repeat("─", contentW)))
	header.WriteString("\n")

	// Input area
	sep := dimmer.Render(strings.Repeat("─", contentW))
	prompt := lipgloss.NewStyle().Foreground(ColorHighlight).Render(" › ")
	inputLine := "\n" + "  " + sep + "\n" + prompt + d.input.View()

	return header.String() + d.viewport.View() + inputLine
}

// dmFormatTime shows relative or absolute time for DM messages.
func dmFormatTime(t time.Time, now time.Time) string {
	if t.IsZero() {
		return ""
	}
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
