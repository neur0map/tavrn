package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"tavrn.sh/internal/chat"
)

// osc8Link wraps text in an OSC 8 hyperlink escape sequence.
// Terminals that support it (iTerm2, Ghostty, Kitty, Wezterm) make it clickable.
// Others just show the text.
func osc8Link(url, text string) string {
	return fmt.Sprintf("\033]8;;%s\033\\%s\033]8;;\033\\", url, text)
}

// RenderRedditBox renders a shared Reddit post as a styled card in chat.
func RenderRedditBox(msg *chat.Message) string {
	dim := lipgloss.NewStyle().Foreground(ColorDim)
	sub := lipgloss.NewStyle().Foreground(lipgloss.Color("208"))

	header := sub.Render("r/" + msg.RedditSub)

	title := msg.RedditTitle
	if len(title) > 50 {
		title = title[:47] + "..."
	}

	meta := fmt.Sprintf("%d ^  %d comments", msg.RedditScore, msg.RedditComments)

	link := osc8Link(msg.RedditURL, msg.RedditURL)

	var content string
	if msg.RedditThumb != "" {
		thumbLines := strings.Split(msg.RedditThumb, "\n")
		textLines := []string{title, dim.Render(meta), dim.Render(link)}
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
				tl = strings.Repeat(" ", 10)
			}
			txt := ""
			if i < len(textLines) {
				txt = textLines[i]
			}
			rows = append(rows, tl+"  "+txt)
		}
		content = header + "\n" + strings.Join(rows, "\n")
	} else {
		content = header + "\n" + title + "\n" + dim.Render(meta) + "\n" + dim.Render(link)
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("208")).
		Padding(0, 1).
		Render(content)
}
