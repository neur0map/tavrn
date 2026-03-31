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
	Rooms          []RoomInfo
	CurrentRoom    string
	Width          int
	Height         int
	MentionCounts  map[string]int // room name → unread mention count
	ActivityCounts map[string]int // room name → messages in last 10min
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
	Users   []string
	Width   int
	Height  int
	Frame   int // for animated online dots
	Tankard *TankardView
}

func NewOnlinePanel() OnlinePanel {
	return OnlinePanel{}
}

// Animated dot cycles for online presence
var onlineDotFrames = []string{"●", "●", "◉", "●"}

const tankardHeight = 6 // 4 art lines + 1 counter + 1 separator

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

	usersContent := b.String()

	// Render tankard below users if there's room
	if o.Tankard != nil && o.Height >= 20 {
		tankardArt := o.Tankard.View()
		tankardSep := dimmer.Render(strings.Repeat("─", o.Width-4))

		// Calculate padding to push tankard to bottom
		// Inner height = Height - border(0) - paddingTop(1)
		innerH := o.Height - 1
		usersLines := strings.Count(usersContent, "\n") + 1
		tankardLines := strings.Count(tankardArt, "\n") + 1
		sepLine := 1
		gap := innerH - usersLines - tankardLines - sepLine
		if gap < 0 {
			gap = 0
		}

		full := usersContent + strings.Repeat("\n", gap) + tankardSep + "\n" + tankardArt
		return RightSidebarStyle.
			Width(o.Width).
			Height(o.Height).
			MaxHeight(o.Height).
			Render(full)
	}

	return RightSidebarStyle.
		Width(o.Width).
		Height(o.Height).
		MaxHeight(o.Height).
		Render(usersContent)
}
