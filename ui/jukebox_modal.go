package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tavrn.sh/internal/jukebox"
)

// JukeboxSkipMsg signals a skip vote from the user.
type JukeboxSkipMsg struct{}

type JukeboxModal struct {
	engine *jukebox.Engine
	userFP string
}

func NewJukeboxModal(engine *jukebox.Engine, userFP string) JukeboxModal {
	return JukeboxModal{engine: engine, userFP: userFP}
}

func (m JukeboxModal) Update(msg tea.Msg) (JukeboxModal, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc":
			return m, func() tea.Msg { return CloseModalMsg{} }
		case "s", "S":
			return m, func() tea.Msg { return JukeboxSkipMsg{} }
		}
	}
	return m, nil
}

func (m JukeboxModal) View(width, height int) string {
	modalW := 44

	// Header
	headerText := " ♪ Lofi Radio "
	fillLen := modalW - lipgloss.Width(headerText)
	if fillLen < 4 {
		fillLen = 4
	}
	leftFill := strings.Repeat("╱", fillLen/2)
	rightFill := strings.Repeat("╱", fillLen-fillLen/2)

	headerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(leftFill)
	headerTitle := lipgloss.NewStyle().Foreground(ColorMusic).Bold(true).Render(headerText)
	headerFillR := lipgloss.NewStyle().Foreground(ColorBorder).Render(rightFill)

	var b strings.Builder
	b.WriteString(headerFill + headerTitle + headerFillR)
	b.WriteString("\n\n")

	state := m.engine.State()

	if state.Current == nil {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Italic(true).Render(
			"  Loading..."))
		b.WriteString("\n")
	} else {
		// Title + artist
		title := truncateWidth(state.Current.Title, modalW-4)
		b.WriteString("  " + lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(title))
		b.WriteString("\n")
		artist := truncateWidth(state.Current.Artist, modalW-4)
		b.WriteString("  " + lipgloss.NewStyle().Foreground(ColorSand).Render(artist))
		b.WriteString("\n\n")

		// Progress bar (only when duration is known)
		if state.Current.Duration > 0 {
			barWidth := modalW - 14
			if barWidth < 10 {
				barWidth = 10
			}
			progress := state.Position.Seconds() / float64(state.Current.Duration)
			if progress > 1.0 {
				progress = 1.0
			}
			filled := int(float64(barWidth) * progress)
			empty := barWidth - filled
			bar := lipgloss.NewStyle().Foreground(ColorMusic).Render(strings.Repeat("▓", filled)) +
				lipgloss.NewStyle().Foreground(ColorDimmer).Render(strings.Repeat("░", empty))
			pos := formatDuration(state.Position)
			dur := formatDuration(state.Current.DurationTime())
			timeStr := lipgloss.NewStyle().Foreground(ColorDim).Render(
				fmt.Sprintf(" %s/%s", pos, dur))
			b.WriteString("  " + bar + timeStr + "\n\n")
		} else {
			b.WriteString("  " + lipgloss.NewStyle().Foreground(ColorDim).Italic(true).Render("buffering...") + "\n\n")
		}

		// Status line
		dot := lipgloss.NewStyle().Foreground(ColorGreen).Render("●")
		listeners := lipgloss.NewStyle().Foreground(ColorDim).Render(
			fmt.Sprintf("%d listening", state.Listeners))
		b.WriteString("  " + dot + " playing · " + listeners + "\n")
	}

	// Footer
	b.WriteString("\n")
	footerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(
		strings.Repeat("╱", modalW))
	b.WriteString(footerFill)
	b.WriteString("\n")

	// Skip + ESC
	sKey := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("S")
	escKey := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("ESC")

	skipLabel := m.skipLabel(state)
	b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render(
		fmt.Sprintf("  %s %s   %s close", sKey, skipLabel, escKey)))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2).
		Render(b.String())
}

func (m JukeboxModal) skipLabel(state jukebox.EngineState) string {
	voted := m.engine.UserSkipped(m.userFP)

	if state.SkipThreshold <= 1 {
		if voted {
			return lipgloss.NewStyle().Foreground(ColorGreen).Render("skip ✓ skipping...")
		}
		return "skip"
	}

	remaining := state.SkipThreshold - state.SkipVotes
	if remaining <= 0 {
		return lipgloss.NewStyle().Foreground(ColorGreen).Render("skip ✓ skipping...")
	}

	if voted {
		check := lipgloss.NewStyle().Foreground(ColorGreen).Render("✓")
		if remaining == 1 {
			return fmt.Sprintf("skip %s (1 more vote)", check)
		}
		return fmt.Sprintf("skip %s (%d more votes)", check, remaining)
	}

	if remaining == 1 {
		return "skip (1 more vote)"
	}
	return fmt.Sprintf("skip (%d more votes)", remaining)
}
