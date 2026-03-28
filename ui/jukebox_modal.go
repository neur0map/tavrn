package ui

import (
	"context"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"tavrn.sh/internal/jukebox"
)

// JukeboxSearchResultMsg carries search results back to the modal.
type JukeboxSearchResultMsg struct {
	Results []jukebox.Track
	Err     error
}

type jukeboxTab int

const (
	tabNowPlaying jukeboxTab = iota
	tabSearch
	tabVote
)

type JukeboxModal struct {
	tab    jukeboxTab
	engine *jukebox.Engine

	// Search tab
	searchInput   textinput.Model
	searchResults []jukebox.Track
	searchCursor  int
	searching     bool
	searchPending bool // one-shot flag consumed by SearchQuery()
	searchErr     string

	// Vote tab
	voteCursor int

	// Confirmation
	lastAdded string // track title just added

	// User fingerprint for vote tracking
	userFP string
}

func NewJukeboxModal(engine *jukebox.Engine, userFP string) JukeboxModal {
	ti := textinput.New()
	ti.Placeholder = "search for music..."
	ti.CharLimit = 100
	ti.Prompt = "> "

	return JukeboxModal{
		tab:         tabNowPlaying,
		engine:      engine,
		searchInput: ti,
		userFP:      userFP,
	}
}

func (m JukeboxModal) Update(msg tea.Msg) (JukeboxModal, tea.Cmd) {
	switch msg := msg.(type) {
	case JukeboxSearchResultMsg:
		m.searching = false
		if msg.Err != nil {
			m.searchErr = msg.Err.Error()
			m.searchResults = nil
		} else {
			m.searchResults = msg.Results
			m.searchErr = ""
		}
		m.searchCursor = 0
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return CloseModalMsg{} }
		case "tab":
			m.tab = (m.tab + 1) % 3
			if m.tab == tabSearch {
				m.searchInput.Focus()
			} else {
				m.searchInput.Blur()
			}
			return m, nil
		case "shift+tab":
			m.tab = (m.tab + 2) % 3
			if m.tab == tabSearch {
				m.searchInput.Focus()
			} else {
				m.searchInput.Blur()
			}
			return m, nil
		}

		switch m.tab {
		case tabSearch:
			return m.updateSearch(msg)
		case tabVote:
			return m.updateVote(msg)
		}
	}

	return m, nil
}

