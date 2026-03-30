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
	Rooms         []RoomInfo
	CurrentRoom   string
	Width         int
	Height        int
	MentionCounts map[string]int // room name → unread mention count
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
			line := indicator + roomName + roomCount
			if r.MentionCounts != nil {
				if mc := r.MentionCounts[rm.Name]; mc > 0 {
					line += lipgloss.NewStyle().Foreground(ColorAmber).Bold(true).
						Render(fmt.Sprintf(" (%d)", mc))
				}
			}
			b.WriteString(line + "\n")
		} else {
			// Other rooms: dimmer
			roomName := lipgloss.NewStyle().Foreground(ColorSand).Render(name)
			roomCount := lipgloss.NewStyle().Foreground(ColorDimmer).Render(count)
			line := " " + roomName + roomCount
			if r.MentionCounts != nil {
				if mc := r.MentionCounts[rm.Name]; mc > 0 {
					line += lipgloss.NewStyle().Foreground(ColorAmber).Bold(true).
						Render(fmt.Sprintf(" (%d)", mc))
				}
			}
			b.WriteString(line + "\n")
		}
	}

	return LeftSidebarStyle.
		Width(r.Width).
		Height(r.Height).
		MaxHeight(r.Height).
		Render(b.String())
}

// ─────────────────────────────────────
// Right sidebar: Online users + Up Next
// ─────────────────────────────────────

const maxVisibleUsers = 5

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
	dim := lipgloss.NewStyle().Foreground(ColorDim)
	dimmer := lipgloss.NewStyle().Foreground(ColorDimmer)

	var b strings.Builder

	// ── Online section ──
	b.WriteString(header.Render("ONLINE"))
	b.WriteString("\n")
	sep := dimmer.Render(strings.Repeat("─", o.Width-4))
	b.WriteString(sep)
	b.WriteString("\n")

	if len(o.Users) == 0 {
		b.WriteString(dim.Render("(empty)"))
	} else {
		dot := onlineDotFrames[o.Frame%len(onlineDotFrames)]
		dotStyle := lipgloss.NewStyle().Foreground(ColorGreen).Render(dot)

		shown := o.Users
		overflow := 0
		if len(shown) > maxVisibleUsers {
			overflow = len(shown) - maxVisibleUsers
			shown = shown[:maxVisibleUsers]
		}

		maxNameW := o.Width - 6
		if maxNameW < 8 {
			maxNameW = 8
		}
		for _, u := range shown {
			fmt.Fprintf(&b, "%s %s\n", dotStyle, truncateWidth(u, maxNameW))
		}
		if overflow > 0 {
			b.WriteString(dim.Render(fmt.Sprintf("  +%d more\n", overflow)))
		}
	}

	return RightSidebarStyle.
		Width(o.Width).
		Height(o.Height).
		MaxHeight(o.Height).
		Render(b.String())
}
