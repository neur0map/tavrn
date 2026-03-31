package ui

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

type BottomBar struct {
	Width        int
	IsGallery    bool
	IsTankard    bool
	MentionCount int
}

func NewBottomBar() BottomBar {
	return BottomBar{}
}

func (b BottomBar) View() string {
	k := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)
	d := lipgloss.NewStyle().Foreground(ColorDim)
	sep := lipgloss.NewStyle().Foreground(ColorDimmer).Render("  ·  ")

	var content string
	if b.IsTankard {
		content = "  " +
			k.Render("SPACE") + " " + d.Render("drink") + sep +
			k.Render("ESC") + " " + d.Render("back")
	} else if b.IsGallery {
		content = "  " +
			k.Render("P") + " " + d.Render("post") + sep +
			k.Render("E") + " " + d.Render("expand") + sep +
			k.Render("D") + " " + d.Render("delete") + sep +
			k.Render("TAB") + " " + d.Render("select")
	} else {
		f4 := k.Render("F4") + " " + d.Render("mentions")
		if b.MentionCount > 0 {
			f4 = k.Render("F4") + " " + d.Render(fmt.Sprintf("mentions(%d)", b.MentionCount))
		}
		content = "  " +
			k.Render("F1") + " " + d.Render("help") + sep +
			k.Render("F2") + " " + d.Render("nick") + sep +
			k.Render("F3") + " " + d.Render("rooms") + sep +
			f4 + sep +
			k.Render("F6") + " " + d.Render("tankard") + sep +
			k.Render("SHIFT+↑↓") + " " + d.Render("scroll")
	}

	return BottomBarStyle.Width(b.Width).MaxWidth(b.Width).Render(content)
}
