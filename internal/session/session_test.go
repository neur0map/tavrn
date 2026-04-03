package session

import "testing"

func TestNewSession(t *testing.T) {
	s := New("fp123", "alice", 5, true, "lounge")
	if s.Fingerprint != "fp123" {
		t.Errorf("fingerprint = %q, want fp123", s.Fingerprint)
	}
	if s.Nickname != "alice" {
		t.Errorf("nickname = %q, want alice", s.Nickname)
	}
	if s.ColorIndex != 5 {
		t.Errorf("colorIndex = %d, want 5", s.ColorIndex)
	}
	if !s.Flair {
		t.Error("expected flair = true")
	}
	if s.Room != "lounge" {
		t.Errorf("default room = %q, want lounge", s.Room)
	}
}

func TestNewSessionDefaults(t *testing.T) {
	s := New("fp", "bob", 0, false, "lounge")
	if s.Flair {
		t.Error("expected flair = false")
	}
	if s.Room != "lounge" {
		t.Errorf("default room = %q, want lounge", s.Room)
	}
}

func TestSessionSendChannel(t *testing.T) {
	s := New("fp", "nick", 0, false, "lounge")
	if cap(s.Send) != 256 {
		t.Errorf("send channel capacity = %d, want 256", cap(s.Send))
	}
}

func TestSessionChatLimiter(t *testing.T) {
	s := New("fp", "nick", 0, false, "lounge")
	if s.ChatLimiter == nil {
		t.Fatal("expected ChatLimiter to be initialized")
	}
	// Should allow initial messages
	if !s.ChatLimiter.Allow() {
		t.Error("expected first message to be allowed")
	}
}

func TestMsgTimestampZeroByDefault(t *testing.T) {
	msg := Msg{Type: MsgChat, Text: "hello"}
	if !msg.Timestamp.IsZero() {
		t.Error("expected zero timestamp by default")
	}
}

func TestMsgTypes(t *testing.T) {
	// Verify msg types are distinct
	types := []MsgType{
		MsgChat, MsgSystem, MsgCanvasDelta, MsgUserJoined,
		MsgUserLeft, MsgPurge, MsgTyping, MsgNoteCreate,
		MsgNoteMove, MsgNoteDelete, MsgBanner,
	}
	seen := make(map[MsgType]bool)
	for _, mt := range types {
		if seen[mt] {
			t.Errorf("duplicate MsgType value: %d", mt)
		}
		seen[mt] = true
	}
}
