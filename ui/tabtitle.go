package ui

import "fmt"

// Animated tab title frames
var tabFrames = []string{
	"╱╱╱ tavrn.sh ╱╱╱",
	"╱╱ tavrn.sh ╱╱╱╱",
	"╱ tavrn.sh ╱╱╱╱╱",
	" tavrn.sh ╱╱╱╱╱╱",
	"tavrn.sh ╱╱╱╱╱╱╱",
	" tavrn.sh ╱╱╱╱╱╱",
	"╱ tavrn.sh ╱╱╱╱╱",
	"╱╱ tavrn.sh ╱╱╱╱",
}

var splashTabFrames = []string{
	"✦ tavrn.sh ✦",
	"✧ tavrn.sh ✧",
	"· tavrn.sh ·",
	"✧ tavrn.sh ✧",
}

func TabTitle(frame int, online int) string {
	base := tabFrames[frame%len(tabFrames)]
	if online > 0 {
		return fmt.Sprintf("%s  [%d]", base, online)
	}
	return base
}

func SplashTabTitle(frame int) string {
	return splashTabFrames[frame%len(splashTabFrames)]
}
