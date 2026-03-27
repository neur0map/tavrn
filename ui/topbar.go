package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// waveChars are the Unicode block characters for the wave animation.
var waveChars = []string{"░", "▁", "▃", "▅", "▇", "▅", "▃", "▁", "░"}

type TopBar struct {
	Room        string
	OnlineCount int
	WeeklyCount int
	Width       int
	Frame       int

	// Jukebox
	NowTitle    string
	NowArtist   string
	NowSource   string
	NowDuration time.Duration
	NowPosition time.Duration
	HasTrack    bool
}

func NewTopBar() TopBar {
	return TopBar{Room: "lounge"}
}

func (t TopBar) waveView() string {
	if !t.HasTrack {
		return ""
	}
	var wave strings.Builder
	for i := range 9 {
		idx := (i + t.Frame) % len(waveChars)
		wave.WriteString(waveChars[idx])
	}
	return lipgloss.NewStyle().Foreground(ColorMusic).Render(wave.String())
}

func (t TopBar) nowPlayingView() string {
	if !t.HasTrack {
		return ""
	}

	note := lipgloss.NewStyle().Foreground(ColorMusic).Render("♪")
	title := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(t.NowTitle)
	dot := lipgloss.NewStyle().Foreground(ColorDimmer).Render(" · ")
	artist := lipgloss.NewStyle().Foreground(ColorDim).Render(t.NowArtist)
	wave := t.waveView()

	pos := formatDuration(t.NowPosition)
	dur := formatDuration(t.NowDuration)
	timeStr := lipgloss.NewStyle().Foreground(ColorDimmer).Render(
		fmt.Sprintf(" %s/%s", pos, dur))

	return fmt.Sprintf("%s %s%s%s %s%s", note, title, dot, artist, wave, timeStr)
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

	// Line 2: #room left | now playing center | online right
	room := lipgloss.NewStyle().Foreground(ColorAmber).Bold(true).Render(
		fmt.Sprintf("  #%s", t.Room))

	onlineDot := lipgloss.NewStyle().Foreground(ColorGreen).Render("●")
	onlineNum := lipgloss.NewStyle().Foreground(ColorSand).Bold(true).Render(
		fmt.Sprintf("%d online", t.OnlineCount))
	weekly := lipgloss.NewStyle().Foreground(ColorDim).Render(
		fmt.Sprintf("%d this week", t.WeeklyCount))
	dot := lipgloss.NewStyle().Foreground(ColorDimmer).Render(" · ")

	statsRight := fmt.Sprintf("%s %s%s%s  ", onlineDot, onlineNum, dot, weekly)

	nowPlaying := t.nowPlayingView()

	roomW := lipgloss.Width(room)
	statsW := lipgloss.Width(statsRight)
	nowW := lipgloss.Width(nowPlaying)

	if nowPlaying != "" {
		totalUsed := roomW + statsW + nowW
		remainingGap := t.Width - totalUsed
		leftGap := remainingGap / 2
		rightGap := remainingGap - leftGap
		if leftGap < 1 {
			leftGap = 1
		}
		if rightGap < 1 {
			rightGap = 1
		}
		statsLine := room + strings.Repeat(" ", leftGap) + nowPlaying + strings.Repeat(" ", rightGap) + statsRight
		visW := lipgloss.Width(statsLine)
		if visW < t.Width {
			statsLine += strings.Repeat(" ", t.Width-visW)
		}

		border := lipgloss.NewStyle().Foreground(ColorBorder).Render(
			"  " + strings.Repeat("─", t.Width-4) + "  ")

		return lipgloss.JoinVertical(lipgloss.Left, diagLine, statsLine, border)
	}

	// No track playing — original layout
	gap := t.Width - roomW - statsW
	if gap < 1 {
		gap = 1
	}
	statsLine := room + strings.Repeat(" ", gap) + statsRight

	border := lipgloss.NewStyle().Foreground(ColorBorder).Render(
		"  " + strings.Repeat("─", t.Width-4) + "  ")

	return lipgloss.JoinVertical(lipgloss.Left, diagLine, statsLine, border)
}

func formatDuration(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}
