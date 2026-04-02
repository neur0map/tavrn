package ui

import (
	"fmt"
	"strings"
	"time"

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
	Rooms          []RoomInfo
	CurrentRoom    string
	Width          int
	Height         int
	MentionCounts  map[string]int // room name → unread mention count
	ActivityCounts map[string]int // room name → messages in last 10min
	SSHLinks       []string
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

	contentW := r.Width - 3 // border(1) + paddingLR(2)

	for _, rm := range r.Rooms {
		isCurrent := rm.Name == r.CurrentRoom
		name := "#" + rm.Name

		// Build right-aligned section: online count + optional badges
		countStr := fmt.Sprintf("%d", rm.Count)
		var badgeStr string
		var badgeW int
		if !isCurrent && r.ActivityCounts != nil {
			if ac := r.ActivityCounts[rm.Name]; ac > 0 {
				part := fmt.Sprintf(" %d", ac)
				badgeStr += lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(part)
				badgeW += len(part)
			}
		}
		if r.MentionCounts != nil {
			if mc := r.MentionCounts[rm.Name]; mc > 0 {
				part := fmt.Sprintf(" @%d", mc)
				badgeStr += lipgloss.NewStyle().Foreground(ColorAmber).Bold(true).Render(part)
				badgeW += len(part)
			}
		}

		// indicator(1) + name + gap(>=1) + count + badges = contentW
		rightW := len(countStr) + badgeW
		gap := contentW - 1 - lipgloss.Width(name) - rightW
		if gap < 1 {
			gap = 1
		}
		padding := strings.Repeat(" ", gap)

		if isCurrent {
			indicator := lipgloss.NewStyle().Foreground(ColorHighlight).Render("▸")
			roomName := lipgloss.NewStyle().Foreground(ColorAmber).Bold(true).Render(name)
			roomCount := lipgloss.NewStyle().Foreground(ColorDim).Render(countStr)
			b.WriteString(indicator + roomName + padding + roomCount + badgeStr + "\n")
		} else {
			roomName := lipgloss.NewStyle().Foreground(ColorSand).Render(name)
			roomCount := lipgloss.NewStyle().Foreground(ColorDimmer).Render(countStr)
			b.WriteString(" " + roomName + padding + roomCount + badgeStr + "\n")
		}
	}

	// ── Other SSH section ──
	if len(r.SSHLinks) > 0 {
		dimmer := lipgloss.NewStyle().Foreground(ColorDimmer)
		addrStyle := lipgloss.NewStyle().Foreground(ColorAccent)

		b.WriteString("\n")
		b.WriteString(dimmer.Render(strings.Repeat("─", r.Width-4)))
		b.WriteString("\n")
		b.WriteString(header.Render("OTHER SSH"))
		b.WriteString("\n")

		maxW := r.Width - 4
		if maxW < 10 {
			maxW = 10
		}
		for _, addr := range r.SSHLinks {
			b.WriteString(addrStyle.Render(truncateWidth(addr, maxW)) + "\n")
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

type LeaderboardMini struct {
	Name   string
	Level  int
	Points int
}

type OnlinePanel struct {
	Users       []string
	Width       int
	Height      int
	Frame       int // for animated online dots
	Tankard     *TankardView
	Leaderboard []LeaderboardMini
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

	// World clock
	b.WriteString("\n")
	b.WriteString(dimmer.Render(strings.Repeat("─", o.Width-4)))
	b.WriteString("\n")
	b.WriteString(renderWorldClock(o.Width - 4))

	// Mini leaderboard
	if len(o.Leaderboard) > 0 {
		accent := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
		amber := lipgloss.NewStyle().Foreground(ColorAmber)

		b.WriteString(dimmer.Render(strings.Repeat("─", o.Width-4)))
		b.WriteString("\n")
		b.WriteString(accent.Render("HACKERS"))
		b.WriteString("\n")

		maxW := o.Width - 6
		for i, e := range o.Leaderboard {
			if i >= 3 {
				break
			}
			rank := fmt.Sprintf("#%d", i+1)
			name := e.Name
			if len(name) > maxW-12 {
				name = name[:maxW-15] + "..."
			}
			pts := fmt.Sprintf("%d", e.Points)

			if i == 0 {
				b.WriteString(amber.Render(rank+" "+name) + dim.Render(" "+pts))
			} else {
				b.WriteString(dim.Render(rank+" "+name) + dimmer.Render(" "+pts))
			}
			b.WriteString("\n")
		}
	}

	usersContent := b.String()

	// Render tankard below everything
	if o.Tankard != nil && o.Height >= 20 {
		tankardArt := o.Tankard.View()
		tankardSep := dimmer.Render(strings.Repeat("─", o.Width-4))
		usersContent += "\n" + tankardSep + "\n" + tankardArt
	}

	return RightSidebarStyle.
		Width(o.Width).
		Height(o.Height).
		MaxHeight(o.Height).
		Render(usersContent)
}

func renderWorldClock(maxW int) string {
	dim := lipgloss.NewStyle().Foreground(ColorDim)
	dimmer := lipgloss.NewStyle().Foreground(ColorDimmer)
	accent := lipgloss.NewStyle().Foreground(ColorAmber)

	now := time.Now().UTC()

	type city struct {
		label  string
		offset int // hours from UTC
	}
	cities := []city{
		{"NYC", -4},
		{"CHI", -5},
		{"LDN", 0},
		{"BER", 2},
		{"TYO", 9},
	}

	var b strings.Builder
	for _, c := range cities {
		t := now.Add(time.Duration(c.offset) * time.Hour)
		hour := t.Format("3:04")
		ampm := t.Format("pm")

		h := t.Hour()
		var bar string
		if h >= 6 && h < 18 {
			bar = accent.Render("*")
		} else {
			bar = dimmer.Render(".")
		}

		b.WriteString(bar + " ")
		b.WriteString(dim.Render(c.label + " "))
		b.WriteString(accent.Render(hour))
		b.WriteString(dimmer.Render(ampm))
		b.WriteString("\n")
	}

	return b.String()
}
