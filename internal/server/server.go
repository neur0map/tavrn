package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/wish/v2"
	bm "charm.land/wish/v2/bubbletea"
	lm "charm.land/wish/v2/elapsed"
	"github.com/charmbracelet/ssh"

	"tavrn/internal/admin"
	"tavrn/internal/hub"
	"tavrn/internal/identity"
	"tavrn/internal/session"
	"tavrn/internal/store"
	"tavrn/ui"
)

type Config struct {
	Host             string
	Port             int
	HostKeyPath      string
	AdminFingerprint string
	Store            *store.Store
	Hub              *hub.Hub
	Admin            *admin.Admin
}

type Server struct {
	cfg  Config
	wish *ssh.Server
}

func New(cfg Config) (*Server, error) {
	s := &Server{cfg: cfg}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	ws, err := wish.NewServer(
		wish.WithAddress(addr),
		wish.WithHostKeyPath(cfg.HostKeyPath),
		wish.WithPublicKeyAuth(func(_ ssh.Context, _ ssh.PublicKey) bool {
			return true
		}),
		wish.WithMiddleware(
			bm.Middleware(s.teaHandler),
			lm.Middleware(),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("wish server: %w", err)
	}

	s.wish = ws
	return s, nil
}

func (s *Server) teaHandler(sshSess ssh.Session) (tea.Model, []tea.ProgramOption) {
	pubKey := sshSess.PublicKey()
	if pubKey == nil {
		wish.Fatalln(sshSess, "SSH key required to enter the tavern.")
		return nil, nil
	}

	hash := sha256.Sum256(pubKey.Marshal())
	fingerprint := hex.EncodeToString(hash[:])

	log.Printf("connect: %s (admin=%v)", fingerprint[:16], fingerprint == s.cfg.AdminFingerprint)

	banned, err := s.cfg.Store.IsBanned(fingerprint)
	if err != nil {
		log.Printf("ban check error: %v", err)
	}
	if banned {
		wish.Fatalln(sshSess, "You have been banned from the tavern.")
		return nil, nil
	}

	nickname := identity.DefaultNickname(fingerprint)
	existing, _ := s.cfg.Store.GetUser(fingerprint)
	if existing != nil {
		nickname = existing.Nickname
	}

	s.cfg.Store.UpsertUser(fingerprint, nickname)
	s.cfg.Store.RecordVisitor(fingerprint)

	user, _ := s.cfg.Store.GetUser(fingerprint)
	visitCount := 1
	if user != nil {
		visitCount = user.VisitCount
	}

	isAdmin := s.cfg.Admin.IsAdmin(fingerprint)
	colorIndex := identity.ColorIndex(fingerprint)
	flair := identity.HasFlair(visitCount)

	sess := session.New(fingerprint, nickname, colorIndex, flair, isAdmin)
	s.cfg.Hub.Register(sess)

	go func() {
		<-sshSess.Context().Done()
		s.cfg.Hub.Unregister(sess)
		s.cfg.Hub.Broadcast("lounge", session.Msg{
			Type: session.MsgUserLeft,
			Text: fmt.Sprintf("%s left the tavern", sess.Nickname),
			Room: "lounge",
		})
	}()

	s.cfg.Hub.Broadcast("lounge", session.Msg{
		Type: session.MsgUserJoined,
		Text: fmt.Sprintf("%s joined the tavern", nickname),
		Room: "lounge",
	})

	// Send recent chat history to this session
	history, _ := s.cfg.Store.RecentMessages("lounge", 50)
	for _, m := range history {
		msgType := session.MsgChat
		if m.IsSystem {
			msgType = session.MsgSystem
		}
		sess.Send <- session.Msg{
			Type:        msgType,
			Nickname:    m.Nickname,
			Fingerprint: m.Fingerprint,
			ColorIndex:  m.ColorIndex,
			Text:        m.Text,
			Room:        m.Room,
		}
	}

	onSend := func(msg session.Msg) {
		// Persist to SQLite
		switch msg.Type {
		case session.MsgChat:
			s.cfg.Store.SaveMessage(msg.Room, msg.Fingerprint, msg.Nickname, msg.ColorIndex, msg.Text, false)
		case session.MsgSystem, session.MsgUserJoined, session.MsgUserLeft:
			s.cfg.Store.SaveMessage(msg.Room, "", "", 0, msg.Text, true)
		}
		s.cfg.Hub.Broadcast(msg.Room, msg)
	}

	model := ui.NewApp(sess, s.cfg.Store, s.cfg.Hub, s.cfg.Admin, onSend)
	return model, nil // alt screen is declarative in v2 View
}

func (s *Server) Start() error {
	log.Printf("tavrn.sh listening on %s:%d", s.cfg.Host, s.cfg.Port)
	return s.wish.ListenAndServe()
}

func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.wish.Shutdown(ctx)
}

func (s *Server) StartHTTPAdmin(addr, token string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/admin/purge", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		s.cfg.Hub.BroadcastAll(session.Msg{
			Type: session.MsgPurge,
			Text: "tavern closing",
		})
		s.cfg.Store.PurgeAll()
		s.cfg.Hub.DisconnectAll()
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "purged")
	})

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Printf("admin HTTP error: %v", err)
		}
	}()
	log.Printf("admin HTTP on %s", addr)
}
