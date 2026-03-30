package session

import (
	"time"

	"tavrn.sh/internal/ratelimit"
)

type MsgType int

const (
	MsgChat MsgType = iota
	MsgSystem
	MsgCanvasDelta
	MsgUserJoined
	MsgUserLeft
	MsgPurge
	MsgTyping
	MsgNoteCreate
	MsgNoteMove
	MsgNoteDelete
	MsgBanner
	MsgRoomAdded
	MsgRoomRenamed  // room renamed: Text=old, Room=new
	MsgRoomRemoved  // room removed: Text=removed room name
	MsgSudokuPlace  // player places a number
	MsgSudokuClear  // player clears a cell
	MsgSudokuCheck  // check board request/response
	MsgSudokuCursor // cursor position broadcast
	MsgSudokuNew    // new puzzle vote/generation
	MsgSudokuState  // full board sync for late joiners
)

// NoteData carries sticky note info through the hub.
type NoteData struct {
	ID    int
	X, Y  int
	Text  string
	Nick  string
	Color int
}

// Msg is a message sent from the hub to a session.
type Msg struct {
	Type        MsgType
	Nickname    string
	Fingerprint string
	ColorIndex  int
	Text        string
	Room        string
	Timestamp   time.Time
	Note        *NoteData
	SudokuRow   int    // row 0-8
	SudokuCol   int    // col 0-8
	SudokuValue int    // digit 1-9 (0 for clear)
	SudokuBoard string // JSON-encoded board state for full sync
}

// Session represents a connected user.
type Session struct {
	Fingerprint string
	Nickname    string
	Flair       bool
	Room        string
	ColorIndex  int
	Send        chan Msg
	ChatLimiter *ratelimit.Limiter
}

// New creates a session with a buffered send channel (256 messages).
func New(fingerprint, nickname string, colorIndex int, flair bool) *Session {
	return &Session{
		Fingerprint: fingerprint,
		Nickname:    nickname,
		Flair:       flair,
		Room:        "lounge",
		ColorIndex:  colorIndex,
		Send:        make(chan Msg, 256),
		ChatLimiter: ratelimit.NewChat(),
	}
}
