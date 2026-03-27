package ui

import (
	"fmt"
	"image/color"
	"math/rand"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const tavernArt = `      _____
     /     \
    | () () |
     \ ___ /
   __|_____|__
  /  |     |  \
 |   | TAV |   |
 |   | ERN |   |
 |   |_____|   |
 |  /       \  |
 |_/  [] []  \_|
   |  []  [] |
   |_________|
   |_|_|_|_|_|`

var artGradientPairs = [][2]color.Color{
	{lipgloss.Color("137"), lipgloss.Color("94")},
	{lipgloss.Color("172"), lipgloss.Color("137")},
	{lipgloss.Color("179"), lipgloss.Color("172")},
	{lipgloss.Color("180"), lipgloss.Color("179")},
	{lipgloss.Color("179"), lipgloss.Color("172")},
	{lipgloss.Color("172"), lipgloss.Color("137")},
}

var enterPulse = []string{
	"[ ENTER ]",
	"[ ENTER ]",
	"[  >>>  ]",
	"[  >>>  ]",
}

// Bright enough to actually see against dark bg
var sparkChars = []string{"✦", "·", "✧", "°", "∘", "⋅", "*", "•"}
var sparkColors = []color.Color{
	lipgloss.Color("94"),  // brown
	lipgloss.Color("136"), // amber
	lipgloss.Color("137"), // copper
	lipgloss.Color("179"), // gold
	lipgloss.Color("172"), // orange
	lipgloss.Color("243"), // grey
	lipgloss.Color("108"), // sage
	lipgloss.Color("140"), // lavender
}

type spark struct {
	x, y    int
	charIdx int
	colIdx  int
	speed   int
	tick    int
}

type splashTickMsg time.Time

type Splash struct {
	nickname    string
	fingerprint string
	flair       bool
	width       int
	height      int
	frame       int
	sparks      []spark
	rng         *rand.Rand
	inited      bool
}

func NewSplash(nickname, fingerprint string, flair bool) Splash {
	return Splash{
		nickname:    nickname,
		fingerprint: fingerprint,
		flair:       flair,
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *Splash) initSparks() {
	if s.width == 0 || s.height == 0 {
		return
	}
	count := (s.width * s.height) / 25
	if count > 300 {
		count = 300
	}
	s.sparks = make([]spark, count)
	for i := range s.sparks {
		s.sparks[i] = spark{
			x:       s.rng.Intn(s.width),
			y:       s.rng.Intn(s.height),
			charIdx: s.rng.Intn(len(sparkChars)),
			colIdx:  s.rng.Intn(len(sparkColors)),
			speed:   1 + s.rng.Intn(3),
			tick:    s.rng.Intn(10),
		}
	}
	s.inited = true
}

func (s *Splash) tickSparks() {
	for i := range s.sparks {
		sp := &s.sparks[i]
		sp.tick++
		if sp.tick%sp.speed == 0 {
			sp.y--
			if s.rng.Intn(2) == 0 {
				sp.x += s.rng.Intn(3) - 1
			}
			if s.rng.Intn(5) == 0 {
				sp.charIdx = s.rng.Intn(len(sparkChars))
				sp.colIdx = s.rng.Intn(len(sparkColors))
			}
			if sp.y < 0 || sp.x < 0 || sp.x >= s.width {
				sp.y = s.height - 1 - s.rng.Intn(4)
				sp.x = s.rng.Intn(s.width)
			}
		}
	}
}

func splashTick() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
		return splashTickMsg(t)
	})
}

func (s Splash) Init() tea.Cmd {
	return splashTick()
}

func (s Splash) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		if !s.inited {
			s.initSparks()
		}
		return s, nil
	case splashTickMsg:
		s.frame++
		s.tickSparks()
		return s, splashTick()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter", "y":
			return s, func() tea.Msg { return EnterTavernMsg{} }
		case "q", "ctrl+c":
			return s, tea.Quit
		}
	}
	return s, nil
}