func (m JukeboxModal) updateSearch(msg tea.KeyPressMsg) (JukeboxModal, tea.Cmd) {
	switch msg.String() {
	case "ctrl+l":
		// Queue a random lofi track
		track := m.pickLofiTrack()
		if track != nil {
			m.lastAdded = track.Title
			m.tab = tabNowPlaying
			m.searchInput.Blur()
			return m, func() tea.Msg { return JukeboxAddMsg{Track: *track} }
		}
		return m, nil
	case "enter":
		if len(m.searchResults) > 0 && m.searchCursor < len(m.searchResults) {
			track := m.searchResults[m.searchCursor]
			m.lastAdded = track.Title
			m.tab = tabNowPlaying
			m.searchInput.Blur()
			return m, func() tea.Msg { return JukeboxAddMsg{Track: track} }
		}
		query := strings.TrimSpace(m.searchInput.Value())
		if query != "" && !m.searching {
			m.searching = true
			m.searchPending = true
			m.searchErr = ""
		}
		return m, nil
	case "up", "k":
		if len(m.searchResults) > 0 {
			m.searchCursor--
			if m.searchCursor < 0 {
				m.searchCursor = len(m.searchResults) - 1
			}
			return m, nil
		}
	case "down", "j":
		if len(m.searchResults) > 0 {
			m.searchCursor++
			if m.searchCursor >= len(m.searchResults) {
				m.searchCursor = 0
			}
			return m, nil
		}
	case "ctrl+s":
		query := strings.TrimSpace(m.searchInput.Value())
		if query != "" && !m.searching {
			m.searching = true
			m.searchPending = true
			m.searchErr = ""
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.searchResults = nil
	m.searchCursor = 0
	return m, cmd
}

func (m JukeboxModal) updateVote(msg tea.KeyPressMsg) (JukeboxModal, tea.Cmd) {
	state := m.engine.State()
	if len(state.Requests) == 0 {
		return m, nil
	}

	switch msg.String() {
	case "up", "k":
		m.voteCursor--
		if m.voteCursor < 0 {
			m.voteCursor = len(state.Requests) - 1
		}
	case "down", "j":
		m.voteCursor++
		if m.voteCursor >= len(state.Requests) {
			m.voteCursor = 0
		}
	case "enter":
		if m.voteCursor < len(state.Requests) {
			trackID := state.Requests[m.voteCursor].Track.ID
			return m, func() tea.Msg { return JukeboxVoteMsg{TrackID: trackID} }
		}
	}
	return m, nil
}

// pickLofiTrack returns a random lofi track from the lofi backend.
func (m JukeboxModal) pickLofiTrack() *jukebox.Track {
	if m.engine == nil {
		return nil
	}
	for _, b := range m.engine.Backends() {
		if b.Name() == "lofi" {
			tracks, err := b.Search(context.Background(), "popular", 1)
			if err == nil && len(tracks) > 0 {
				return &tracks[0]
			}
		}
	}
	return nil
}

// SearchQuery returns the current search query if a search was triggered.
// One-shot: returns true only once per trigger, then resets the pending flag.
func (m *JukeboxModal) SearchQuery() (string, bool) {
	if m.searchPending {
		m.searchPending = false
		return strings.TrimSpace(m.searchInput.Value()), true
	}
	return "", false
}

func (m JukeboxModal) View(width, height int) string {
	modalW := 52

	headerText := " ♪ Jukebox "
	fillLen := modalW - len(headerText) - 1
	if fillLen < 4 {
		fillLen = 4
	}
	leftFill := strings.Repeat("╱", fillLen/2)
	rightFill := strings.Repeat("╱", fillLen-fillLen/2)

	headerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(leftFill)
	headerTitle := lipgloss.NewStyle().Foreground(ColorMusic).Bold(true).Render(headerText)
	headerFillR := lipgloss.NewStyle().Foreground(ColorBorder).Render(rightFill)

	var b strings.Builder
	b.WriteString(headerFill + headerTitle + headerFillR)
	b.WriteString("\n\n")

	b.WriteString("  ")
	tabs := []struct {
		name string
		tab  jukeboxTab
	}{
		{"Now Playing", tabNowPlaying},
		{"Search", tabSearch},
		{"Vote", tabVote},
	}
	for i, t := range tabs {
		if i > 0 {
			b.WriteString(lipgloss.NewStyle().Foreground(ColorDimmer).Render("  "))
		}
		if t.tab == m.tab {
			b.WriteString(lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("[" + t.name + "]"))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render(" " + t.name + " "))
		}
	}
	b.WriteString("\n\n")

	switch m.tab {
	case tabNowPlaying:
		b.WriteString(m.viewNowPlaying(modalW))
	case tabSearch:
		b.WriteString(m.viewSearch(modalW))
	case tabVote:
		b.WriteString(m.viewVote(modalW))
	}

	b.WriteString("\n")
	footerFill := lipgloss.NewStyle().Foreground(ColorBorder).Render(
		strings.Repeat("╱", modalW))
	b.WriteString(footerFill)
	b.WriteString("\n")

	tabK := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("TAB")
	escK := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("ESC")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render(
		fmt.Sprintf("  %s switch  ·  %s close", tabK, escK)))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2).
		Render(b.String())
}

