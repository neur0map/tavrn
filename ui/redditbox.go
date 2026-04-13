package ui

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"tavrn.sh/internal/chat"
)

// RenderRedditBox renders a shared Reddit post as a styled card in chat.
func RenderRedditBox(msg *chat.Message) string {
	dim := lipgloss.NewStyle().Foreground(ColorDim)
	sub := lipgloss.NewStyle().Foreground(lipgloss.Color("208"))

	title := msg.RedditTitle
	if len(title) > 60 {
		title = title[:57] + "..."
	}

	meta := fmt.Sprintf("%d ^  %d comments", msg.RedditScore, msg.RedditComments)

	content := sub.Render("r/"+msg.RedditSub) + "\n" +
		title + "\n" +
		dim.Render(meta) + "\n" +
		dim.Render(msg.RedditURL)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("208")).
		Padding(0, 1).
		Render(content)
}
