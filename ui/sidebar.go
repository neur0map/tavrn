package ui

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

type RoomInfo struct {
	Name  string
	Count int
}

type Sidebar struct {
	Rooms       []RoomInfo
	OnlineUsers []string
	Width       int
	Height      int
}

func NewSidebar() Sidebar {
	return Sidebar{
		Rooms: []RoomInfo{{Name: "lounge", Count: 0}},
	}
}

func (s Sidebar) View() string {
	header := lipgloss.NewStyle().Bold(true).Foreground(ColorSand)

	// Online users
	content := header.Render("NOW ONLINE") + "\n"
	for _, u := range s.OnlineUsers {
		bullet := lipgloss.NewStyle().Foreground(lipgloss.Color("108")).Render("*")
		content += fmt.Sprintf(" %s %s\n", bullet, u)
	}

	content += "\n"
	content += header.Render("ROOMS") + "\n"
	for _, r := range s.Rooms {
		line := fmt.Sprintf(" #%-10s %d", r.Name, r.Count)
		content += line + "\n"
	}

	content += "\n"
	content += header.Render("UP NEXT") + "\n"
	content += lipgloss.NewStyle().Foreground(ColorDim).Render(" (coming soon)") + "\n"

	return SidebarStyle.
		Width(s.Width).
		Height(s.Height).
		MaxHeight(s.Height).
		Render(content)
}
