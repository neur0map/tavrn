package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// WargameSignupMsg signals the user wants to join wargames.
type WargameSignupMsg struct{}

type WargameRulesModal struct {
	wargame       string
	isParticipant bool
}

func NewWargameRulesModal(wargame string, isParticipant bool) WargameRulesModal {
	return WargameRulesModal{wargame: wargame, isParticipant: isParticipant}
}

func (w WargameRulesModal) Update(msg tea.Msg) (WargameRulesModal, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc", "q":
			return w, func() tea.Msg { return CloseModalMsg{} }
		case "enter":
			return w, func() tea.Msg { return CloseModalMsg{} }
		case "y":
			if !w.isParticipant {
				w.isParticipant = true
				return w, func() tea.Msg { return WargameSignupMsg{} }
			}
		}
	}
	return w, nil
}

func (w WargameRulesModal) View(width, height int) string {
	highlight := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)
	accent := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	dim := lipgloss.NewStyle().Foreground(ColorDim)
	dimmer := lipgloss.NewStyle().Foreground(ColorDimmer)
	green := lipgloss.NewStyle().Foreground(ColorGreen)
	amber := lipgloss.NewStyle().Foreground(ColorAmber)

	name := strings.ToUpper(w.wargame)

	var b strings.Builder

	b.WriteString(highlight.Render("WARGAME: "+name) + "\n")
	b.WriteString(dimmer.Render(strings.Repeat("─", 38)) + "\n\n")

	b.WriteString(accent.Render("WHAT IS THIS") + "\n")
	b.WriteString(dim.Render("  Practice hacking challenges from") + "\n")
	b.WriteString(dim.Render("  OverTheWire and other wargames.") + "\n")
	b.WriteString(dim.Render("  Solve levels, submit flags, earn") + "\n")
	b.WriteString(dim.Render("  points and climb the leaderboard.") + "\n\n")

	b.WriteString(accent.Render("HOW TO PLAY") + "\n")
	b.WriteString(dim.Render("  1. Sign up with ") + green.Render("Y") + dim.Render(" below") + "\n")
	b.WriteString(dim.Render("  2. Go to ") + amber.Render("overthewire.org") + "\n")
	b.WriteString(dim.Render("  3. Solve levels, find the flag") + "\n")
	b.WriteString(dim.Render("  4. Type ") + green.Render("/submit") + dim.Render(" to enter it") + "\n")
	b.WriteString(dim.Render("  5. Earn points and climb ranks") + "\n\n")

	b.WriteString(accent.Render("POINTS") + "\n")
	b.WriteString(dim.Render("  Level N = N points") + "\n")
	b.WriteString(dim.Render("  Lv.1=1  Lv.5=5  Lv.10=10") + "\n")
	b.WriteString(dim.Render("  Points are permanent.") + "\n\n")

	// Status + controls
	b.WriteString(dimmer.Render(strings.Repeat("─", 38)) + "\n")
	if w.isParticipant {
		b.WriteString(green.Render("  SIGNED UP") + dim.Render(" — you're on the board") + "\n\n")
		b.WriteString(dimmer.Render("ENTER") + dim.Render(" continue  ") +
			dimmer.Render("ESC") + dim.Render(" close"))
	} else {
		b.WriteString(amber.Render("  NOT SIGNED UP") + "\n\n")
		b.WriteString(green.Bold(true).Render("Y") + dim.Render(" sign up  ") +
			dimmer.Render("ESC") + dim.Render(" close"))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2).
		Render(b.String())
}
