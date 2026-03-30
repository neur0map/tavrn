package hub

import (
	"sync"
	"tavrn.sh/internal/session"
)

type Hub struct {
	sessions   map[string]*session.Session
	mu         sync.RWMutex
	register   chan *session.Session
	unregister chan *session.Session
	broadcast  chan broadcastMsg
	quit       chan struct{}
}

type broadcastMsg struct {
	room string
	msg  session.Msg
}

func New() *Hub {
	return &Hub{
		sessions:   make(map[string]*session.Session),
		register:   make(chan *session.Session),
		unregister: make(chan *session.Session),
		broadcast:  make(chan broadcastMsg, 256),
		quit:       make(chan struct{}),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case s := <-h.register:
			h.mu.Lock()
			h.sessions[s.Fingerprint] = s
			h.mu.Unlock()
		case s := <-h.unregister:
			h.mu.Lock()
			delete(h.sessions, s.Fingerprint)
			h.mu.Unlock()
		case bm := <-h.broadcast:
			h.mu.RLock()
			for _, s := range h.sessions {
				if s.Room == bm.room {
					select {
					case s.Send <- bm.msg:
					default:
						// Drop message for slow client
					}
				}
			}
			h.mu.RUnlock()
		case <-h.quit:
			return
		}
	}
}

func (h *Hub) Stop() {
	close(h.quit)
}

func (h *Hub) Register(s *session.Session) {
	h.register <- s
}

func (h *Hub) Unregister(s *session.Session) {
	h.unregister <- s
}

func (h *Hub) Broadcast(room string, msg session.Msg) {
	h.broadcast <- broadcastMsg{room: room, msg: msg}
}

// BroadcastAll sends to all connected sessions regardless of room.
func (h *Hub) BroadcastAll(msg session.Msg) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, s := range h.sessions {
		select {
		case s.Send <- msg:
		default:
		}
	}
}

func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.sessions)
}

// Sessions returns a snapshot of current sessions in a room.
func (h *Hub) Sessions(room string) []*session.Session {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var result []*session.Session
	for _, s := range h.sessions {
		if s.Room == room {
			result = append(result, s)
		}
	}
	return result
}

// Kick disconnects a session by fingerprint. Returns true if found.
func (h *Hub) Kick(fingerprint string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	s, ok := h.sessions[fingerprint]
	if !ok {
		return false
	}
	close(s.Send)
	delete(h.sessions, fingerprint)
	return true
}

// DisconnectAll closes all session send channels.
func (h *Hub) DisconnectAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for fp, s := range h.sessions {
		close(s.Send)
		delete(h.sessions, fp)
	}
}