func (m JukeboxModal) viewNowPlaying(w int) string {
	state := m.engine.State()

	var b strings.Builder

	// Confirmation banner
	if m.lastAdded != "" {
		added := m.lastAdded
		if lipgloss.Width(added) > w-10 {
			runes := []rune(added)
			for lipgloss.Width(string(runes)) > w-13 && len(runes) > 0 {
				runes = runes[:len(runes)-1]
			}
			added = string(runes) + "..."
		}
		b.WriteString(lipgloss.NewStyle().Foreground(ColorGreen).Render(
			fmt.Sprintf("  ✓ Added \"%s\"", added)))
		b.WriteString("\n\n")
	}

	if state.Current == nil {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Italic(true).Render(
			"  No track playing"))
		b.WriteString("\n")
		return b.String()
	}

	// Now playing
	nowTitle := state.Current.Title
	maxNowTitle := w - 4
	if lipgloss.Width(nowTitle) > maxNowTitle {
		runes := []rune(nowTitle)
		for lipgloss.Width(string(runes)) > maxNowTitle-3 && len(runes) > 0 {
			runes = runes[:len(runes)-1]
		}
		nowTitle = string(runes) + "..."
	}
	title := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(nowTitle)
	artist := lipgloss.NewStyle().Foreground(ColorSand).Render(state.Current.Artist)
	source := lipgloss.NewStyle().Foreground(ColorDimmer).Render("[" + state.Current.Source + "]")

	b.WriteString("  " + title + "\n")
	b.WriteString("  " + artist + "  " + source + "\n\n")

	// Progress bar
	pos := formatDuration(state.Position)
	dur := formatDuration(state.Current.DurationTime())
	barWidth := w - 14
	if barWidth < 10 {
		barWidth = 10
	}
	progress := 0.0
	if state.Current.Duration > 0 {
		progress = state.Position.Seconds() / float64(state.Current.Duration)
		if progress > 1.0 {
			progress = 1.0
		}
	}
	filled := int(float64(barWidth) * progress)
	empty := barWidth - filled
	bar := lipgloss.NewStyle().Foreground(ColorMusic).Render(strings.Repeat("▓", filled)) +
		lipgloss.NewStyle().Foreground(ColorDimmer).Render(strings.Repeat("░", empty))
	timeStr := lipgloss.NewStyle().Foreground(ColorDim).Render(
		fmt.Sprintf(" %s/%s", pos, dur))
	b.WriteString("  " + bar + timeStr + "\n\n")

	// Up next queue
	if len(state.Requests) > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Render("  Up Next:"))
		b.WriteString("\n")
		limit := 5
		if len(state.Requests) < limit {
			limit = len(state.Requests)
		}
		for i := 0; i < limit; i++ {
			req := state.Requests[i]
			num := lipgloss.NewStyle().Foreground(ColorDim).Render(fmt.Sprintf("  %d.", i+1))
			reqTitle := req.Track.Title
			if len(reqTitle) > w-12 {
				reqTitle = reqTitle[:w-15] + "..."
			}
			name := lipgloss.NewStyle().Foreground(ColorSand).Render(reqTitle)
			count := lipgloss.NewStyle().Foreground(ColorDimmer).Render(
				fmt.Sprintf("  %d req", req.Count))
			fmt.Fprintf(&b, "%s %s%s\n", num, name, count)
		}
		if len(state.Requests) > 5 {
			b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render(
				fmt.Sprintf("  +%d more\n", len(state.Requests)-5)))
		}
		b.WriteString("\n")
	}

	// Phase + stats
	phaseStr := ""
	switch state.Phase {
	case jukebox.PhasePlaying:
		phaseStr = lipgloss.NewStyle().Foreground(ColorGreen).Render("● playing")
	case jukebox.PhaseIdle:
		phaseStr = lipgloss.NewStyle().Foreground(ColorDim).Render("● idle")
	}
	listeners := lipgloss.NewStyle().Foreground(ColorDim).Render(
		fmt.Sprintf("%d listening", state.Listeners))
	requests := lipgloss.NewStyle().Foreground(ColorDim).Render(
		fmt.Sprintf("%d requests", len(state.Requests)))
	dot := lipgloss.NewStyle().Foreground(ColorDimmer).Render(" · ")
	b.WriteString("  " + phaseStr + dot + listeners + dot + requests + "\n")

	return b.String()
}

func (m JukeboxModal) viewSearch(w int) string {
	var b strings.Builder

	b.WriteString("  " + m.searchInput.View() + "\n")

	if m.searching {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Italic(true).Render(
			"  searching..."))
		b.WriteString("\n")
		return b.String()
	}

	if m.searchErr != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("131")).Render(
			"  " + m.searchErr))
		b.WriteString("\n")
		return b.String()
	}

	if len(m.searchResults) > 0 {
		b.WriteString("\n")

		// Scrolling window: show 5 results around the cursor
		visible := 5
		total := len(m.searchResults)
		start := m.searchCursor - visible/2
		if start < 0 {
			start = 0
		}
		if start+visible > total {
			start = total - visible
		}
		if start < 0 {
			start = 0
		}
		end := start + visible
		if end > total {
			end = total
		}

		maxTitle := w - 16 // room for " ▸ N. " + " [source]"
		if maxTitle < 12 {
			maxTitle = 12
		}

		for i := start; i < end; i++ {
			track := m.searchResults[i]
			isSelected := i == m.searchCursor
			num := fmt.Sprintf("%d.", i+1)
			trackTitle := track.Title
			if lipgloss.Width(trackTitle) > maxTitle {
				runes := []rune(trackTitle)
				for lipgloss.Width(string(runes)) > maxTitle-3 && len(runes) > 0 {
					runes = runes[:len(runes)-1]
				}
				trackTitle = string(runes) + "..."
			}
			trackSource := "[" + track.Source + "]"

			if isSelected {
				indicator := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(" ▸ ")
				titleS := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(trackTitle)
				sourceS := lipgloss.NewStyle().Foreground(ColorDimmer).Render(trackSource)
				fmt.Fprintf(&b, "%s%s %s %s\n",
					indicator,
					lipgloss.NewStyle().Foreground(ColorDim).Render(num),
					titleS, sourceS)
			} else {
				titleS := lipgloss.NewStyle().Foreground(ColorSand).Render(trackTitle)
				sourceS := lipgloss.NewStyle().Foreground(ColorDimmer).Render(trackSource)
				fmt.Fprintf(&b, "   %s %s %s\n",
					lipgloss.NewStyle().Foreground(ColorDim).Render(num),
					titleS, sourceS)
			}
		}

		if total > visible {
			b.WriteString(lipgloss.NewStyle().Foreground(ColorDimmer).Render(
				fmt.Sprintf("  %d of %d results\n", m.searchCursor+1, total)))
		}

		b.WriteString("\n")
		jk := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("j/k")
		enter := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("ENTER")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render(
			fmt.Sprintf("  %s navigate  ·  %s add to requests", jk, enter)))
		b.WriteString("\n")
	} else {
		b.WriteString("\n")
		ctrlS := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("CTRL+S")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render(
			fmt.Sprintf("  type a query, press %s to search", ctrlS)))
		b.WriteString("\n")
	}

	// Lofi shortcut
	b.WriteString("\n")
	ctrlL := lipgloss.NewStyle().Foreground(ColorMusic).Bold(true).Render("CTRL+L")
	lofiLabel := lipgloss.NewStyle().Foreground(ColorDim).Render("queue lofi")
	fmt.Fprintf(&b, "  %s %s\n", ctrlL, lofiLabel)

	return b.String()
}

