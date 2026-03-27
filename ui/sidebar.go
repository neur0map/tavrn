package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

type RoomInfo struct {
	Name  string
	Count int
}

// ─────────────────────────────────────
// Left sidebar: Rooms / Channels
// ─────────────────────────────────────

type RoomsPanel struct {
	Rooms       []RoomInfo
	CurrentRoom string
	Width       int
	Height      int
}

func NewRoomsPanel() RoomsPanel {
	return RoomsPanel{CurrentRoom: "lounge"}
}

func (r RoomsPanel) View() string {
	header := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)

	var b strings.Builder
	b.WriteString(header.Render("ROOMS"))
	b.WriteString("\n")
	sep := lipgloss.NewStyle().Foreground(ColorDimmer).Render(
		strings.Repeat("─", r.Width-4))
	b.WriteString(sep)
	b.WriteString("\n")

	for _, rm := range r.Rooms {
		isCurrent := rm.Name == r.CurrentRoom
		name := fmt.Sprintf("#%s", rm.Name)
		count := fmt.Sprintf(" %d", rm.Count)

		if isCurrent {
			// Active room: highlighted with indicator
			indicator := lipgloss.NewStyle().Foreground(ColorHighlight).Render("▸")
			roomName := lipgloss.NewStyle().Foreground(ColorAmber).Bold(true).Render(name)
			roomCount := lipgloss.NewStyle().Foreground(ColorDim).Render(count)
			b.WriteString(indicator + roomName + roomCount + "\n")
		} else {
			// Other rooms: dimmer
			roomName := lipgloss.NewStyle().Foreground(ColorSand).Render(name)
			roomCount := lipgloss.NewStyle().Foreground(ColorDimmer).Render(count)
			b.WriteString(" " + roomName + roomCount + "\n")
		}
	}

	return SidebarStyle.
		Width(r.Width).
		Height(r.Height).
		MaxHeight(r.Height).
		Padding(1, 1).
		Render(b.String())
}

// ─────────────────────────────────────
// Right sidebar: Online users
// ─────────────────────────────────────

type OnlinePanel struct {
	Users  []string
	Width  int
	Height int
	Frame  int // for animated online dots
}

func NewOnlinePanel() OnlinePanel {
	return OnlinePanel{}
}

// Animated dot cycles for online presence
var onlineDotFrames = []string{"●", "●", "◉", "●"}

func (o OnlinePanel) View() string {
	header := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)

	var b strings.Builder
	b.WriteString(header.Render("ONLINE"))
	b.WriteString("\n")
	sep := lipgloss.NewStyle().Foreground(ColorDimmer).Render(
		strings.Repeat("─", o.Width-4))
	b.WriteString(sep)
	b.WriteString("\n")

	if len(o.Users) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render("(empty)"))
	} else {
		dot := onlineDotFrames[o.Frame%len(onlineDotFrames)]
		dotStyle := lipgloss.NewStyle().Foreground(ColorGreen).Render(dot)
		for _, u := range o.Users {
			b.WriteString(fmt.Sprintf("%s %s\n", dotStyle, u))
		}
	}

	return SidebarStyle.
		Width(o.Width).
		Height(o.Height).
		MaxHeight(o.Height).
		Padding(1, 1).
		Render(b.String())
}
