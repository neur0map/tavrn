package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tavrn.sh/internal/jukebox"
)

type JukeboxModal struct {
	engine      *jukebox.Engine
	selectedTab int // cursor position in genre tab bar
}

func NewJukeboxModal(engine *jukebox.Engine, userFP string) JukeboxModal {
	// Initialize cursor to the currently active genre
	active := int(engine.State().ActiveGenre)
	return JukeboxModal{
		engine:      engine,
		selectedTab: active,
	}
}

func (m JukeboxModal) Update(msg tea.Msg) (JukeboxModal, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	genres := jukebox.AllGenres()

	switch keyMsg.String() {
	case "left", "h":
		if m.selectedTab > 0 {
			m.selectedTab--
		}
		m.engine.SetGenre(genres[m.selectedTab])
	case "right", "l":
		if m.selectedTab < len(genres)-1 {
			m.selectedTab++
		}
		m.engine.SetGenre(genres[m.selectedTab])
	}
	return m, nil
}

func (m JukeboxModal) View(width, height int) string {
	modalW := 44

	// Header
	headerText := " ♪ Tavern Radio "
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

	// Genre tab bar
	state := m.engine.State()
	genres := jukebox.AllGenres()
	var tabs []string
	for i, g := range genres {
		label := g.String()
		switch {
		case i == m.selectedTab:
			// Selected tab: bracketed and highlighted
			tabs = append(tabs, lipgloss.NewStyle().
				Foreground(ColorMusic).Bold(true).
				Render("["+label+"]"))
		case g == state.ActiveGenre:
			// Active but not selected: bright
			tabs = append(tabs, lipgloss.NewStyle().
				Foreground(ColorHighlight).
				Render(label))
		default:
			tabs = append(tabs, lipgloss.NewStyle().
				Foreground(ColorDim).
				Render(label))
		}
	}
	b.WriteString("  " + strings.Join(tabs, "  "))
	b.WriteString("\n")

	// "Switching after this track" indicator
	if state.PendingGenre != state.ActiveGenre {
		nextLabel := state.PendingGenre.String()
		b.WriteString("  " + lipgloss.NewStyle().Foreground(ColorDim).Italic(true).
			Render("↳ switching to "+nextLabel+" next"))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Now playing
	if state.Current == nil {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Italic(true).Render(
			"  Loading..."))
		b.WriteString("\n")
	} else {
		title := truncateWidth(state.Current.Title, modalW-4)
		b.WriteString("  " + lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(title))
		b.WriteString("\n")
		artist := truncateWidth(state.Current.Artist, modalW-4)
		b.WriteString("  " + lipgloss.NewStyle().Foreground(ColorSand).Render(artist))
		b.WriteString("\n\n")

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

		dot := lipgloss.NewStyle().Foreground(ColorGreen).Render("●")
		listeners := lipgloss.NewStyle().Foreground(ColorDim).Render(
			fmt.Sprintf("%d listening", state.Listeners))
		b.WriteString("  " + dot + " playing · " + listeners + "\n")
	}

	b.WriteString("\n")
	footerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(
		strings.Repeat("╱", modalW))
	b.WriteString(footerFill)
	b.WriteString("\n")

	// Hotkey help
	lrKey := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("←→")
	escKey := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("ESC")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render(
		fmt.Sprintf("  %s genre  %s close", lrKey, escKey)))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2).
		Render(b.String())
}
