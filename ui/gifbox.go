package ui

import (
	"time"

	"charm.land/lipgloss/v2"
	"tavrn.sh/internal/chat"
)

const maxAnimatingGifs = 3

// RenderGifBox renders a GIF message as a bordered box with the current frame.
func RenderGifBox(msg *chat.Message) string {
	if len(msg.GifFrames) == 0 {
		return ""
	}

	frame := msg.GifFrames[msg.GifFrame%len(msg.GifFrames)]

	dim := lipgloss.NewStyle().Foreground(ColorDim)
	accent := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)

	label := msg.GifTitle
	if len(label) > 30 {
		label = label[:27] + "..."
	}
	header := accent.Render("GIF") + " " + dim.Render(label)
	content := header + "\n\n" + frame + "\n"

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1).
		Render(content)
}

// TickGifAnimations advances frames for the most recent N animated GIFs.
// Returns true if any frame changed (needs redraw).
func TickGifAnimations(messages []chat.Message) bool {
	now := time.Now()
	changed := false

	// Find the most recent animated GIFs (up to maxAnimatingGifs)
	animating := 0
	for i := len(messages) - 1; i >= 0 && animating < maxAnimatingGifs; i-- {
		msg := &messages[i]
		if !msg.IsGif || len(msg.GifFrames) <= 1 {
			continue
		}
		animating++

		delay := 100 // default
		if msg.GifFrame < len(msg.GifDelays) {
			delay = msg.GifDelays[msg.GifFrame]
		}
		if delay < 20 {
			delay = 20
		}

		if now.Sub(msg.GifLastTick) >= time.Duration(delay)*time.Millisecond {
			msg.GifFrame = (msg.GifFrame + 1) % len(msg.GifFrames)
			msg.GifLastTick = now
			changed = true
		}
	}

	return changed
}