func (m JukeboxModal) viewVote(w int) string {
	state := m.engine.State()
	votedFor := m.engine.UserVotedFor(m.userFP)

	var b strings.Builder

	// Header
	b.WriteString(lipgloss.NewStyle().Foreground(ColorAccent).Bold(true).Render(
		"  VOTE FOR NEXT TRACK"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorDimmer).Render(
		"  most votes plays next"))
	b.WriteString("\n\n")

	if len(state.Requests) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Italic(true).Render(
			"  No tracks requested yet."))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Italic(true).Render(
			"  Search and add songs first!"))
		b.WriteString("\n")
		return b.String()
	}

	// Find max votes for bar scaling
	maxVotes := 0
	for _, req := range state.Requests {
		if req.Votes > maxVotes {
			maxVotes = req.Votes
		}
	}

	// Show up to 5 requests
	limit := 5
	if len(state.Requests) < limit {
		limit = len(state.Requests)
	}

	barMaxWidth := 8

	for i := 0; i < limit; i++ {
		req := state.Requests[i]
		isSelected := i == m.voteCursor
		isVoted := votedFor == req.Track.ID

		// Title (truncate)
		title := req.Track.Title
		maxTitle := w - 18
		if maxTitle < 12 {
			maxTitle = 12
		}
		if len(title) > maxTitle {
			title = title[:maxTitle-3] + "..."
		}

		// Vote bar
		barLen := 0
		if maxVotes > 0 {
			barLen = (req.Votes * barMaxWidth) / maxVotes
			if req.Votes > 0 && barLen == 0 {
				barLen = 1
			}
		}

		if isSelected {
			// Selected row — highlighted
			indicator := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(" ▸ ")
			titleS := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(title)
			b.WriteString(indicator + titleS)
		} else {
			titleS := lipgloss.NewStyle().Foreground(ColorSand).Render(title)
			b.WriteString("   " + titleS)
		}

		// Vote indicator
		if isVoted {
			b.WriteString(lipgloss.NewStyle().Foreground(ColorGreen).Bold(true).Render(" ✓"))
		}
		b.WriteString("\n")

		// Vote bar + count on next line (indented)
		if req.Votes > 0 {
			bar := lipgloss.NewStyle().Foreground(ColorMusic).Render(
				strings.Repeat("█", barLen))
			voteCount := lipgloss.NewStyle().Foreground(ColorDim).Render(
				fmt.Sprintf(" %d vote", req.Votes))
			if req.Votes > 1 {
				voteCount = lipgloss.NewStyle().Foreground(ColorDim).Render(
					fmt.Sprintf(" %d votes", req.Votes))
			}
			b.WriteString("     " + bar + voteCount + "\n")
		} else {
			emptyBar := lipgloss.NewStyle().Foreground(ColorDimmer).Render("░░░░░░░░")
			b.WriteString("     " + emptyBar + "\n")
		}
	}

	if len(state.Requests) > 5 {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render(
			fmt.Sprintf("\n  +%d more tracks\n", len(state.Requests)-5)))
	}

	// Footer hints
	b.WriteString("\n")
	jk := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("j/k")
	enter := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("ENTER")
	if votedFor != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render(
			fmt.Sprintf("  %s navigate  ·  %s change vote", jk, enter)))
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorDim).Render(
			fmt.Sprintf("  %s navigate  ·  %s vote", jk, enter)))
	}
	b.WriteString("\n")

	return b.String()
}
