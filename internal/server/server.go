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
	"tavrn.sh/internal/gif"
	"tavrn.sh/internal/hub"
	"tavrn.sh/internal/identity"
	"tavrn.sh/internal/jukebox"
	"tavrn.sh/internal/poll"
	"tavrn.sh/internal/session"
	"tavrn.sh/internal/store"
	"tavrn.sh/internal/sudoku"
	"tavrn.sh/internal/wargame"
	"tavrn.sh/ui"
)

type Config struct {
	Host             string
	Port             int
	HostKeyPath      string
	Store            *store.Store
	Hub              *hub.Hub
	JukeboxEngine    *jukebox.Engine
	SudokuGame       *sudoku.Game
	PollStore        *poll.Store
	Bartender        *bartender.Bartender
	TavernName       string
	TavernDomain     string
	Tagline          string
	OwnerName        string
	OwnerFingerprint string
	FirstRoom        string
	RoomTypes        map[string]string
	GifClient        *gif.KlipyClient
	WargameStore     *wargame.Store
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

	firstRoom := s.cfg.FirstRoom

	sess := session.New(fingerprint, nickname, colorIndex, flair)
	s.cfg.Hub.Register(sess)

	go func() {
		<-sshSess.Context().Done()
		s.cfg.Hub.Unregister(sess)
		s.cfg.Hub.Broadcast(firstRoom, session.Msg{
			Type: session.MsgUserLeft,
			Text: fmt.Sprintf("%s left the tavern", sess.Nickname),
			Room: firstRoom,
		})
	}()

	s.cfg.Hub.Broadcast(firstRoom, session.Msg{
		Type: session.MsgUserJoined,
		Text: fmt.Sprintf("%s joined the tavern", nickname),
		Room: firstRoom,
	})

	// Send recent chat history
	history, _ := s.cfg.Store.RecentMessages(firstRoom, 50)

	// Collect GIF URLs from history to re-render (max 10)
	type gifHistoryItem struct {
		index int
		url   string
	}
	var gifItems []gifHistoryItem
	for i, m := range history {
		if m.GifURL != "" && len(gifItems) < 10 {
			gifItems = append(gifItems, gifHistoryItem{index: i, url: m.GifURL})
		}
	}

	// Pre-render GIF frames for history (background-friendly)
	gifFrameCache := make(map[int]struct {
		frames []string
		delays []int
	})
	if s.cfg.GifClient != nil && len(gifItems) > 0 {
		log.Printf("gif history: re-rendering %d GIFs for joining user", len(gifItems))
		for _, gi := range gifItems {
			data, err := s.cfg.GifClient.FetchGIF(gi.url)
			if err != nil {
				log.Printf("gif history: fetch failed: %v", err)
				continue
			}
			decoded, err := gif.Decode(data)
			if err != nil {
				log.Printf("gif history: decode failed: %v", err)
				continue
			}
			frames := gif.RenderFrames(decoded.Frames, 60)
			gifFrameCache[gi.index] = struct {
				frames []string
				delays []int
			}{frames: frames, delays: decoded.Delays}
			log.Printf("gif history: rendered %d frames for %s", len(frames), gi.url)
		}
	}

	for i, m := range history {
		if m.GifURL != "" {
			if cached, ok := gifFrameCache[i]; ok {
				sess.Send <- session.Msg{
					Type:        session.MsgGif,
					Nickname:    m.Nickname,
					Fingerprint: m.Fingerprint,
					ColorIndex:  m.ColorIndex,
					Text:        m.Text,
					Room:        m.Room,
					Timestamp:   m.CreatedAt,
					GifFrames:   cached.frames,
					GifDelays:   cached.delays,
					GifTitle:    m.Text,
					GifURL:      m.GifURL,
				}
				continue
			}
		}
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
		sessions := s.cfg.Hub.Sessions(firstRoom)
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
			ActivePolls:     len(s.cfg.PollStore.ActiveRoomPolls(firstRoom)),
		}
	}

	gatherContext := func() []bartender.ChatMsg {
		history, _ := s.cfg.Store.RecentMessages(firstRoom, 50)
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
			Room:       firstRoom,
		}
		s.cfg.Store.SaveMessage(firstRoom, "", "bartender", 6, reply, false)
		s.cfg.Hub.Broadcast(firstRoom, btMsg)
	}

	onSend := func(msg session.Msg) {
		switch msg.Type {
		case session.MsgChat:
			s.cfg.Store.SaveMessage(msg.Room, msg.Fingerprint, msg.Nickname, msg.ColorIndex, msg.Text, false)
		case session.MsgGif:
			s.cfg.Store.SaveMessageWithGif(msg.Room, msg.Fingerprint, msg.Nickname, msg.ColorIndex, msg.Text, false, msg.GifURL)
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
					// Keep typing indicator alive until API responds
					done := make(chan struct{})
					go func() {
						ticker := time.NewTicker(3 * time.Second)
						defer ticker.Stop()
						s.cfg.Hub.Broadcast(firstRoom, session.Msg{
							Type: session.MsgTyping, Nickname: "bartender", Room: firstRoom,
						})
						for {
							select {
							case <-done:
								return
							case <-ticker.C:
								s.cfg.Hub.Broadcast(firstRoom, session.Msg{
									Type: session.MsgTyping, Nickname: "bartender", Room: firstRoom,
								})
							}
						}
					}()
					reply, err := s.cfg.Bartender.Respond(gatherContext(), tavernState(), msg.Fingerprint, msg.Nickname, msg.Text, s.cfg.Store.IsOwner(msg.Fingerprint))
					close(done)
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

		// Unprompted remark — only on first-room messages
		if msg.Room == firstRoom && s.cfg.Bartender.ShouldRemark() {
			go func() {
				done := make(chan struct{})
				go func() {
					ticker := time.NewTicker(3 * time.Second)
					defer ticker.Stop()
					s.cfg.Hub.Broadcast(firstRoom, session.Msg{
						Type: session.MsgTyping, Nickname: "bartender", Room: firstRoom,
					})
					for {
						select {
						case <-done:
							return
						case <-ticker.C:
							s.cfg.Hub.Broadcast(firstRoom, session.Msg{
								Type: session.MsgTyping, Nickname: "bartender", Room: firstRoom,
							})
						}
					}
				}()
				reply, err := s.cfg.Bartender.Remark(tavernState(), gatherContext())
				close(done)
				if err != nil {
					log.Printf("bartender remark error: %v", err)
					return
				}
				broadcastBartender(reply)
			}()
		}
	}

	model := ui.NewApp(sess, s.cfg.Store, s.cfg.Hub, onSend, s.cfg.SudokuGame, s.cfg.PollStore,
		s.cfg.TavernName, s.cfg.TavernDomain, s.cfg.Tagline,
		s.cfg.OwnerName, s.cfg.OwnerFingerprint, s.cfg.FirstRoom,
		s.cfg.RoomTypes, s.cfg.GifClient, s.cfg.WargameStore)
	return model, nil
}

func (s *Server) Start(ctx context.Context) error {
	if s.cfg.JukeboxEngine != nil {
		go s.cfg.JukeboxEngine.Run(ctx)
	}

	log.Printf("%s listening on %s:%d", s.cfg.TavernDomain, s.cfg.Host, s.cfg.Port)
	return s.wish.ListenAndServe()
}

func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.wish.Shutdown(ctx)
}
