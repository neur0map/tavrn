package hub

import (
	"tavrn.sh/internal/session"
	"testing"
	"time"
)

func TestRegisterAndBroadcast(t *testing.T) {
	h := New()
	go h.Run()
	defer h.Stop()

	s1 := session.New("fp1", "alice", 0, false, "lounge")
	s2 := session.New("fp2", "bob", 1, false, "lounge")

	h.Register(s1)
	h.Register(s2)
	time.Sleep(10 * time.Millisecond)

	if h.OnlineCount() != 2 {
		t.Errorf("online = %d, want 2", h.OnlineCount())
	}

	h.Broadcast("lounge", session.Msg{Type: session.MsgChat, Text: "hello"})

	select {
	case msg := <-s1.Send:
		if msg.Text != "hello" {
			t.Errorf("s1 got %q", msg.Text)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("s1 timeout")
	}

	select {
	case msg := <-s2.Send:
		if msg.Text != "hello" {
			t.Errorf("s2 got %q", msg.Text)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("s2 timeout")
	}
}

func TestUnregister(t *testing.T) {
	h := New()
	go h.Run()
	defer h.Stop()

	s1 := session.New("fp1", "alice", 0, false, "lounge")
	h.Register(s1)
	time.Sleep(10 * time.Millisecond)

	h.Unregister(s1)
	time.Sleep(10 * time.Millisecond)

	if h.OnlineCount() != 0 {
		t.Errorf("online = %d, want 0", h.OnlineCount())
	}
}

func TestBroadcastOnlyToRoom(t *testing.T) {
	h := New()
	go h.Run()
	defer h.Stop()

	s1 := session.New("fp1", "alice", 0, false, "lounge")
	s1.Room = "lounge"
	s2 := session.New("fp2", "bob", 1, false, "lounge")
	s2.Room = "gallery"

	h.Register(s1)
	h.Register(s2)
	time.Sleep(10 * time.Millisecond)

	h.Broadcast("lounge", session.Msg{Type: session.MsgChat, Text: "hello"})

	select {
	case <-s1.Send:
	case <-time.After(100 * time.Millisecond):
		t.Error("s1 should receive")
	}

	select {
	case <-s2.Send:
		t.Error("s2 in different room should not receive")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestDropSlowClient(t *testing.T) {
	h := New()
	go h.Run()
	defer h.Stop()

	s1 := session.New("fp1", "alice", 0, false, "lounge")
	h.Register(s1)
	time.Sleep(10 * time.Millisecond)

	for i := 0; i < 256; i++ {
		h.Broadcast("lounge", session.Msg{Type: session.MsgChat, Text: "spam"})
	}
	time.Sleep(10 * time.Millisecond)

	// Should not block
	h.Broadcast("lounge", session.Msg{Type: session.MsgChat, Text: "overflow"})
	time.Sleep(10 * time.Millisecond)
}
