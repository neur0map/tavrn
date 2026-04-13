package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"tavrn.sh/internal/reddit"
)

type feedState int

const (
	feedStateList feedState = iota
	feedStateComment
)

type FeedView struct {
	state  feedState
	posts  []reddit.Post
	cursor int
	scroll int
	width  int
	height int

	// Shared reddit client (server-wide thumbnail cache lives here)
	client *reddit.Client

	// Comment view
	viewport    viewport.Model
	comments    []reddit.Comment
	commentPost *reddit.Post

	// Loading state
	loading        bool
	loadingComment bool
	lastUpdate     time.Time
}

func NewFeedView(client *reddit.Client) FeedView {
	vp := viewport.New(viewport.WithWidth(40), viewport.WithHeight(20))
	return FeedView{
		state:    feedStateList,
		client:   client,
		viewport: vp,
	}
}

func (f *FeedView) SetSize(width, height int) {
	f.width = width
	f.height = height
	f.viewport.SetWidth(width - 2)
	f.viewport.SetHeight(height - 2)
}

func (f *FeedView) SetPosts(posts []reddit.Post) {
	f.posts = posts
	f.lastUpdate = time.Now()
	f.loading = false
}

func (f *FeedView) SetComments(comments []reddit.Comment, post *reddit.Post) {
	f.comments = comments
	f.commentPost = post
	f.loadingComment = false
	f.state = feedStateComment
	f.renderCommentView()
}

func (f *FeedView) SelectedPost() *reddit.Post {
	if f.cursor >= 0 && f.cursor < len(f.posts) {
		return &f.posts[f.cursor]
	}
	return nil
}

func (f *FeedView) InCommentView() bool {
	return f.state == feedStateComment
}

func (f *FeedView) BackToList() {
	f.state = feedStateList
	f.comments = nil
	f.commentPost = nil
}

// NextThumbToLoad returns the first visible post that needs a thumbnail loaded.
// Returns nil if nothing needs loading.
func (f *FeedView) NextThumbToLoad() *reddit.Post {
	if f.client == nil {
		return nil
	}
	for i := f.scroll; i < len(f.posts) && i < f.scroll+10; i++ {
		p := f.posts[i]
		if p.HasImage && p.PreviewURL != "" {
			if _, ok := f.client.GetThumb(p.ID); !ok {
				return &f.posts[i]
			}
		}
	}
	return nil
}

// View renders the feed panel.
func (f FeedView) View() string {
	if f.state == feedStateComment {
		return f.viewComments()
	}
	return f.viewList()
}

