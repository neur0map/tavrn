package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	tankardFrameCount    = 3
	tankardFrameInterval = 150 * time.Millisecond
)

type tankardTickMsg struct{}

type TankardView struct {
	count     int
	frame     int  // 0 = idle, 1-2 = press animation
	animating bool // animation in progress
	focused   bool
	width     int
}

func NewTankardView() TankardView {
	return TankardView{}
}

// Compact tankard — 3 frames, 4 lines tall.
var tankardFrames = [tankardFrameCount]string{
	// Frame 0: idle
	"  ╭━━━╮  \n" +
		"  ┃▓▓▓┃╮ \n" +
		"  ┃▓▓▓┃│ \n" +
		"  ╰━━━╯╯ ",
	// Frame 1: pressed — squish + splash
	"  ╭━━━╮  \n" +
		" *┃▓▓▓┃╮ \n" +
		"  ┃▒▒▒┃│ \n" +
		"  ╰━━━╯╯ ",
	// Frame 2: bounce back — foam
	" ·    · \n" +
		"  ╭━━━╮  \n" +
		"  ┃▓▓▓┃╮ \n" +
		"  ╰━━━╯╯ ",
}

func (t TankardView) Update(msg tea.Msg) (TankardView, tea.Cmd) {
	if _, ok := msg.(tankardTickMsg); ok {
		if !t.animating {
			return t, nil
		}
		t.frame++
		if t.frame >= tankardFrameCount {
			t.frame = 0
			t.animating = false
			return t, nil
		}
		return t, t.tickCmd()
	}
	return t, nil
}

// Press increments the counter. Blocked while animating (~300ms)
// which prevents hold-repeat from counting.
func (t *TankardView) Press() tea.Cmd {
	if t.animating {
		return nil
	}
	t.count++
	t.animating = true
	t.frame = 1
	return t.tickCmd()
}

func (t TankardView) tickCmd() tea.Cmd {
	return tea.Tick(tankardFrameInterval, func(time.Time) tea.Msg {
		return tankardTickMsg{}
	})
}

func formatCount(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	s := fmt.Sprintf("%d", n)
	var parts []string
	for i := len(s); i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		parts = append([]string{s[start:i]}, parts...)
	}
	return strings.Join(parts, ",")
}

func (t TankardView) View() string {
	art := tankardFrames[t.frame]

	var mugColor, fillColor, handleColor, splashColor lipgloss.Style

	if t.focused {
		mugColor = lipgloss.NewStyle().Foreground(ColorAmber).Bold(true)
		fillColor = lipgloss.NewStyle().Foreground(lipgloss.Color("178"))
		handleColor = lipgloss.NewStyle().Foreground(lipgloss.Color("136"))
		splashColor = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Bold(true)
	} else {
		mugColor = lipgloss.NewStyle().Foreground(ColorDim)
		fillColor = lipgloss.NewStyle().Foreground(ColorDimmer)
		handleColor = lipgloss.NewStyle().Foreground(ColorDimmer)
		splashColor = lipgloss.NewStyle().Foreground(ColorDim)
	}

	lines := strings.Split(art, "\n")
	var colored []string
	for _, line := range lines {
		var out strings.Builder
		for _, ch := range line {
			switch ch {
			case '▓', '▒':
				out.WriteString(fillColor.Render(string(ch)))
			case '╮', '│', '╯':
				out.WriteString(handleColor.Render(string(ch)))
			case '*', '·':
				out.WriteString(splashColor.Render(string(ch)))
			default:
				out.WriteString(mugColor.Render(string(ch)))
			}
		}
		colored = append(colored, out.String())
	}

	result := strings.Join(colored, "\n")

	var counterStyle lipgloss.Style
	if t.focused {
		counterStyle = lipgloss.NewStyle().Foreground(ColorAmber).Bold(true)
	} else {
		counterStyle = lipgloss.NewStyle().Foreground(ColorDim)
	}
	counter := counterStyle.Render(fmt.Sprintf("  x %s", formatCount(t.count)))
	result += "\n" + counter

	return result
}
