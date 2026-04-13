package chat

import (
	"strings"
	"time"
)

// Message represents a chat message or system notification.
type Message struct {
	Fingerprint string
	Nickname    string
	ColorIndex  int
	Room        string
	Text        string
	Timestamp   time.Time
	IsSystem    bool
	IsBanner    bool
	IsPoll      bool
	IsGif       bool
	GifFrames   []string // pre-rendered half-block frames
	GifDelays   []int    // frame delays in milliseconds
	GifTitle    string
	GifURL      string
	GifFrame    int // current animation frame index
	GifLastTick time.Time

	// Reddit embed
	IsReddit       bool
	RedditTitle    string
	RedditSub      string
	RedditScore    int
	RedditComments int
	RedditURL      string
	RedditThumb    string // pre-rendered thumbnail
}

// ParseResult holds the result of parsing user input.
type ParseResult struct {
	IsCommand bool
	Command   string
	Args      string
	Text      string
}

// ParseInput determines if input is a command (starts with /) or a chat message.
func ParseInput(input string) ParseResult {
	if len(input) == 0 {
		return ParseResult{Text: input}
	}
	if input[0] != '/' {
		return ParseResult{Text: input}
	}

	rest := input[1:]
	parts := strings.SplitN(rest, " ", 2)
	cmd := strings.ToLower(parts[0])
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}

	return ParseResult{
		IsCommand: true,
		Command:   cmd,
		Args:      args,
	}
}

func NewUserMessage(fingerprint, nickname, room, text string, colorIndex int) Message {
	return Message{
		Fingerprint: fingerprint,
		Nickname:    nickname,
		ColorIndex:  colorIndex,
		Room:        room,
		Text:        text,
		Timestamp:   time.Now(),
	}
}

func NewSystemMessage(room, text string) Message {
	return Message{
		Room:      room,
		Text:      text,
		Timestamp: time.Now(),
		IsSystem:  true,
	}
}

func NewBannerMessage(room, text string) Message {
	return Message{
		Room:      room,
		Text:      text,
		Timestamp: time.Now(),
		IsSystem:  true,
		IsBanner:  true,
	}
}
