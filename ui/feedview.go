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

	// Comment view
	viewport    viewport.Model
	comments    []reddit.Comment
	commentPost *reddit.Post

	// Loading state
	loading        bool
	loadingComment bool
	loadError      string
	lastUpdate     time.Time

	// Temporary notice (e.g. "Link copied!")
	notice   string
	noticeAt time.Time
}

func NewFeedView() FeedView {
	vp := viewport.New(viewport.WithWidth(40), viewport.WithHeight(20))
	return FeedView{
		state:    feedStateList,
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
	f.loadError = ""
}

func (f *FeedView) SetError(msg string) {
	f.loading = false
	f.loadError = msg
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

// CurrentPost returns the relevant post for the current view.
// In comment view, returns the post being viewed. In list, returns the selected post.
func (f *FeedView) CurrentPost() *reddit.Post {
	if f.state == feedStateComment && f.commentPost != nil {
		return f.commentPost
	}
	return f.SelectedPost()
}

func (f *FeedView) InCommentView() bool {
	return f.state == feedStateComment
}

func (f *FeedView) BackToList() {
	f.state = feedStateList
	f.comments = nil
	f.commentPost = nil
}

func (f *FeedView) ShowNotice(text string) {
	f.notice = text
	f.noticeAt = time.Now()
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
	if f.loadError != "" {
		return f.centeredText("Feed unavailable — " + f.loadError)
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
	// Show temporary notice (fades after 2s)
	if f.notice != "" && time.Since(f.noticeAt) < 2*time.Second {
		notice := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true).Render(f.notice)
		ago += "  " + notice
	}
	lines = append(lines, header+ago)
	lines = append(lines, lipgloss.NewStyle().Foreground(ColorDim).
		Render(strings.Repeat("─", f.width-4)))

	usedLines := 2
	for i := f.scroll; i < len(f.posts) && usedLines < contentH; i++ {
		post := f.posts[i]
		selected := i == f.cursor
		card := f.renderCard(post, selected)
		cardLines := strings.Count(card, "\n") + 1
		if usedLines+cardLines > contentH && i > f.scroll {
			break // don't break on first post — always show at least one
		}
		lines = append(lines, card)
		usedLines += cardLines
	}

	content := strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Width(f.width).
		Render(content)
}

func (f FeedView) renderCard(post reddit.Post, selected bool) string {
	cardW := f.width - 8 // border + padding + cursor
	if cardW < 20 {
		cardW = 20
	}

	dimStyle := lipgloss.NewStyle().Foreground(ColorDim)
	titleStyle := lipgloss.NewStyle().Bold(true)
	subStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	scoreStyle := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	linkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Underline(true)

	if selected {
		titleStyle = titleStyle.Foreground(lipgloss.Color("15"))
	}

	// Title (wrapped)
	titleLines := feedWrapText(post.Title, cardW)

	// Meta: subreddit, score, comments, time
	sub := subStyle.Render("r/" + post.Subreddit)
	score := scoreStyle.Render(fmt.Sprintf("%d ^", post.Score))
	ago := feedShortTimeAgo(post.CreatedUTC)
	meta := sub + "  " + score + "  " + dimStyle.Render(fmt.Sprintf("%d comments  %s", post.NumComments, ago))

	// Link
	url := post.URL
	if post.IsSelf {
		url = "https://reddit.com" + post.Permalink
	}
	link := linkStyle.Render(truncateURL(url, cardW))

	var parts []string

	// Image tag if post has image
	if post.HasImage && !post.IsSelf {
		imgTag := lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Background(lipgloss.Color("236")).
			Padding(0, 1).
			Render("image")
		parts = append(parts, imgTag)
	}

	// Title
	for _, line := range titleLines {
		parts = append(parts, titleStyle.Render(line))
	}

	// Meta + link
	parts = append(parts, meta)
	parts = append(parts, link)

	content := strings.Join(parts, "\n")

	border := lipgloss.RoundedBorder()
	borderColor := ColorBorder
	if selected {
		borderColor = lipgloss.Color("11")
	}

	card := lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(f.width - 4).
		Render(content)

	if selected {
		cardLines := strings.Split(card, "\n")
		cursor := lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
		for i, line := range cardLines {
			if i == 0 {
				cardLines[i] = cursor.Render("▸ ") + line
			} else {
				cardLines[i] = cursor.Render("│ ") + line
			}
		}
		return strings.Join(cardLines, "\n")
	}
	return "  " + strings.ReplaceAll(card, "\n", "\n  ")
}

func truncateURL(url string, maxW int) string {
	if len(url) <= maxW {
		return url
	}
	return url[:maxW-3] + "..."
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
	// Show link
	url := p.URL
	if p.IsSelf {
		url = "https://reddit.com" + p.Permalink
	}
	linkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	b.WriteString(linkStyle.Render(url))
	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", f.width-4)))
	b.WriteString("\n")

	if p.Selftext != "" {
		b.WriteString("\n")
		for _, line := range feedWrapText(p.Selftext, f.width-6) {
			b.WriteString("  " + line)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Comments header — clear separator
	b.WriteString("\n")
	commentsHeader := lipgloss.NewStyle().
		Foreground(ColorAccent).Bold(true).
		Render(fmt.Sprintf("COMMENTS (%d)", p.NumComments))
	b.WriteString(commentsHeader + "\n")
	b.WriteString(dimStyle.Render(strings.Repeat("─", f.width-4)))
	b.WriteString("\n\n")

	if len(f.comments) == 0 {
		b.WriteString(dimStyle.Render("  No comments yet"))
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

	headerText := "ESC back  |  s share"
	header := lipgloss.NewStyle().
		Foreground(ColorDim).
		Render(headerText)

	// Show temporary notice (fades after 2s)
	if f.notice != "" && time.Since(f.noticeAt) < 2*time.Second {
		notice := lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true).Render(f.notice)
		header += "  " + notice
	}

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
	if f.cursor < f.scroll {
		f.scroll = f.cursor
		return
	}
	// Measure actual rendered card heights to see if cursor is visible
	contentH := f.height - 4
	used := 0
	for i := f.scroll; i <= f.cursor && i < len(f.posts); i++ {
		card := f.renderCard(f.posts[i], i == f.cursor)
		cardH := strings.Count(card, "\n") + 1
		used += cardH
		if used > contentH && i <= f.cursor {
			// Cursor card doesn't fit — advance scroll
			f.scroll++
			f.ensureVisible() // recalculate
			return
		}
	}
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
