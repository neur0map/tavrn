package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/wish/v2"
	bm "charm.land/wish/v2/bubbletea"
	lm "charm.land/wish/v2/elapsed"
	"github.com/charmbracelet/ssh"
	"tavrn.sh/internal/bartender"
	"tavrn.sh/internal/hub"
	"tavrn.sh/internal/identity"
	"tavrn.sh/internal/jukebox"
	"tavrn.sh/internal/poll"
	"tavrn.sh/internal/session"
	"tavrn.sh/internal/store"
	"tavrn.sh/internal/sudoku"
	"tavrn.sh/ui"
)

type Config struct {
	Host          string
	Port          int
	HostKeyPath   string
	Store         *store.Store
	Hub           *hub.Hub
	JukeboxEngine *jukebox.Engine
	SudokuGame    *sudoku.Game
	PollStore     *poll.Store
	Bartender     *bartender.Bartender
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
	s.cfg.Store.RecordAllTimeVisitor(fingerprint)

	user, _ := s.cfg.Store.GetUser(fingerprint)
	visitCount := 1
	if user != nil {
		visitCount = user.VisitCount
	}

	colorIndex := identity.ColorIndex(fingerprint)
	flair := identity.HasFlair(visitCount)

	sess := session.New(fingerprint, nickname, colorIndex, flair)
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

	// Send recent chat history
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
			Timestamp:   m.CreatedAt,
		}
	}

	tavernState := func() bartender.TavernState {
		wc, _ := s.cfg.Store.WeeklyVisitorCount()
		sessions := s.cfg.Hub.Sessions("lounge")
		var names []string
		for _, sess := range sessions {
			names = append(names, sess.Nickname)
		}
		return bartender.TavernState{
			OnlineCount:     s.cfg.Hub.OnlineCount(),
			OnlineNames:     names,
			TimeUTC:         time.Now().UTC(),
			WeeklyVisitors:  wc,
			AllTimeVisitors: s.cfg.Store.AllTimeVisitorCount(),
			ActivePolls:     len(s.cfg.PollStore.ActiveRoomPolls("lounge")),
		}
	}

	gatherContext := func() []bartender.ChatMsg {
		history, _ := s.cfg.Store.RecentMessages("lounge", 50)
		var ctx []bartender.ChatMsg
		for _, m := range history {
			if !m.IsSystem {
				ctx = append(ctx, bartender.ChatMsg{Nickname: m.Nickname, Text: m.Text})
			}
		}
		return ctx
	}

	broadcastBartender := func(reply string) {
		btMsg := session.Msg{
			Type:       session.MsgChat,
			Nickname:   "bartender",
			ColorIndex: 6,
			Text:       reply,
			Room:       "lounge",
		}
		s.cfg.Store.SaveMessage("lounge", "", "bartender", 6, reply, false)
		s.cfg.Hub.Broadcast("lounge", btMsg)
	}

	onSend := func(msg session.Msg) {
		switch msg.Type {
		case session.MsgChat:
			s.cfg.Store.SaveMessage(msg.Room, msg.Fingerprint, msg.Nickname, msg.ColorIndex, msg.Text, false)
		case session.MsgSystem, session.MsgUserJoined, session.MsgUserLeft:
			s.cfg.Store.SaveMessage(msg.Room, "", "", 0, msg.Text, true)
		}
		s.cfg.Hub.Broadcast(msg.Room, msg)

		if s.cfg.Bartender == nil || msg.Type != session.MsgChat {
			return
		}

		// Direct @bartender trigger
		if bartender.ShouldRespond(msg.Text, msg.Room) {
			if s.cfg.Bartender.CanRespond(msg.Fingerprint) {
				go func() {
					s.cfg.Hub.Broadcast("lounge", session.Msg{
						Type: session.MsgTyping, Nickname: "bartender", Room: "lounge",
					})
					reply, err := s.cfg.Bartender.Respond(gatherContext(), tavernState(), msg.Fingerprint, msg.Nickname, msg.Text)
					if err != nil {
						log.Printf("bartender error: %v", err)
						broadcastBartender("Wipes the glass and says nothing.")
						return
					}
					broadcastBartender(reply)
				}()
			}
			return
		}

		// Unprompted remark — only on lounge messages
		if msg.Room == "lounge" && s.cfg.Bartender.ShouldRemark() {
			go func() {
				s.cfg.Hub.Broadcast("lounge", session.Msg{
					Type: session.MsgTyping, Nickname: "bartender", Room: "lounge",
				})
				reply, err := s.cfg.Bartender.Remark(tavernState(), gatherContext())
				if err != nil {
					log.Printf("bartender remark error: %v", err)
					return
				}
				broadcastBartender(reply)
			}()
		}
	}

	model := ui.NewApp(sess, s.cfg.Store, s.cfg.Hub, onSend, s.cfg.SudokuGame, s.cfg.PollStore)
	return model, nil
}

func (s *Server) Start(ctx context.Context) error {
	if s.cfg.JukeboxEngine != nil {
		go s.cfg.JukeboxEngine.Run(ctx)
	}

	log.Printf("tavrn.sh listening on %s:%d", s.cfg.Host, s.cfg.Port)
	return s.wish.ListenAndServe()
}

func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.wish.Shutdown(ctx)
}
