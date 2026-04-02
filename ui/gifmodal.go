package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"tavrn.sh/internal/gif"
)

const gifModalRenderWidth = 60

// GifSendMsg carries the selected GIF data back to the app.
type GifSendMsg struct {
	Frames []string
	Delays []int
	Title  string
}

// GifSearchResultMsg delivers search results from the background search.
type GifSearchResultMsg struct {
	Results []gif.KlipyResult
	Err     error
}

// GifFrameDataMsg delivers decoded+rendered frames for a single result.
type GifFrameDataMsg struct {
	Index  int
	Frames []string
	Delays []int
	Err    error
}

type gifTickMsg time.Time

type gifCacheEntry struct {
	frames []string
	delays []int
}

type GifModal struct {
	query   string
	results []gif.KlipyResult
	cursor  int
	loading bool
	err     string
	client  *gif.KlipyClient

	// Current preview animation
	frames    []string
	delays    []int
	frame     int
	lastTick  time.Time
	animating bool

	// Cache of already-fetched results
	cache map[int]gifCacheEntry
}

func NewGifModal(query string, client *gif.KlipyClient) GifModal {
	return GifModal{
		query:   query,
		loading: true,
		client:  client,
		cache:   make(map[int]gifCacheEntry),
	}
}

func (g GifModal) Init() tea.Cmd {
	return tea.Batch(g.doSearch(), gifAnimTick())
}

func gifAnimTick() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return gifTickMsg(t)
	})
}

func (g GifModal) Update(msg tea.Msg) (GifModal, tea.Cmd) {
	switch msg := msg.(type) {
	case GifSearchResultMsg:
		g.loading = false
		if msg.Err != nil {
			g.err = msg.Err.Error()
			return g, nil
		}
		g.results = msg.Results
		if len(g.results) == 0 {
			g.err = "no results"
			return g, nil
		}
		g.cursor = 0
		return g, g.fetchCurrentGif()

	case GifFrameDataMsg:
		if msg.Err != nil {
			g.err = msg.Err.Error()
			g.animating = false
			return g, nil
		}
		g.cache[msg.Index] = gifCacheEntry{frames: msg.Frames, delays: msg.Delays}
		if msg.Index == g.cursor {
			g.frames = msg.Frames
			g.delays = msg.Delays
			g.frame = 0
			g.lastTick = time.Now()
			g.animating = true
			g.err = ""
		}
		return g, nil

	case gifTickMsg:
		if g.animating && len(g.frames) > 1 {
			now := time.Now()
			delay := 100
			if g.frame < len(g.delays) {
				delay = g.delays[g.frame]
			}
			if now.Sub(g.lastTick) >= time.Duration(delay)*time.Millisecond {
				g.frame = (g.frame + 1) % len(g.frames)
				g.lastTick = now
			}
		}
		return g, gifAnimTick()

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return g, func() tea.Msg { return CloseModalMsg{} }
		case "enter":
			if len(g.frames) > 0 {
				title := g.query
				if g.cursor < len(g.results) {
					title = g.results[g.cursor].Title
				}
				return g, func() tea.Msg {
					return GifSendMsg{
						Frames: g.frames,
						Delays: g.delays,
						Title:  title,
					}
				}
			}
		case "left", "h":
			if len(g.results) > 0 {
				g.cursor = (g.cursor - 1 + len(g.results)) % len(g.results)
				return g, g.loadCurrent()
			}
		case "right", "l":
			if len(g.results) > 0 {
				g.cursor = (g.cursor + 1) % len(g.results)
				return g, g.loadCurrent()
			}
		}
	}
	return g, nil
}

func (g *GifModal) loadCurrent() tea.Cmd {
	if cached, ok := g.cache[g.cursor]; ok {
		g.frames = cached.frames
		g.delays = cached.delays
		g.frame = 0
		g.lastTick = time.Now()
		g.animating = true
		g.err = ""
		return nil
	}
	g.animating = false
	g.frames = nil
	g.loading = true
	return g.fetchCurrentGif()
}

func (g GifModal) doSearch() tea.Cmd {
	query := g.query
	client := g.client
	return func() tea.Msg {
		results, err := client.Search(query)
		return GifSearchResultMsg{Results: results, Err: err}
	}
}

func (g GifModal) fetchCurrentGif() tea.Cmd {
	if g.cursor >= len(g.results) {
		return nil
	}
	result := g.results[g.cursor]
	index := g.cursor
	client := g.client
	return func() tea.Msg {
		data, err := client.FetchGIF(result.URL)
		if err != nil {
			return GifFrameDataMsg{Index: index, Err: err}
		}
		decoded, err := gif.Decode(data)
		if err != nil {
			return GifFrameDataMsg{Index: index, Err: err}
		}
		frames := gif.RenderFrames(decoded.Frames, gifModalRenderWidth)
		return GifFrameDataMsg{Index: index, Frames: frames, Delays: decoded.Delays}
	}
}

func (g GifModal) View(width, height int) string {
	border := lipgloss.NewStyle().Foreground(ColorBorder)
	accent := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	dim := lipgloss.NewStyle().Foreground(ColorDim)
	highlight := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)

	var b strings.Builder

	// Title
	title := fmt.Sprintf(" Search KLIPY: %s ", g.query)
	b.WriteString(accent.Render(title))
	b.WriteString("\n\n")

	if g.loading && len(g.frames) == 0 {
		b.WriteString(dim.Render("  loading..."))
		b.WriteString("\n")
	} else if g.err != "" && len(g.frames) == 0 {
		b.WriteString(dim.Render("  " + g.err))
		b.WriteString("\n")
	} else if len(g.frames) > 0 {
		// Render current frame
		frame := g.frames[g.frame%len(g.frames)]
		for _, line := range strings.Split(frame, "\n") {
			b.WriteString("  " + line)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	// Navigation info
	if len(g.results) > 0 {
		title := g.query
		if g.cursor < len(g.results) {
			title = g.results[g.cursor].Title
		}
		// Truncate long titles
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		pos := fmt.Sprintf("(%d/%d)", g.cursor+1, len(g.results))
		b.WriteString("  " + highlight.Render(title) + " " + dim.Render(pos))
		b.WriteString("\n\n")
	}

	// Controls
	controls := border.Render("←→") + dim.Render(" browse  ") +
		border.Render("ENTER") + dim.Render(" send  ") +
		border.Render("ESC") + dim.Render(" close")
	b.WriteString("  " + controls)

	content := b.String()

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2).
		Render(content)
}
