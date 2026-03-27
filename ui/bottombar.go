package ui

import (
	"charm.land/lipgloss/v2"
)

type BottomBar struct {
	Width      int
	IsGallery  bool
}

func NewBottomBar() BottomBar {
	return BottomBar{}
}

func (b BottomBar) View() string {
	keyStyle := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(ColorDim)
	sep := lipgloss.NewStyle().Foreground(ColorDimmer).Render("  ·  ")

	var content string
	if b.IsGallery {
		// Gallery: single-key shortcuts
		content = "  " +
			keyStyle.Render("P") + " " + descStyle.Render("post") + sep +
			keyStyle.Render("J") + " " + descStyle.Render("rooms") + sep +
			keyStyle.Render("H") + " " + descStyle.Render("help") + sep +
			keyStyle.Render("D") + " " + descStyle.Render("delete") + sep +
			keyStyle.Render("TAB") + " " + descStyle.Render("select") + sep +
			keyStyle.Render("Q") + " " + descStyle.Render("exit")
	} else {
		// Chat: F-keys + slash commands
		content = "  " +
			keyStyle.Render("F1") + " " + descStyle.Render("help") + sep +
			keyStyle.Render("F2") + " " + descStyle.Render("nick") + sep +
			keyStyle.Render("F3") + " " + descStyle.Render("rooms") + sep +
			keyStyle.Render("F4") + " " + descStyle.Render("post") + sep +
			keyStyle.Render("/cmd") + " " + descStyle.Render("slash cmds")
	}

	return BottomBarStyle.Width(b.Width).MaxWidth(b.Width).Render(content)
}