func (s Splash) View() tea.View {
	if s.width == 0 || s.height == 0 {
		v := tea.NewView("")
		v.AltScreen = true
		return v
	}

	card := s.renderCard()
	box := SplashBorderStyle.Render(card)

	// Build full-screen output line by line
	// First: create the plain background with sparks
	sparkMap := make(map[[2]int]spark, len(s.sparks))
	for _, sp := range s.sparks {
		if sp.y >= 0 && sp.y < s.height && sp.x >= 0 && sp.x < s.width {
			sparkMap[[2]int{sp.x, sp.y}] = sp
		}
	}

	// Get box dimensions
	boxLines := strings.Split(box, "\n")
	boxH := len(boxLines)
	boxW := 0
	for _, l := range boxLines {
		w := lipgloss.Width(l)
		if w > boxW {
			boxW = w
		}
	}

	startY := (s.height - boxH) / 2
	startX := (s.width - boxW) / 2
	if startY < 0 {
		startY = 0
	}
	if startX < 0 {
		startX = 0
	}

	endY := startY + boxH
	endX := startX + boxW

	var screenLines []string
	boxIdx := 0

	for y := 0; y < s.height; y++ {
		var line strings.Builder

		if y >= startY && y < endY && boxIdx < len(boxLines) {
			// This row has the box: left sparks + box + right sparks
			// Left margin
			for x := 0; x < startX; x++ {
				if sp, ok := sparkMap[[2]int{x, y}]; ok {
					c := sparkColors[sp.colIdx%len(sparkColors)]
					ch := sparkChars[sp.charIdx%len(sparkChars)]
					line.WriteString(lipgloss.NewStyle().Foreground(c).Render(ch))
				} else {
					line.WriteRune(' ')
				}
			}
			// Box content
			line.WriteString(boxLines[boxIdx])
			boxIdx++
			// Right margin
			for x := endX; x < s.width; x++ {
				if sp, ok := sparkMap[[2]int{x, y}]; ok {
					c := sparkColors[sp.colIdx%len(sparkColors)]
					ch := sparkChars[sp.charIdx%len(sparkChars)]
					line.WriteString(lipgloss.NewStyle().Foreground(c).Render(ch))
				} else {
					line.WriteRune(' ')
				}
			}
		} else {
			// Full width sparks row
			for x := 0; x < s.width; x++ {
				if sp, ok := sparkMap[[2]int{x, y}]; ok {
					c := sparkColors[sp.colIdx%len(sparkColors)]
					ch := sparkChars[sp.charIdx%len(sparkChars)]
					line.WriteString(lipgloss.NewStyle().Foreground(c).Render(ch))
				} else {
					line.WriteRune(' ')
				}
			}
		}

		screenLines = append(screenLines, line.String())
	}

	v := tea.NewView(strings.Join(screenLines, "\n"))
	v.AltScreen = true
	return v
}

func (s Splash) renderCard() string {
	pair := artGradientPairs[s.frame%len(artGradientPairs)]

	var b strings.Builder

	diag := GradientText(strings.Repeat("╱", 44), pair[0], pair[1], false)
	b.WriteString(diag)
	b.WriteString("\n\n")

	title := GradientText("TAVRN.SH", pair[1], pair[0], true)
	b.WriteString(centerText(title, 8, 44))
	b.WriteString("\n")
	sub := SplashSubtitleStyle.Render("a quiet place in the terminal")
	b.WriteString(centerText(sub, 29, 44))
	b.WriteString("\n\n")

	artLines := strings.Split(tavernArt, "\n")
	for _, line := range artLines {
		colored := GradientText(line, pair[0], pair[1], false)
		b.WriteString(colored)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	nick := s.nickname
	if s.flair {
		nick = "~" + nick
	}
	b.WriteString(SplashDescStyle.Render("you are ") + NickStyle(0).Render(nick))
	b.WriteString("\n")
	fpShort := s.fingerprint
	if len(fpShort) > 16 {
		fpShort = fpShort[:16]
	}
	b.WriteString(SplashDescStyle.Render(fmt.Sprintf("key: %s...", fpShort)))

	b.WriteString("\n\n")
	b.WriteString(SplashCategoryStyle.Render("COMMANDS"))
	b.WriteString("\n")
	for _, c := range []struct{ cmd, desc string }{
		{"/nick NAME", "change your handle"},
		{"/who", "see who's around"},
		{"/help", "show all commands"},
	} {
		b.WriteString(fmt.Sprintf("  %s %s\n",
			SplashCommandStyle.Width(16).Render(c.cmd),
			SplashDescStyle.Render(c.desc)))
	}

	b.WriteString("\n")
	b.WriteString(SplashCategoryStyle.Render("KEYS"))
	b.WriteString("\n")
	for _, k := range []struct{ key, desc string }{
		{"ENTER", "send message"},
		{"CTRL+C", "exit tavern"},
		{"ESC", "close modals"},
	} {
		b.WriteString(fmt.Sprintf("  %s %s\n",
			SplashCommandStyle.Width(16).Render(k.key),
			SplashDescStyle.Render(k.desc)))
	}

	b.WriteString("\n")
	b.WriteString(SplashDescStyle.Italic(true).Render("all data purged every sunday 23:59 UTC"))
	b.WriteString("\n")
	b.WriteString(SplashDescStyle.Italic(true).Render("nothing is permanent. draw while you can."))

	b.WriteString("\n\n")
	enterFrame := enterPulse[s.frame%len(enterPulse)]
	enterKey := SplashKeyStyle.Render(enterFrame)
	enterDesc := lipgloss.NewStyle().Foreground(ColorSand).Render(" enter the tavern")
	quitKey := SplashKeyStyle.Render("[ Q ]")
	quitDesc := lipgloss.NewStyle().Foreground(ColorDim).Render(" exit")
	b.WriteString(enterKey + enterDesc + "    " + quitKey + quitDesc)

	b.WriteString("\n\n")
	bottomPair := artGradientPairs[(s.frame+3)%len(artGradientPairs)]
	b.WriteString(GradientText(strings.Repeat("╱", 44), bottomPair[0], bottomPair[1], false))
	b.WriteString("\n")
	b.WriteString(centerText(SplashDescStyle.Render("[ v0.2 ]"), 8, 44))

	return b.String()
}

func centerText(rendered string, rawLen, totalWidth int) string {
	pad := (totalWidth - rawLen) / 2
	if pad <= 0 {
		return rendered
	}
	return strings.Repeat(" ", pad) + rendered
}

type EnterTavernMsg struct{}
type ShowHelpMsg struct{}
