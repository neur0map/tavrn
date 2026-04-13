package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"tavrn.sh/internal/chat"
)

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

	var content string
	if msg.RedditThumb != "" {
		thumbLines := strings.Split(msg.RedditThumb, "\n")
		textLines := []string{title, dim.Render(meta), dim.Render(msg.RedditURL)}
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
		content = header + "\n" + title + "\n" + dim.Render(meta) + "\n" + dim.Render(msg.RedditURL)
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("208")).
		Padding(0, 1).
		Render(content)
}
