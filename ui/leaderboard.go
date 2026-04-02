package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"tavrn.sh/internal/wargame"
)

type LeaderboardModal struct {
	entries  []wargame.LeaderboardEntry
	progress []wargame.Progress
	myRank   int
	myPoints int
	myLevel  int
}

func NewLeaderboardModal(entries []wargame.LeaderboardEntry, progress []wargame.Progress, myFingerprint string) LeaderboardModal {
	rank := 0
	points := 0
	level := 0
	for i, e := range entries {
		if e.Fingerprint == myFingerprint {
			rank = i + 1
			points = e.TotalPoints
			level = e.TotalLevel
			break
		}
	}
	// If not on leaderboard, calculate from progress
	if rank == 0 {
		for _, p := range progress {
			level += p.Level
			points += p.Points
		}
	}

	return LeaderboardModal{
		entries:  entries,
		progress: progress,
		myRank:   rank,
		myPoints: points,
		myLevel:  level,
	}
}

func (l LeaderboardModal) Update(msg tea.Msg) (LeaderboardModal, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		if keyMsg.String() == "esc" || keyMsg.String() == "f7" {
			return l, func() tea.Msg { return CloseModalMsg{} }
		}
	}
	return l, nil
}

func (l LeaderboardModal) View(width, height int) string {
	highlight := lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)
	accent := lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	gold := lipgloss.NewStyle().Foreground(ColorAmber).Bold(true)
	dim := lipgloss.NewStyle().Foreground(ColorDim)
	dimmer := lipgloss.NewStyle().Foreground(ColorDimmer)
	green := lipgloss.NewStyle().Foreground(ColorGreen)

	var b strings.Builder

	// Title
	b.WriteString(highlight.Render("LEADERBOARD"))
	b.WriteString("\n")
	b.WriteString(dimmer.Render(strings.Repeat("─", 38)))
	b.WriteString("\n\n")

	// Rankings
	if len(l.entries) == 0 {
		b.WriteString(dim.Render("  No hackers on the board yet."))
		b.WriteString("\n")
	} else {
		medals := []string{"*", ".", "."}
		for i, e := range l.entries {
			if i >= 10 {
				break
			}
			rankStr := fmt.Sprintf("#%d", i+1)

			// Medal for top 3
			medal := " "
			if i < len(medals) {
				medal = medals[i]
			}

			name := e.Nickname
			if len(name) > 16 {
				name = name[:13] + "..."
			}

			levelStr := fmt.Sprintf("Lv.%d", e.TotalLevel)
			ptsStr := fmt.Sprintf("%d", e.TotalPoints)

			// Pad columns
			nameCol := fmt.Sprintf("%-16s", name)
			levelCol := fmt.Sprintf("%-7s", levelStr)

			if i == 0 {
				b.WriteString(gold.Render(fmt.Sprintf("  %s %s ", medal, rankStr)))
				b.WriteString(accent.Render(nameCol))
				b.WriteString(dim.Render(levelCol))
				b.WriteString(highlight.Render(fmt.Sprintf("%5s pts", ptsStr)))
			} else if i < 3 {
				b.WriteString(dim.Render(fmt.Sprintf("  %s %s ", medal, rankStr)))
				b.WriteString(accent.Render(nameCol))
				b.WriteString(dimmer.Render(levelCol))
				b.WriteString(dim.Render(fmt.Sprintf("%5s pts", ptsStr)))
			} else {
				b.WriteString(dimmer.Render(fmt.Sprintf("    %s ", rankStr)))
				b.WriteString(dim.Render(nameCol))
				b.WriteString(dimmer.Render(levelCol))
				b.WriteString(dimmer.Render(fmt.Sprintf("%5s pts", ptsStr)))
			}
			b.WriteString("\n")
		}
	}

	// Your progress section
	if len(l.progress) > 0 {
		b.WriteString("\n")
		b.WriteString(dimmer.Render(strings.Repeat("─", 38)))
		b.WriteString("\n")
		b.WriteString(accent.Render("YOUR PROGRESS"))
		b.WriteString("\n\n")

		for _, p := range l.progress {
			name := strings.ToUpper(p.Wargame)
			nameW := 12
			if len(name) > nameW {
				name = name[:nameW]
			}
			// Pad name manually then style (ANSI codes break fmt padding)
			padded := name + strings.Repeat(" ", nameW-len(name))

			if p.MaxLevel == 0 {
				// No flags set yet — just show the name
				b.WriteString("  " + dim.Render(padded) + " " + dimmer.Render("no flags yet"))
			} else {
				levelStr := fmt.Sprintf("%d/%d", p.Level, p.MaxLevel)
				levelPad := fmt.Sprintf("%-6s", levelStr)

				barW := 12
				filled := p.Level * barW / p.MaxLevel
				if filled > barW {
					filled = barW
				}
				bar := strings.Repeat("█", filled) + strings.Repeat("░", barW-filled)

				b.WriteString("  " + dim.Render(padded) + " " + dimmer.Render(levelPad) + " ")
				if p.Level > 0 {
					b.WriteString(green.Render(bar))
				} else {
					b.WriteString(dimmer.Render(bar))
				}
			}
			b.WriteString("\n")
		}

		b.WriteString("\n")
		rankStr := "unranked"
		if l.myRank > 0 {
			rankStr = fmt.Sprintf("#%d", l.myRank)
		}
		b.WriteString(dim.Render(fmt.Sprintf("  Total: Lv.%d  %d pts  Rank: %s",
			l.myLevel, l.myPoints, rankStr)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimmer.Render("ESC") + dim.Render(" close"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2).
		Render(b.String())
}
