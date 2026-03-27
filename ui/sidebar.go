package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"tavrn/internal/jukebox"
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
	Users    []string
	Queue    []jukebox.Request // populated from engine state
	NowTitle string
	Width    int
	Height   int
	Frame    int // for animated online dots
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

		for _, u := range shown {
			b.WriteString(fmt.Sprintf("%s %s\n", dotStyle, u))
		}
		if overflow > 0 {
			b.WriteString(dim.Render(fmt.Sprintf("  +%d more\n", overflow)))
		}
	}

	// ── Up Next section ──
	if o.NowTitle != "" || len(o.Queue) > 0 {
		b.WriteString("\n")
		b.WriteString(header.Render("UP NEXT"))
		b.WriteString("\n")
		b.WriteString(sep)
		b.WriteString("\n")

		if o.NowTitle != "" {
			note := lipgloss.NewStyle().Foreground(ColorMusic).Render("♪")
			title := o.NowTitle
			if len(title) > o.Width-6 {
				title = title[:o.Width-9] + "..."
			}
			nowStyle := lipgloss.NewStyle().Foreground(ColorHighlight).Render(title)
			b.WriteString(fmt.Sprintf("%s %s\n", note, nowStyle))
		}

		if len(o.Queue) > 0 {
			if o.NowTitle != "" {
				b.WriteString("\n")
			}
			limit := 5
			if len(o.Queue) < limit {
				limit = len(o.Queue)
			}
			for i := 0; i < limit; i++ {
				req := o.Queue[i]
				title := req.Track.Title
				if len(title) > o.Width-8 {
					title = title[:o.Width-11] + "..."
				}
				num := dimmer.Render(fmt.Sprintf("%d.", i+1))
				name := dim.Render(title)
				count := dimmer.Render(fmt.Sprintf(" %d", req.Count))
				b.WriteString(fmt.Sprintf("%s %s%s\n", num, name, count))
			}
			if len(o.Queue) > 5 {
				b.WriteString(dim.Render(fmt.Sprintf("  +%d more\n", len(o.Queue)-5)))
			}
		} else if o.NowTitle == "" {
			b.WriteString(dim.Italic(true).Render("(empty)"))
			b.WriteString("\n")
		}
	}

	return RightSidebarStyle.
		Width(o.Width).
		Height(o.Height).
		MaxHeight(o.Height).
		Render(b.String())
}
