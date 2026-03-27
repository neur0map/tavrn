package ui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type ModalType int

const (
	ModalNone ModalType = iota
	ModalHelp
	ModalNick
)

// CloseModalMsg signals modal should close.
type CloseModalMsg struct{}

// NickChangeMsg carries the new nickname from the modal.
type NickChangeMsg struct{ Nick string }

// ─────────────────────────────────────
// Help Modal
// ─────────────────────────────────────

type HelpModal struct {
	width  int
	height int
}

func NewHelpModal() HelpModal {
	return HelpModal{}
}

func (h HelpModal) View(width, height int) string {
	// Header with ╱╱╱ fill
	headerText := " Help "
	fillLen := 40 - len(headerText)
	if fillLen < 4 {
		fillLen = 4
	}
	leftFill := strings.Repeat("╱", fillLen/2)
	rightFill := strings.Repeat("╱", fillLen-fillLen/2)

	headerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(leftFill)
	headerTitle := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(headerText)
	headerFillR := lipgloss.NewStyle().Foreground(ColorBorder).Render(rightFill)
	header := headerFill + headerTitle + headerFillR

	// Sections
	cat := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	cmd := lipgloss.NewStyle().Foreground(ColorCommand).Bold(true)
	desc := lipgloss.NewStyle().Foreground(ColorDesc)

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")

	b.WriteString(cat.Render("CHAT"))
	b.WriteString("\n")
	cmds := []struct{ c, d string }{
		{"/join ROOM", "switch rooms"},
		{"/nick NAME", "change your handle"},
		{"/who", "see who's around"},
		{"/help", "show this card"},
	}
	for _, c := range cmds {
		b.WriteString(fmt.Sprintf("  %s  %s\n", cmd.Width(18).Render(c.c), desc.Render(c.d)))
	}

	b.WriteString("\n")
	b.WriteString(cat.Render("KEYS"))
	b.WriteString("\n")
	keys := []struct{ k, d string }{
		{"ENTER", "send message"},
		{"CTRL+C", "exit tavern"},
		{"ESC", "close modal"},
		{"UP / DOWN", "scroll chat"},
	}
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("  %s  %s\n", cmd.Width(18).Render(k.k), desc.Render(k.d)))
	}

	b.WriteString("\n")
	b.WriteString(cat.Render("INFO"))
	b.WriteString("\n")
	b.WriteString(desc.Italic(true).Render(
		"  All data purged every Sunday 23:59 UTC.\n"))
	b.WriteString(desc.Italic(true).Render(
		"  Nothing is permanent. Draw while you can."))

	// Footer
	b.WriteString("\n\n")
	footerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(
		strings.Repeat("╱", 40))
	b.WriteString(footerFill)
	b.WriteString("\n")
	esc := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("ESC")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render(
		fmt.Sprintf("  %s close", esc)))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2).
		Render(b.String())
}

// ─────────────────────────────────────
// Nick Modal
// ─────────────────────────────────────

type NickModal struct {
	input  textinput.Model
	width  int
	height int
	err    string
}

func NewNickModal(currentNick string) NickModal {
	ti := textinput.New()
	ti.Placeholder = "enter new nickname..."
	ti.Focus()
	ti.CharLimit = 20
	ti.Prompt = "> "
	ti.SetValue(currentNick)

	return NickModal{input: ti}
}

func (n NickModal) Update(msg tea.Msg) (NickModal, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return n, func() tea.Msg { return CloseModalMsg{} }
		case "enter":
			val := strings.TrimSpace(n.input.Value())
			if len(val) >= 2 && len(val) <= 20 {
				return n, func() tea.Msg { return NickChangeMsg{Nick: val} }
			}
			n.err = "nickname must be 2-20 characters"
			return n, nil
		}
	}

	var cmd tea.Cmd
	n.input, cmd = n.input.Update(msg)
	n.err = "" // clear error on typing
	return n, cmd
}

func (n NickModal) View(width, height int) string {
	// Header
	headerText := " Change Nickname "
	fillLen := 36 - len(headerText)
	if fillLen < 4 {
		fillLen = 4
	}
	leftFill := strings.Repeat("╱", fillLen/2)
	rightFill := strings.Repeat("╱", fillLen-fillLen/2)

	headerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(leftFill)
	headerTitle := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(headerText)
	headerFillR := lipgloss.NewStyle().Foreground(ColorBorder).Render(rightFill)
	header := headerFill + headerTitle + headerFillR

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render("  Enter a new nickname (2-20 chars)"))
	b.WriteString("\n\n")
	b.WriteString("  " + n.input.View())

	if n.err != "" {
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("131")).Render("  " + n.err))
	}

	// Footer
	b.WriteString("\n\n")
	footerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(
		strings.Repeat("╱", 36))
	b.WriteString(footerFill)
	b.WriteString("\n")
	enter := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("ENTER")
	esc := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("ESC")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render(
		fmt.Sprintf("  %s confirm  ·  %s cancel", enter, esc)))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2).
		Render(b.String())
}
