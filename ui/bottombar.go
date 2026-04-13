package ui

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

type BottomBar struct {
	Width        int
	IsGallery    bool
	IsFeed       bool
	IsTankard    bool
	IsDMMode     bool
	MentionCount int
	DMUnread     int
}

func NewBottomBar() BottomBar {
	return BottomBar{}
}

func (b BottomBar) View() string {
	k := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)
	d := lipgloss.NewStyle().Foreground(ColorDim)
	sep := lipgloss.NewStyle().Foreground(ColorDimmer).Render("  ·  ")

	// DM badge
	dmBadge := ""
	if b.DMUnread > 0 {
		dmBadge = fmt.Sprintf("(%d)", b.DMUnread)
	}

	var content string
	if b.IsDMMode {
		content = "  " +
			k.Render("TAB") + " " + d.Render("tavern") + sep +
			k.Render("ESC") + " " + d.Render("back") + sep +
			k.Render("↑↓") + " " + d.Render("navigate") + sep +
			k.Render("ENTER") + " " + d.Render("open")
	} else if b.IsTankard {
		content = "  " +
			k.Render("SPACE") + " " + d.Render("drink") + sep +
			k.Render("ESC") + " " + d.Render("back")
	} else if b.IsGallery {
		content = "  " +
			k.Render("P") + " " + d.Render("post") + sep +
			k.Render("E") + " " + d.Render("expand") + sep +
			k.Render("D") + " " + d.Render("delete") + sep +
			k.Render("TAB") + " " + d.Render("select")
	} else if b.IsFeed {
		content = "  " +
			k.Render("TAB") + " " + d.Render("chat") + sep +
			k.Render("j/k") + " " + d.Render("scroll") + sep +
			k.Render("ENTER") + " " + d.Render("comments") + sep +
			k.Render("s") + " " + d.Render("share") + sep +
			k.Render("ESC") + " " + d.Render("back") + sep +
			k.Render("S-TAB") + " " + d.Render("hide feed")
	} else {
		f4 := k.Render("F4") + " " + d.Render("mentions")
		if b.MentionCount > 0 {
			f4 = k.Render("F4") + " " + d.Render(fmt.Sprintf("mentions(%d)", b.MentionCount))
		}
		tabDM := k.Render("TAB") + " " + d.Render("DMs"+dmBadge)
		content = "  " +
			k.Render("F1") + " " + d.Render("help") + sep +
			k.Render("F2") + " " + d.Render("nick") + sep +
			k.Render("F3") + " " + d.Render("rooms") + sep +
			f4 + sep +
			tabDM + sep +
			k.Render("F6") + " " + d.Render("tankard") + sep +
			k.Render("F7") + " " + d.Render("leaderboard") + sep +
			k.Render("SHIFT+↑↓") + " " + d.Render("scroll")
	}

	return BottomBarStyle.Width(b.Width).MaxWidth(b.Width).Render(content)
}
