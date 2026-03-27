package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const tavernArt = `
        .  *  .     *    .  *
    *  .    ___________    .
   .   *  /           \  *   .
      .  |  __|   |__  |  .
   *     | |  |   |  | |     *
     .   | |__|___|__| |   .
  *   .  |    |   |    |  .   *
      .  |  __|   |__  |  .
   .     |_/___________\_|     .
      .  |___|_|___|_|___|  .
    *    |   |       |   |    *
  .   .  |___|_______|___|  .   .
      *  |               |  *
   .     |_______________|     .
        .  *  .     *    .  *
`

type Splash struct {
	nickname    string
	fingerprint string
	flair       bool
	width       int
	height      int
}

func NewSplash(nickname, fingerprint string, flair bool) Splash {
	return Splash{
		nickname:    nickname,
		fingerprint: fingerprint,
		flair:       flair,
	}
}

func (s Splash) Init() tea.Cmd {
	return nil
}

func (s Splash) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		return s, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter", "y":
			return s, func() tea.Msg { return EnterTavernMsg{} }
		case "q", "ctrl+c":
			return s, tea.Quit
		case "?":
			return s, func() tea.Msg { return ShowHelpMsg{} }
		}
	}
	return s, nil
}

func (s Splash) View() tea.View {
	if s.width == 0 {
		v := tea.NewView("Loading...")
		v.AltScreen = true
		return v
	}

	// Build splash content
	var b strings.Builder

	// ASCII tavern art
	art := lipgloss.NewStyle().Foreground(ColorAccent).Render(tavernArt)
	b.WriteString(art)
	b.WriteString("\n")

	// Title
	title := SplashTitleStyle.Render("// WELCOME TO TAVRN //")
	b.WriteString(title)
	b.WriteString("\n")
	subtitle := SplashSubtitleStyle.Render("a quiet place in the terminal")
	b.WriteString(subtitle)
	b.WriteString("\n\n")

	// Identity
	nick := s.nickname
	if s.flair {
		nick = "~" + nick
	}
	identLine := fmt.Sprintf("you are %s", NickStyle(0).Render(nick))
	b.WriteString(identLine)
	b.WriteString("\n")
	fpShort := s.fingerprint
	if len(fpShort) > 16 {
		fpShort = fpShort[:16]
	}
	fpLine := SplashDescStyle.Render(fmt.Sprintf("key: %s...", fpShort))
	b.WriteString(fpLine)
	b.WriteString("\n")

	// Commands section
	b.WriteString("\n")
	b.WriteString(SplashCategoryStyle.Render("COMMANDS"))
	b.WriteString("\n")
	cmds := []struct{ cmd, desc string }{
		{"/nick NAME", "change your handle"},
		{"/who", "see who's around"},
		{"/help", "show help card"},
	}
	for _, c := range cmds {
		line := fmt.Sprintf("  %s  %s",
			SplashCommandStyle.Width(16).Render(c.cmd),
			SplashDescStyle.Render(c.desc),
		)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Keybinds
	b.WriteString("\n")
	b.WriteString(SplashCategoryStyle.Render("KEYS"))
	b.WriteString("\n")
	keys := []struct{ key, desc string }{
		{"CTRL+C", "exit tavern"},
		{"ENTER", "send message"},
		{"UP/DOWN", "scroll chat"},
	}
	for _, k := range keys {
		line := fmt.Sprintf("  %s  %s",
			SplashCommandStyle.Width(16).Render(k.key),
			SplashDescStyle.Render(k.desc),
		)
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Purge notice
	b.WriteString("\n")
	purge := SplashDescStyle.Render("all data purged every sunday 23:59 UTC")
	b.WriteString(purge)
	b.WriteString("\n")
	b.WriteString(SplashDescStyle.Render("nothing is permanent. draw while you can."))

	// Footer with actions
	b.WriteString("\n\n")
	enter := SplashKeyStyle.Render("[ ENTER ]") + " " + SplashDescStyle.Render("enter the tavern")
	quit := SplashKeyStyle.Render("[ Q ]") + " " + SplashDescStyle.Render("exit")
	footer := enter + "    " + quit
	b.WriteString(footer)

	// Version
	b.WriteString("\n\n")
	b.WriteString(SplashDescStyle.Render("[ v0.2 ]"))

	// Put it all in a bordered box, centered on screen
	box := SplashBorderStyle.Render(b.String())
	bgStyle := lipgloss.NewStyle().Background(ColorDarkBg)
	centered := lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, box,
		lipgloss.WithWhitespaceStyle(bgStyle),
	)

	v := tea.NewView(centered)
	v.AltScreen = true
	return v
}

// EnterTavernMsg signals transition from splash to main tavern UI.
type EnterTavernMsg struct{}

// ShowHelpMsg signals the help overlay should be shown.
type ShowHelpMsg struct{}
