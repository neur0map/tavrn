package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

type TopBar struct {
	Room        string
	OnlineCount int
	WeeklyCount int
	NowPlaying  string
	Width       int
	Frame       int
}

func NewTopBar() TopBar {
	return TopBar{Room: "lounge"}
}

func (t TopBar) View() string {
	if t.Width < 20 {
		return ""
	}

	// Line 1: Diagonal fill with TAVRN.SH embedded
	label := " TAVRN.SH "
	fillTotal := t.Width - len(label)
	leftN := fillTotal / 2
	rightN := fillTotal - leftN
	if leftN < 0 {
		leftN = 0
	}
	if rightN < 0 {
		rightN = 0
	}
	diagFill := lipgloss.NewStyle().Foreground(ColorBorder)
	diagTitle := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)
	diagLine := diagFill.Render(strings.Repeat("╱", leftN)) +
		diagTitle.Render(label) +
		diagFill.Render(strings.Repeat("╱", rightN))

	// Line 2: Stats left, title center, room right
	onlineDot := lipgloss.NewStyle().Foreground(ColorGreen).Render("●")
	onlineNum := lipgloss.NewStyle().Foreground(ColorSand).Bold(true).Render(
		fmt.Sprintf("%d online", t.OnlineCount))
	weekly := lipgloss.NewStyle().Foreground(ColorDim).Render(
		fmt.Sprintf("%d this week", t.WeeklyCount))
	dot := lipgloss.NewStyle().Foreground(ColorDimmer).Render(" · ")

	statsLeft := fmt.Sprintf("  %s %s%s%s", onlineDot, onlineNum, dot, weekly)

	titleText := "TAVRN.SH"
	title := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(titleText)

	room := lipgloss.NewStyle().Foreground(ColorAmber).Bold(true).Render(
		fmt.Sprintf("#%s  ", t.Room))

	leftW := lipgloss.Width(statsLeft)
	rightW := lipgloss.Width(room)
	centerPos := (t.Width - len(titleText)) / 2
	gapLeft := centerPos - leftW
	gapRight := t.Width - centerPos - len(titleText) - rightW
	if gapLeft < 1 {
		gapLeft = 1
	}
	if gapRight < 1 {
		gapRight = 1
	}

	statsLine := statsLeft + strings.Repeat(" ", gapLeft) + title + strings.Repeat(" ", gapRight) + room

	// Line 3: Border
	border := lipgloss.NewStyle().Foreground(ColorBorder).Render(
		"  " + strings.Repeat("─", t.Width-4) + "  ")

	return lipgloss.JoinVertical(lipgloss.Left, diagLine, statsLine, border)
}