func (f FeedView) viewList() string {
	if f.loading {
		return f.centeredText("Loading feed...")
	}
	if len(f.posts) == 0 {
		return f.centeredText("No posts yet")
	}

	contentH := f.height - 4
	var lines []string

	header := lipgloss.NewStyle().
		Foreground(ColorAccent).Bold(true).
		Render("FEED")
	ago := ""
	if !f.lastUpdate.IsZero() {
		ago = " " + lipgloss.NewStyle().Foreground(ColorDim).
			Render(feedFormatTimeAgo(f.lastUpdate))
	}
	lines = append(lines, header+ago)
	lines = append(lines, lipgloss.NewStyle().Foreground(ColorDim).
		Render(strings.Repeat("─", f.width-4)))

	usedLines := 2
	for i := f.scroll; i < len(f.posts) && usedLines < contentH; i++ {
		post := f.posts[i]
		selected := i == f.cursor

		if post.HasImage {
			card := f.renderCard(post, selected)
			cardLines := strings.Count(card, "\n") + 1
			if usedLines+cardLines > contentH {
				break
			}
			lines = append(lines, card)
			usedLines += cardLines
		} else {
			row := f.renderCompact(post, selected)
			rowLines := strings.Count(row, "\n") + 1
			if usedLines+rowLines > contentH {
				break
			}
			lines = append(lines, row)
			usedLines += rowLines
		}
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Width(f.width).
		Height(f.height).
		Render(content)
}

func (f FeedView) renderCompact(post reddit.Post, selected bool) string {
	cardW := f.width - 4
	if cardW < 20 {
		cardW = 20
	}

	titleStyle := lipgloss.NewStyle()
	dimStyle := lipgloss.NewStyle().Foreground(ColorDim)

	if selected {
		titleStyle = titleStyle.Bold(true).Foreground(lipgloss.Color("15"))
	}

	title := post.Title
	titleW := cardW - 4
	if len(title) > titleW {
		title = title[:titleW-3] + "..."
	}

	sub := "r/" + post.Subreddit
	ago := feedShortTimeAgo(post.CreatedUTC)
	meta := fmt.Sprintf("%s  %d ^  %d comments  %s", sub, post.Score, post.NumComments, ago)

	content := titleStyle.Render(title) + "\n" + dimStyle.Render(meta)

	border := lipgloss.RoundedBorder()
	borderColor := ColorBorder
	if selected {
		borderColor = ColorAccent
	}

	return lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(f.width - 2).
		Render(content)
}

func (f FeedView) renderCard(post reddit.Post, selected bool) string {
	cardW := f.width - 4 // border + padding
	if cardW < 20 {
		cardW = 20
	}

	dimStyle := lipgloss.NewStyle().Foreground(ColorDim)
	titleStyle := lipgloss.NewStyle()
	if selected {
		titleStyle = titleStyle.Bold(true).Foreground(lipgloss.Color("15"))
	}

	// Title
	title := post.Title
	if len(title) > cardW*2 {
		title = title[:cardW*2-3] + "..."
	}
	titleLines := feedWrapText(title, cardW)

	// Meta line
	sub := "r/" + post.Subreddit
	ago := feedShortTimeAgo(post.CreatedUTC)
	meta := fmt.Sprintf("%s  %d ^  %d comments  %s", sub, post.Score, post.NumComments, ago)

	// Build card content: image on top (full width), then title, then meta
	var parts []string

	// Image — full width of the card
	if f.client != nil {
		thumb, ok := f.client.GetThumb(post.ID)
		if ok && thumb != "" {
			parts = append(parts, thumb)
		}
	}

	// Title
	for _, line := range titleLines {
		parts = append(parts, titleStyle.Render(line))
	}

	// Meta
	parts = append(parts, dimStyle.Render(meta))

	content := strings.Join(parts, "\n")

	border := lipgloss.RoundedBorder()
	borderColor := ColorBorder
	if selected {
		borderColor = ColorAccent
	}

	return lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(f.width - 2).
		Render(content)
}

// Comment view

func (f *FeedView) renderCommentView() {
	if f.commentPost == nil {
		return
	}

	var b strings.Builder
	p := f.commentPost

	titleStyle := lipgloss.NewStyle().Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(ColorDim)
	accentStyle := lipgloss.NewStyle().Foreground(ColorAccent)

	b.WriteString(titleStyle.Render(p.Title))
	b.WriteString("\n")
	meta := fmt.Sprintf("r/%s  %d ^  %d comments  %s",
		p.Subreddit, p.Score, p.NumComments, feedShortTimeAgo(p.CreatedUTC))
	b.WriteString(dimStyle.Render(meta))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", f.width-4)))
	b.WriteString("\n")

	if p.Selftext != "" {
		for _, line := range feedWrapText(p.Selftext, f.width-6) {
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(f.comments) == 0 {
		b.WriteString(dimStyle.Render("No comments"))
	}

	for _, c := range f.comments {
		feedRenderComment(&b, c, f.width-4, accentStyle, dimStyle)
	}

	f.viewport.SetContent(b.String())
	f.viewport.GotoTop()
}

func feedRenderComment(b *strings.Builder, c reddit.Comment, width int, accent, dim lipgloss.Style) {
	indent := strings.Repeat("  ", c.Depth)
	prefix := ""
	if c.Depth > 0 {
		prefix = indent + dim.Render("|") + " "
	}

	header := accent.Render(c.Author) + "  " +
		dim.Render(fmt.Sprintf("%d ^  %s", c.Score, feedShortTimeAgo(c.Created)))
	b.WriteString(prefix + header + "\n")

	bodyW := width - len(indent) - 4
	if bodyW < 20 {
		bodyW = 20
	}
	for _, line := range feedWrapText(c.Body, bodyW) {
		b.WriteString(prefix + "  " + line + "\n")
	}
	b.WriteString("\n")

	for _, child := range c.Children {
		feedRenderComment(b, child, width, accent, dim)
	}
}

func (f FeedView) viewComments() string {
	if f.loadingComment {
		return f.centeredText("Loading comments...")
	}

	header := lipgloss.NewStyle().
		Foreground(ColorDim).
		Render("ESC back  |  s share")

	return header + "\n" + f.viewport.View()
}

// Update handles input.
func (f FeedView) Update(msg tea.Msg) (FeedView, tea.Cmd) {
	if f.state == feedStateComment {
		return f.updateComments(msg)
	}
	return f.updateList(msg)
}

func (f FeedView) updateList(msg tea.Msg) (FeedView, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if f.cursor > 0 {
				f.cursor--
				f.ensureVisible()
			}
		case "down", "j":
			if f.cursor < len(f.posts)-1 {
				f.cursor++
				f.ensureVisible()
			}
		}
	case tea.MouseWheelMsg:
		if msg.Y < 0 {
			if f.cursor > 0 {
				f.cursor--
				f.ensureVisible()
			}
		} else if msg.Y > 0 {
			if f.cursor < len(f.posts)-1 {
				f.cursor++
				f.ensureVisible()
			}
		}
	}
	return f, nil
}

func (f FeedView) updateComments(msg tea.Msg) (FeedView, tea.Cmd) {
	var cmd tea.Cmd
	f.viewport, cmd = f.viewport.Update(msg)
	return f, cmd
}

func (f *FeedView) ensureVisible() {
	visible := f.visibleCount()
	if f.cursor < f.scroll {
		f.scroll = f.cursor
	} else if f.cursor >= f.scroll+visible {
		f.scroll = f.cursor - visible + 1
	}
}

func (f FeedView) visibleCount() int {
	h := f.height - 4
	count := 0
	used := 0
	for i := f.scroll; i < len(f.posts) && used < h; i++ {
		if f.posts[i].HasImage {
			used += 8 // image card takes more space
		} else {
			used += 4 // compact card with border
		}
		count++
	}
	if count == 0 {
		count = 1
	}
	return count
}

// Helpers

func (f FeedView) centeredText(text string) string {
	return lipgloss.NewStyle().
		Width(f.width).
		Height(f.height).
		Foreground(ColorDim).
		Render(text)
}

func feedShortTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func feedFormatTimeAgo(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh ago", int(d.Hours()))
}

func feedWrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	words := strings.Fields(text)
	var lines []string
	var current string
	for _, word := range words {
		if current == "" {
			current = word
		} else if len(current)+1+len(word) <= width {
			current += " " + word
		} else {
			lines = append(lines, current)
			current = word
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}
