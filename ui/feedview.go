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
	state    feedState
	posts    []reddit.Post
	cursor   int
	scroll   int
	width    int
	height   int
	viewport viewport.Model

	// Comment view
	comments    []reddit.Comment
	commentPost *reddit.Post

	// Rendered thumbnail cache: post ID -> rendered string
	thumbCache map[string]string

	// Loading state
	loading        bool
	loadingComment bool
	lastUpdate     time.Time
}

func NewFeedView() FeedView {
	vp := viewport.New(viewport.WithWidth(40), viewport.WithHeight(20))
	return FeedView{
		state:      feedStateList,
		thumbCache: make(map[string]string),
		viewport:   vp,
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

func (f *FeedView) SetThumbnail(postID, rendered string) {
	f.thumbCache[postID] = rendered
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
			if usedLines+1 > contentH {
				break
			}
			lines = append(lines, row)
			usedLines++
		}
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Width(f.width).
		Height(f.height).
		Render(content)
}

func (f FeedView) renderCompact(post reddit.Post, selected bool) string {
	w := f.width - 6

	score := fmt.Sprintf("%4d", post.Score)
	sub := "r/" + post.Subreddit
	ago := feedShortTimeAgo(post.CreatedUTC)

	right := sub + "  " + ago
	titleW := w - len(score) - len(right) - 6
	if titleW < 10 {
		titleW = 10
	}
	title := post.Title
	if len(title) > titleW {
		title = title[:titleW-3] + "..."
	}

	scoreStyle := lipgloss.NewStyle().Foreground(ColorAccent)
	dimStyle := lipgloss.NewStyle().Foreground(ColorDim)
	titleStyle := lipgloss.NewStyle()

	if selected {
		titleStyle = titleStyle.Bold(true).Foreground(lipgloss.Color("15"))
		scoreStyle = scoreStyle.Bold(true)
	}

	padding := titleW - len(title)
	if padding < 1 {
		padding = 1
	}

	row := scoreStyle.Render(score+" ^") + "  " +
		titleStyle.Render(title) +
		strings.Repeat(" ", padding) +
		dimStyle.Render(right)

	if selected {
		return lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Width(w).
			Render(row)
	}
	return row
}

func (f FeedView) renderCard(post reddit.Post, selected bool) string {
	thumbW := 12
	thumb, hasThumb := f.thumbCache[post.ID]

	titleW := f.width - thumbW - 8
	if titleW < 10 {
		titleW = 10
	}
	title := post.Title
	if len(title) > titleW*2 {
		title = title[:titleW*2-3] + "..."
	}

	titleLines := feedWrapText(title, titleW)

	sub := "r/" + post.Subreddit
	ago := feedShortTimeAgo(post.CreatedUTC)
	meta := fmt.Sprintf("%s  %d ^  %d comments  %s", sub, post.Score, post.NumComments, ago)

	dimStyle := lipgloss.NewStyle().Foreground(ColorDim)
	accentStyle := lipgloss.NewStyle().Foreground(ColorAccent)

	var textLines []string
	for _, line := range titleLines {
		textLines = append(textLines, line)
	}
	textLines = append(textLines, dimStyle.Render(meta))

	var content string
	if hasThumb && thumb != "" {
		thumbLines := strings.Split(thumb, "\n")
		maxLines := len(thumbLines)
		if len(textLines) > maxLines {
			maxLines = len(textLines)
		}
		var rows []string
		for i := 0; i < maxLines; i++ {
			tl := ""
			if i < len(thumbLines) {
				tl = thumbLines[i]
			} else {
				tl = strings.Repeat(" ", thumbW)
			}
			txt := ""
			if i < len(textLines) {
				txt = textLines[i]
			}
			rows = append(rows, tl+"  "+txt)
		}
		content = strings.Join(rows, "\n")
	} else {
		placeholder := accentStyle.Render("[img]")
		content = placeholder + "  " + strings.Join(textLines, "\n"+"       ")
	}

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
			used += 5
		} else {
			used++
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
