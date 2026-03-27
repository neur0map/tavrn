package ui

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

type TopBar struct {
	Room        string
	OnlineCount int
	WeeklyCount int
	NowPlaying  string
	Width       int
}

func NewTopBar() TopBar {
	return TopBar{Room: "lounge"}
}

func (t TopBar) View() string {
	left := fmt.Sprintf(" tavrn.sh * #%s * %d online * %d this week",
		t.Room, t.OnlineCount, t.WeeklyCount)

	right := ""
	if t.NowPlaying != "" {
		right = fmt.Sprintf("  ~ Now Playing: %s ", t.NowPlaying)
	}

	gap := t.Width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	padding := ""
	for i := 0; i < gap; i++ {
		padding += " "
	}

	content := left + padding + right
	return TopBarStyle.Width(t.Width).MaxWidth(t.Width).Render(content)
}
