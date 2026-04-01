package ui

import (
	"net/url"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
)

var urlRegex = regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`)

// extractURLs finds all URLs in text and returns them.
func extractURLs(text string) []string {
	return urlRegex.FindAllString(text, -1)
}

// renderURLBox renders a URL as a styled box for chat display.
func renderURLBox(rawURL string) string {
	border := lipgloss.NewStyle().Foreground(ColorBorder)
	link := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)
	dim := lipgloss.NewStyle().Foreground(ColorDim)

	// Extract domain for the header
	domain := rawURL
	if parsed, err := url.Parse(rawURL); err == nil && parsed.Host != "" {
		domain = parsed.Host
	}

	// Clean display URL — remove protocol for cleaner look
	display := rawURL
	display = strings.TrimPrefix(display, "https://")
	display = strings.TrimPrefix(display, "http://")
	display = strings.TrimSuffix(display, "/")

	var b strings.Builder
	b.WriteString(border.Render("╭─") + dim.Render(" "+domain+" ") + border.Render("─╮"))
	b.WriteString("\n")
	b.WriteString(border.Render("│ ") + link.Render(display) + border.Render(" │"))
	b.WriteString("\n")
	b.WriteString(border.Render("╰─") + border.Render(strings.Repeat("─", lipgloss.Width(display)+1)) + border.Render("╯"))

	return b.String()
}

// renderTextWithURLs renders message text, replacing URLs with styled boxes.
// Returns the text lines with URLs extracted and rendered as boxes below.
func renderTextWithURLs(text string, wrapWidth int) []string {
	urls := extractURLs(text)
	if len(urls) == 0 {
		return nil
	}

	var boxes []string
	for _, u := range urls {
		for _, line := range strings.Split(renderURLBox(u), "\n") {
			boxes = append(boxes, "      "+line)
		}
	}
	return boxes
}
