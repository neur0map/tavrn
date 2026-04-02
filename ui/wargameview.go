package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// Wargame descriptions for the mission briefing header.
var wargameInfo = map[string]struct {
	target string
	desc   string
}{
	"bandit":    {"overthewire.org/bandit", "Linux basics, SSH, file permissions"},
	"natas":     {"overthewire.org/natas", "Web security, server-side exploits"},
	"leviathan": {"overthewire.org/leviathan", "Reverse engineering, binary exploitation"},
}

// WargameHeader renders the mission briefing header for a wargame room.
func WargameHeader(wargame string, currentLevel, maxLevel, points, width int) string {
	accent := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	highlight := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)
	dim := lipgloss.NewStyle().Foreground(ColorDim)
	dimmer := lipgloss.NewStyle().Foreground(ColorDimmer)
	green := lipgloss.NewStyle().Foreground(ColorGreen)

	name := strings.ToUpper(wargame)
	info, ok := wargameInfo[wargame]
	if !ok {
		info.target = "overthewire.org"
		info.desc = "Hacking challenges"
	}

	var b strings.Builder

	// Mission header
	b.WriteString("  " + accent.Render("[ "+name+" ]"))
	nextLevel := fmt.Sprintf("  next: level %d", currentLevel+1)
	if currentLevel >= maxLevel && maxLevel > 0 {
		nextLevel = "  " + green.Render("COMPLETE")
	}
	b.WriteString(highlight.Render(nextLevel))
	b.WriteString("\n")

	// Target + description
	b.WriteString("  " + dimmer.Render("target: ") + dim.Render(info.target))
	b.WriteString("\n")
	b.WriteString("  " + dimmer.Render(info.desc))
	b.WriteString("\n")

	// Separator with /submit hint
	sepW := width - 6
	if sepW < 10 {
		sepW = 10
	}
	hint := green.Render("/submit")
	sepLeft := sepW - 10
	if sepLeft < 4 {
		sepLeft = 4
	}
	b.WriteString("  " + dimmer.Render(strings.Repeat("─", sepLeft)+" ") + hint + dimmer.Render(" ─"))
	b.WriteString("\n")

	return b.String()
}
