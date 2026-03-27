package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// Animated glyphs that cycle for the ambient pulse
var pulseFrames = []string{"·", ":", "·", " "}

type TopBar struct {
	Room        string
	OnlineCount int
	WeeklyCount int
	NowPlaying  string
	Width       int
	Frame       int // animation frame counter
}

func NewTopBar() TopBar {
	return TopBar{Room: "lounge"}
}

func (t TopBar) View() string {
	if t.Width < 20 {
		return ""
	}

	// Line 1: Brand banner
	diag := lipgloss.NewStyle().Foreground(ColorBorder).Bold(true).Render("╱╱╱")
	title := GradientText(" TAVRN.SH ", ColorHighlight, ColorAmber, true)
	diagR := lipgloss.NewStyle().Foreground(ColorBorder).Bold(true).Render("╱╱╱")

	// Animated pulse dot
	pulse := pulseFrames[t.Frame%len(pulseFrames)]
	pulseStyled := lipgloss.NewStyle().Foreground(ColorHighlight).Render(pulse)

	brandLine := fmt.Sprintf("  %s%s%s  %s", diag, title, diagR, pulseStyled)

	// Line 2: Stats
	onlineDot := lipgloss.NewStyle().Foreground(ColorGreen).Render("●")
	onlineNum := lipgloss.NewStyle().Foreground(ColorSand).Bold(true).Render(
		fmt.Sprintf("%d online", t.OnlineCount))
	weekly := lipgloss.NewStyle().Foreground(ColorDim).Render(
		fmt.Sprintf("%d this week", t.WeeklyCount))
	room := lipgloss.NewStyle().Foreground(ColorAmber).Bold(true).Render(
		fmt.Sprintf("#%s", t.Room))
	dot := lipgloss.NewStyle().Foreground(ColorDimmer).Render(" · ")

	statsLine := fmt.Sprintf("  %s %s%s%s%s%s", onlineDot, onlineNum, dot, weekly, dot, room)

	// Now playing on stats line if present
	if t.NowPlaying != "" {
		note := lipgloss.NewStyle().Foreground(ColorAmber).Render("♪")
		np := lipgloss.NewStyle().Foreground(ColorDim).Render(t.NowPlaying)
		statsLine += dot + note + " " + np
	}

	// Pad brand line
	brandPad := t.Width - lipgloss.Width(brandLine)
	if brandPad > 0 {
		brandLine += strings.Repeat(" ", brandPad)
	}

	// Pad stats line
	statsPad := t.Width - lipgloss.Width(statsLine)
	if statsPad > 0 {
		statsLine += strings.Repeat(" ", statsPad)
	}

	// Bottom border
	border := lipgloss.NewStyle().Foreground(ColorBorder).Render(
		"  " + strings.Repeat("─", t.Width-4))

	return lipgloss.JoinVertical(lipgloss.Left, brandLine, statsLine, border)
}
