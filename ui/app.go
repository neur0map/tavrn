package ui

import (
	"fmt"
	"image/color"
	"math"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/harmonica"
	"tavrn.sh/internal/chat"
	"tavrn.sh/internal/dm"
	"tavrn.sh/internal/gif"
	"tavrn.sh/internal/hub"
	"tavrn.sh/internal/identity"
	"tavrn.sh/internal/mention"
	"tavrn.sh/internal/poll"
	"tavrn.sh/internal/reddit"
	"tavrn.sh/internal/sanitize"
	"tavrn.sh/internal/session"
	"tavrn.sh/internal/store"
	"tavrn.sh/internal/sudoku"
	"tavrn.sh/internal/wargame"
)

type HubMsg session.Msg
type tickMsg time.Time

type feedCommentsMsg struct {
	comments []reddit.Comment
	post     *reddit.Post
}

type feedPostsMsg struct {
	posts []reddit.Post
}

type feedThumbnailMsg struct {
	postID   string
	rendered string
}

type appState int

const (
	stateSplash appState = iota
	stateTransition
	stateTavern
)

const (
	splashTickInterval     = 150 * time.Millisecond
	transitionTickInterval = time.Second / 30
	typingTickInterval     = 250 * time.Millisecond
	idleTickInterval       = 2 * time.Second
)

type App struct {
	state          appState
	splash         Splash
	session        *session.Session
	chat           ChatView
	topBar         TopBar
	bottomBar      BottomBar
	rooms          RoomsPanel
	online         OnlinePanel
	width          int
	height         int
	chatWidth      int
	store          *store.Store
	hub            *hub.Hub
	onSend         func(session.Msg)
	lastTypingSent time.Time

	// Gallery
	gallery GalleryView

	// Reddit feed
	feed         FeedView
	feedActive   bool
	feedFocused  bool // true = feed has input, false = chat has input
	redditClient *reddit.Client

	// Modal
	modal           ModalType
	helpModal       HelpModal
	nickModal       NickModal
	joinRoomModal   JoinRoomModal
	postModal       PostModal
	expandNoteModal ExpandNoteModal
	mentionModal    MentionModal
	pollModal       PollModal
	pollVoteOverlay PollVoteOverlay
	changelogModal  ChangelogModal
	gifModal        GifModal

	// Polls
	pollStore *poll.Store

	// GIF search
	gifClient *gif.KlipyClient

	// Wargame CTF
	wargameStore      *wargame.Store
	submitModal       SubmitModal
	leaderboardModal  LeaderboardModal
	wargameRulesModal WargameRulesModal
	seenWargameRooms  map[string]bool

	// Mentions
	mentions []mention.Mention

	// Sudoku
	sudokuView *SudokuView
	sudokuGame *sudoku.Game

	// Direct messages
	dmStore       *dm.Store
	dmMode        bool // true = DM screen, false = tavern
	dmInbox       DMInbox
	dmChatView    ChatView
	dmInConvo     bool   // true = viewing a conversation, false = inbox
	dmPeerFP      string // current DM partner fingerprint
	dmPeerNick    string // current DM partner nickname
	dmUnreadCache int

	// Cached sidebar/topbar data (refreshed on tick, not every render)
	cachedWeeklyCount    int
	cachedAllTimeCount   int
	cachedRoomInfos      []RoomInfo
	cachedActivityCounts map[string]int
	cachedSSHLinks       []string
	cachedOnlineNames    []string
	cachedLeaderboard    []LeaderboardMini

	// Tankard collectible
	tankard        TankardView
	tankardFocused bool

	// Transition animation
	transSpring harmonica.Spring
	transPos    float64 // 0.0 = fully hidden, 1.0 = fully revealed
	transVel    float64

	// Config-driven branding
	tavernName       string
	tavernDomain     string
	tagline          string
	ownerName        string
	ownerFingerprint string
	firstRoom        string
	roomTypes        map[string]string
}

// roomByType returns the name of the first room with the given type.
func (a App) roomByType(roomType string) string {
	for name, rt := range a.roomTypes {
		if rt == roomType {
			return name
		}
	}
	return ""
}

// wargameRoomNames returns all room names with type "wargame".
func (a App) wargameRoomNames() []string {
	var names []string
	for name, rt := range a.roomTypes {
		if rt == "wargame" {
			names = append(names, name)
		}
	}
	return names
}

func NewApp(sess *session.Session, st *store.Store, h *hub.Hub, onSend func(session.Msg),
	game *sudoku.Game, ps *poll.Store,
	tavernName, tavernDomain, tagline, ownerName, ownerFingerprint, firstRoom string,
	roomTypes map[string]string, gifClient *gif.KlipyClient, ws *wargame.Store,
	ds *dm.Store, rc *reddit.Client) App {
	app := App{
		state:            stateSplash,
		splash:           NewSplash(sess.Nickname, sess.Fingerprint, sess.Flair, tavernDomain, tagline),
		session:          sess,
		chat:             NewChatView(),
		topBar:           TopBar{TavernName: tavernName, Room: firstRoom},
		bottomBar:        NewBottomBar(),
		rooms:            NewRoomsPanel(),
		online:           NewOnlinePanel(),
		gallery:          NewGalleryView(sess.Fingerprint),
		feed:             NewFeedView(rc),
		redditClient:     rc,
		store:            st,
		hub:              h,
		onSend:           onSend,
		modal:            ModalNone,
		sudokuGame:       game,
		pollStore:        ps,
		tavernName:       tavernName,
		tavernDomain:     tavernDomain,
		tagline:          tagline,
		ownerName:        ownerName,
		ownerFingerprint: ownerFingerprint,
		firstRoom:        firstRoom,
		roomTypes:        roomTypes,
		gifClient:        gifClient,
		wargameStore:     ws,
		seenWargameRooms: make(map[string]bool),
		dmStore:          ds,
	}
	app.chat.SetOwnNickname(sess.Nickname)
	app.chat.OwnerName = ownerName
	app.chat.OwnerFingerprint = ownerFingerprint
	drinkCount, _ := st.GetDrinkCount(sess.Fingerprint)
	app.tankard = NewTankardView()
	app.tankard.count = drinkCount
	return app
}

func WaitForHubMsg(ch <-chan session.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return tea.Quit()
		}
		return HubMsg(msg)
	}
}

func doTick(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (a App) Init() tea.Cmd {
	return tea.Batch(WaitForHubMsg(a.session.Send), doTick(a.nextTickInterval()))
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case feedCommentsMsg:
		a.feed.SetComments(msg.comments, msg.post)
		return a, nil
	case feedPostsMsg:
		a.feed.SetPosts(msg.posts)
		return a, nil
	case feedThumbnailMsg:
		// Thumbnail is already stored in server-wide cache by loadThumbnail.
		// Returning here triggers a redraw to show it.
		return a, nil
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		if a.state == stateSplash {
			a.splash.width = msg.Width
			a.splash.height = msg.Height
			if !a.splash.inited {
				a.splash.initSparks()
			}
		} else {
			a.doLayout()
		}
		return a, nil

	case tickMsg:
		if a.hub.OnlineCount() > 0 {
			a.online.Frame++
		}
		if a.state == stateTavern {
			a.refreshCaches()
			a.chat.Tick()
			if !a.chat.IsScrolling() && TickGifAnimations(a.chat.messages) {
				a.chat.renderMessages()
			}
			if a.dmMode && a.dmInConvo {
				a.dmChatView.Tick()
			}
			a.pruneExpiredMentions()
			if a.sudokuView != nil {
				a.sudokuView.Tick()
			}
			// Refresh reddit feed
			if a.feedActive && a.redditClient != nil {
				if a.redditClient.NeedsRefresh() {
					subs := a.store.FeedSubreddits()
					if len(subs) > 0 {
						rc := a.redditClient
						go func() {
							rc.FetchMerged(subs, 25)
						}()
					}
				}
				posts, _ := a.redditClient.Posts()
				if len(posts) > 0 && len(posts) != len(a.feed.posts) {
					a.feed.SetPosts(posts)
				}
			}
			// Load thumbnails for visible feed posts (up to 3 concurrent)
			if a.feedActive && a.redditClient != nil {
				var thumbCmds []tea.Cmd
				for i := 0; i < 3; i++ {
					p := a.feed.NextThumbToLoad()
					if p == nil {
						break
					}
					if a.redditClient.MarkThumbLoading(p.ID) {
						thumbCmds = append(thumbCmds, a.loadThumbnail(p.ID, p.PreviewURL))
					}
				}
				if len(thumbCmds) > 0 {
					thumbCmds = append(thumbCmds, doTick(a.nextTickInterval()))
					return a, tea.Batch(thumbCmds...)
				}
			}
		}
		if a.state == stateSplash {
			a.splash.frame++
			a.splash.tickSparks()
		}
		if a.state == stateTransition {
			a.transPos, a.transVel = a.transSpring.Update(a.transPos, a.transVel, 1.0)
			// Snap to done when close enough
			if math.Abs(a.transPos-1.0) < 0.01 {
				a.transPos = 1.0
				a.state = stateTavern
			}
		}
		return a, doTick(a.nextTickInterval())

	case HubMsg:
		inner := session.Msg(msg)
		a.handleHubMsg(inner)
		return a, WaitForHubMsg(a.session.Send)

	case tankardTickMsg:
		var cmd tea.Cmd
		a.tankard, cmd = a.tankard.Update(msg)
		return a, cmd

	case CloseModalMsg:
		a.modal = ModalNone
		return a, nil

	case NickChangeMsg:
		return a.applyNickChange(msg.Nick)

	case JoinRoomMsg:
		a.modal = ModalNone
		if msg.Room != a.session.Room {
			a.switchRoom(msg.Room)
		}
		return a, nil

	case PostNoteMsg:
		a.modal = ModalNone
		galleryRoom := a.roomByType("gallery")
		if a.session.Room != galleryRoom {
			a.switchRoom(galleryRoom)
		}
		text := sanitize.CleanNote(msg.Text)
		x, y := a.gallery.RandomPosition()
		noteID, err := a.store.CreateNote(x, y, text, a.session.Fingerprint, a.session.Nickname, a.session.ColorIndex)
		if err != nil {
			return a, nil
		}
		note := GalleryNote{
			ID: noteID, X: x, Y: y,
			Text: text, Nickname: a.session.Nickname,
			Fingerprint: a.session.Fingerprint, ColorIndex: a.session.ColorIndex,
		}
		a.gallery.AddNote(note)
		a.onSend(session.Msg{
			Type: session.MsgNoteCreate,
			Room: galleryRoom,
			Note: &session.NoteData{
				ID: noteID, X: x, Y: y,
				Text: text, Nick: a.session.Nickname, Color: a.session.ColorIndex,
			},
			Fingerprint: a.session.Fingerprint,
		})
		return a, nil

	case MentionJumpMsg:
		a.modal = ModalNone
		if msg.Room != a.session.Room {
			a.switchRoom(msg.Room)
		}
		return a, nil

	case GalleryExpandMsg:
		isOwn := msg.Note.Fingerprint == a.session.Fingerprint
		a.modal = ModalExpandNote
		a.expandNoteModal = NewExpandNoteModal(
			msg.Note.Text, msg.Note.Nickname, msg.Note.ColorIndex, isOwn, msg.Note.ID)
		return a, nil

	case GalleryDeleteMsg:
		a.store.DeleteNote(msg.NoteID, a.session.Fingerprint)
		a.gallery.RemoveNote(msg.NoteID)
		a.onSend(session.Msg{
			Type: session.MsgNoteDelete,
			Room: a.roomByType("gallery"),
			Note: &session.NoteData{ID: msg.NoteID},
		})
		return a, nil

	case GalleryMoveMsg:
		a.store.MoveNote(msg.NoteID, msg.X, msg.Y, a.session.Fingerprint)
		a.onSend(session.Msg{
			Type: session.MsgNoteMove,
			Room: a.roomByType("gallery"),
			Note: &session.NoteData{ID: msg.NoteID, X: msg.X, Y: msg.Y},
		})
		return a, nil

	case PollCreateMsg:
		a.modal = ModalNone
		p := a.pollStore.Create(a.session.Room, a.session.Fingerprint, a.session.Nickname, msg.Title, msg.Options)
		a.chat.AddMessage(chat.Message{
			Room:   a.session.Room,
			Text:   RenderPollCard(p),
			IsPoll: true,
		})
		a.onSend(session.Msg{
			Type:        session.MsgPollCreate,
			Room:        a.session.Room,
			Fingerprint: a.session.Fingerprint,
			PollID:      p.ID,
		})
		return a, nil

	case PollVoteMsg:
		a.pollStore.Vote(msg.PollID, a.session.Fingerprint, msg.OptionIndex)
		// Refresh the overlay
		polls := a.pollStore.RoomPolls(a.session.Room)
		a.pollVoteOverlay.SetPolls(polls)
		a.onSend(session.Msg{
			Type:        session.MsgPollVote,
			Room:        a.session.Room,
			Fingerprint: a.session.Fingerprint,
			PollID:      msg.PollID,
		})
		return a, nil

	case GifSendMsg:
		a.modal = ModalNone
		// Don't add locally — the broadcast will deliver it to everyone including us
		a.onSend(session.Msg{
			Type:        session.MsgGif,
			Room:        a.session.Room,
			Nickname:    a.session.Nickname,
			Fingerprint: a.session.Fingerprint,
			ColorIndex:  a.session.ColorIndex,
			Text:        msg.Title,
			GifFrames:   msg.Frames,
			GifDelays:   msg.Delays,
			GifTitle:    msg.Title,
			GifURL:      msg.URL,
		})
		return a, nil

	case SubmitFlagMsg:
		a.modal = ModalNone
		if a.wargameStore == nil {
			return a, nil
		}
		ok, newLevel, err := a.wargameStore.SubmitFlag(a.session.Fingerprint, a.session.Room, msg.Flag)
		if err != nil {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, err.Error()))
			return a, nil
		}
		if !ok {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Wrong flag. Try again."))
			return a, nil
		}
		// Success — announce to the entire tavern
		totalLevel := a.wargameStore.UserTotalLevel(a.session.Fingerprint)
		totalPts := a.wargameStore.UserTotalPoints(a.session.Fingerprint)
		announcement := fmt.Sprintf(">> %s hacked %s level %d  [Lv.%d | %d pts]",
			a.session.Nickname, strings.ToUpper(a.session.Room), newLevel, totalLevel, totalPts)
		a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, announcement))
		// Broadcast as banner so all rooms see it
		a.hub.BroadcastAll(session.Msg{
			Type: session.MsgBanner,
			Text: announcement,
		})
		return a, nil

	case WargameSignupMsg:
		if a.wargameStore != nil {
			a.wargameStore.Signup(a.session.Fingerprint)
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room,
				fmt.Sprintf("%s joined the wargames", a.session.Nickname)))
			a.onSend(session.Msg{
				Type: session.MsgSystem,
				Room: a.session.Room,
				Text: fmt.Sprintf("%s joined the wargames", a.session.Nickname),
			})
		}
		a.modal = ModalNone
		return a, nil

	case DMSendMsg:
		if a.dmStore != nil {
			a.dmStore.Send(a.session.Fingerprint, msg.ToFP, a.session.Nickname, msg.Text)
			a.dmChatView.AddMessage(chat.NewUserMessage(
				a.session.Fingerprint, a.session.Nickname, "dm", msg.Text, a.session.ColorIndex))
			a.onSend(session.Msg{
				Type:        session.MsgDM,
				Fingerprint: a.session.Fingerprint,
				Nickname:    a.session.Nickname,
				ColorIndex:  a.session.ColorIndex,
				Text:        msg.ToFP + "\x00" + msg.Text,
				Room:        "dm",
			})
		}
		return a, nil

	case DMOpenConvoMsg:
		a.dmPeerFP = msg.PeerFP
		a.dmPeerNick = msg.PeerNick
		a.dmInConvo = true
		a.openDMConvo(msg.PeerFP, msg.PeerNick)
		return a, nil

	case DMBackToInboxMsg:
		a.dmInConvo = false
		a.dmPeerFP = ""
		a.dmPeerNick = ""
		if a.dmStore != nil {
			convos := a.dmStore.Conversations(a.session.Fingerprint)
			a.dmInbox.SetConversations(convos)
		}
		return a, nil
	}

	// Splash state — handle keys directly (tick/resize handled above)
	if a.state == stateSplash {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			// Changelog modal open on splash — only Esc closes it
			if a.modal == ModalChangelog {
				if keyMsg.String() == "esc" {
					a.modal = ModalNone
				}
				return a, nil
			}
			switch keyMsg.String() {
			case "enter", "y":
				a.state = stateTransition
				a.transSpring = harmonica.NewSpring(harmonica.FPS(30), 6.0, 0.8)
				a.transPos = 0.0
				a.transVel = 0.0
				a.doLayout()
				a.refreshCaches()
				a.chat.AddMessage(chat.NewSystemMessage(a.session.Room,
					"Welcome to the tavern. Type /help for commands."))
				if banner := a.store.GetBanner(); banner != "" {
					a.chat.AddMessage(chat.NewBannerMessage(a.session.Room, banner))
				}
				// Restore active poll cards
				for _, p := range a.pollStore.RoomPolls(a.session.Room) {
					p := p
					a.chat.AddMessage(chat.Message{
						Room:   a.session.Room,
						Text:   RenderPollCard(&p),
						IsPoll: true,
					})
				}
				return a, nil
			case "c":
				a.modal = ModalChangelog
				a.changelogModal = NewChangelogModal()
				return a, nil
			case "q", "ctrl+c":
				return a, tea.Quit
			}
		}
		return a, nil
	}

	// Modal captures all input when open
	if a.modal != ModalNone {
		return a.updateModal(msg)
	}

	// Global keybinds — F-keys and ctrl sequences safe for SSH
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "f1":
			a.modal = ModalHelp
			a.helpModal = NewHelpModal()
			return a, nil
		case "?":
			if !a.dmMode && (a.roomTypes[a.session.Room] == "gallery" || !a.chat.HasInput()) {
				a.modal = ModalHelp
				a.helpModal = NewHelpModal()
				return a, nil
			}
		case "f2":
			a.modal = ModalNick
			a.nickModal = NewNickModal(a.session.Nickname)
			return a, nil
		case "f3":
			allRooms := a.store.AllRooms()
			var counts []int
			for _, rName := range allRooms {
				counts = append(counts, len(a.hub.Sessions(rName)))
			}
			a.modal = ModalJoinRoom
			a.joinRoomModal = NewJoinRoomModal(allRooms, counts, a.session.Room, a.roomTypes)
			return a, nil
		case "f4":
			unread := a.unreadMentions()
			if len(unread) == 0 {
				a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "No recent mentions"))
				return a, nil
			}
			contexts := a.buildMentionContexts(unread)
			a.modal = ModalMention
			a.mentionModal = NewMentionModal(unread, contexts)
			// Mark first as read
			a.markMentionRead(0)
			return a, nil
		case "f5":
			a.modal = ModalPost
			a.postModal = NewPostModal()
			return a, nil
		case "f6":
			a.tankardFocused = !a.tankardFocused
			a.tankard.focused = a.tankardFocused
			return a, nil
		case "f7":
			if a.wargameStore != nil {
				entries := a.wargameStore.Leaderboard(10)
				progress := a.wargameStore.UserProgress(a.session.Fingerprint, a.wargameRoomNames()...)
				a.modal = ModalLeaderboard
				a.leaderboardModal = NewLeaderboardModal(entries, progress, a.session.Fingerprint)
				return a, nil
			}
		case "shift+tab":
			if a.session.Room == a.firstRoom && a.redditClient != nil {
				a.feedActive = !a.feedActive
				if a.feedActive {
					a.feedFocused = true
					a.chat.input.Blur()
				} else {
					a.feedFocused = false
					a.chat.input.Focus()
				}
				a.doLayout()
				if a.feedActive && len(a.feed.posts) == 0 {
					a.feed.loading = true
					subs := a.store.FeedSubreddits()
					if len(subs) > 0 {
						rc := a.redditClient
						return a, func() tea.Msg {
							posts, _ := rc.FetchMerged(subs, 25)
							return feedPostsMsg{posts: posts}
						}
					}
				}
				return a, nil
			}
		case "tab":
			// When feed is active, Tab switches focus between feed and chat
			if a.feedActive && a.session.Room == a.firstRoom && !a.dmMode {
				a.feedFocused = !a.feedFocused
				if !a.feedFocused {
					a.chat.input.Focus()
				} else {
					a.chat.input.Blur()
				}
				return a, nil
			}
			// Toggle DM mode only when input is empty and not in gallery/games
			if a.dmStore != nil && !a.chat.HasInput() &&
				a.roomTypes[a.session.Room] != "gallery" &&
				!a.chat.MentionPopupActive() {
				a.toggleDMMode()
				return a, nil
			}
		}
	}

	// Tankard captures input when focused
	if a.tankardFocused {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			switch keyMsg.String() {
			case "space", " ":
				cmd := a.tankard.Press()
				a.store.IncrementDrinkCount(a.session.Fingerprint)
				return a, cmd
			case "esc", "f6":
				a.tankardFocused = false
				a.tankard.focused = false
				return a, nil
			case "ctrl+c":
				return a, tea.Quit
			}
		}
		return a, nil
	}

	// DM mode captures all input
	if a.dmMode {
		if a.dmInConvo {
			if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
				switch keyMsg.String() {
				case "esc":
					return a, func() tea.Msg { return DMBackToInboxMsg{} }
				case "enter":
					return a.handleDMInput()
				case "ctrl+c":
					return a, tea.Quit
				}
			}
			var cmd tea.Cmd
			a.dmChatView, cmd = a.dmChatView.Update(msg)
			return a, cmd
		}
		var cmd tea.Cmd
		a.dmInbox, cmd = a.dmInbox.Update(msg)
		return a, cmd
	}

	// Gallery room: single-key shortcuts + mouse
	if a.roomTypes[a.session.Room] == "gallery" {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch msg.String() {
			case "ctrl+c", "q":
				return a, tea.Quit
			case "enter":
				return a.handleInput()
			case "p":
				a.modal = ModalPost
				a.postModal = NewPostModal()
				return a, nil
			case "n":
				a.modal = ModalNick
				a.nickModal = NewNickModal(a.session.Nickname)
				return a, nil
			case "j":
				allRooms := a.store.AllRooms()
				var counts []int
				for _, rName := range allRooms {
					counts = append(counts, len(a.hub.Sessions(rName)))
				}
				a.modal = ModalJoinRoom
				a.joinRoomModal = NewJoinRoomModal(allRooms, counts, a.session.Room, a.roomTypes)
				return a, nil
			case "h":
				a.modal = ModalHelp
				a.helpModal = NewHelpModal()
				return a, nil
			}
			// Forward d/delete/tab to gallery
			if msg.String() == "d" || msg.String() == "delete" || msg.String() == "backspace" || msg.String() == "tab" || msg.String() == "e" {
				var cmd tea.Cmd
				a.gallery, cmd = a.gallery.Update(msg)
				return a, cmd
			}
		case tea.MouseClickMsg, tea.MouseReleaseMsg, tea.MouseMotionMsg:
			var cmd tea.Cmd
			a.gallery, cmd = a.gallery.Update(msg)
			return a, cmd
		}
		// Text input still works for /post command
		var cmd tea.Cmd
		a.chat, cmd = a.chat.Update(msg)
		return a, cmd
	}

	// Games room: sudoku view
	if a.roomTypes[a.session.Room] == "games" && a.sudokuView != nil {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch msg.String() {
			case "ctrl+c":
				return a, tea.Quit
			case "esc":
				if a.sudokuView.FocusChat() {
					// ESC in chat mode just blurs chat (handled by sudoku view)
				} else {
					a.switchRoom(a.firstRoom)
					return a, nil
				}
			case "enter":
				if a.sudokuView.FocusChat() && a.sudokuView.HasChatInput() {
					text := sanitize.CleanChat(a.sudokuView.ChatInput())
					if text != "" {
						a.onSend(session.Msg{
							Type:       session.MsgChat,
							Text:       text,
							Room:       a.session.Room,
							Nickname:   a.session.Nickname,
							ColorIndex: a.session.ColorIndex,
						})
					}
					return a, nil
				}
			}
		}
		var cmd tea.Cmd
		sv := *a.sudokuView
		sv, cmd = sv.Update(msg)
		a.sudokuView = &sv
		// Check if puzzle is solved
		if a.sudokuGame.IsSolved() {
			a.sudokuView.AddMessage(chat.NewSystemMessage(a.session.Room, "Puzzle solved! New puzzle starting..."))
			a.sudokuGame.Reset()
		}
		return a, cmd
	}

	// Feed panel active in lounge — input goes to feed when focused
	if a.feedActive && a.session.Room == a.firstRoom && !a.dmMode && a.feedFocused {
		switch msg := msg.(type) {
		case tea.KeyPressMsg:
			switch msg.String() {
			case "enter":
				if !a.feed.InCommentView() {
					if post := a.feed.SelectedPost(); post != nil {
						a.feed.loadingComment = true
						postCopy := *post
						rc := a.redditClient
						return a, func() tea.Msg {
							comments, _ := rc.FetchComments(postCopy.Subreddit, postCopy.ID, 20)
							return feedCommentsMsg{comments: comments, post: &postCopy}
						}
					}
				}
				return a, nil
			case "s":
				if post := a.feed.SelectedPost(); post != nil {
					return a.shareFeedPost(post)
				}
			case "o":
				// Show clickable OSC 8 link for selected post
				if post := a.feed.SelectedPost(); post != nil {
					url := "https://reddit.com" + post.Permalink
					a.chat.AddSystemLog(osc8Link(url, url))
				}
				return a, nil
			case "esc":
				if a.feed.InCommentView() {
					a.feed.BackToList()
					return a, nil
				}
			case "ctrl+c":
				return a, tea.Quit
			}
		case tea.MouseWheelMsg:
			var cmd tea.Cmd
			a.feed, cmd = a.feed.Update(msg)
			return a, cmd
		}
		var cmd tea.Cmd
		a.feed, cmd = a.feed.Update(msg)
		return a, cmd
	}

	// Normal chat rooms
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "enter":
			if a.chat.MentionPopupActive() {
				// Let ChatView.Update handle Tab/Enter completion
				break
			}
			return a.handleInput()
		case "esc":
			return a, nil
		default:
			if time.Since(a.lastTypingSent) > 2*time.Second && a.chat.HasInput() {
				a.lastTypingSent = time.Now()
				a.onSend(session.Msg{
					Type:     session.MsgTyping,
					Nickname: a.session.Nickname,
					Room:     a.session.Room,
				})
			}
		}
	}

	var cmd tea.Cmd
	a.chat, cmd = a.chat.Update(msg)

	// Refresh mention autocomplete popup
	sessions := a.hub.Sessions(a.session.Room)
	var names []string
	for _, s := range sessions {
		if s.Fingerprint != a.session.Fingerprint {
			names = append(names, s.Nickname)
		}
	}
	if a.session.Room == a.firstRoom {
		names = append(names, "bartender")
	}
	a.chat.UpdateMentionPopup(names)

	return a, cmd
}

func (a App) updateModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "esc" {
			a.modal = ModalNone
			return a, nil
		}
	}

	switch a.modal {
	case ModalNick:
		var cmd tea.Cmd
		a.nickModal, cmd = a.nickModal.Update(msg)
		return a, cmd
	case ModalJoinRoom:
		var cmd tea.Cmd
		a.joinRoomModal, cmd = a.joinRoomModal.Update(msg)
		return a, cmd
	case ModalPost:
		var cmd tea.Cmd
		a.postModal, cmd = a.postModal.Update(msg)
		return a, cmd
	case ModalExpandNote:
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			if keyMsg.String() == "d" && a.expandNoteModal.IsOwn {
				noteID := a.expandNoteModal.NoteID
				a.modal = ModalNone
				a.store.DeleteNote(noteID, a.session.Fingerprint)
				a.gallery.RemoveNote(noteID)
				a.onSend(session.Msg{
					Type: session.MsgNoteDelete,
					Room: a.roomByType("gallery"),
					Note: &session.NoteData{ID: noteID},
				})
				return a, nil
			}
		}
		return a, nil
	case ModalHelp:
		// Help modal only responds to ESC (handled above)
		return a, nil
	case ModalMention:
		prev := a.mentionModal.Current()
		var cmd tea.Cmd
		a.mentionModal, cmd = a.mentionModal.Update(msg)
		// Mark new mention as read if user navigated
		if a.mentionModal.Current() != prev {
			a.markMentionRead(a.mentionModal.Current())
		}
		return a, cmd
	case ModalPoll:
		var cmd tea.Cmd
		a.pollModal, cmd = a.pollModal.Update(msg)
		return a, cmd
	case ModalPollVote:
		var cmd tea.Cmd
		a.pollVoteOverlay, cmd = a.pollVoteOverlay.Update(msg)
		return a, cmd
	case ModalGif:
		var cmd tea.Cmd
		a.gifModal, cmd = a.gifModal.Update(msg)
		return a, cmd
	case ModalSubmitFlag:
		var cmd tea.Cmd
		a.submitModal, cmd = a.submitModal.Update(msg)
		return a, cmd
	case ModalLeaderboard:
		var cmd tea.Cmd
		a.leaderboardModal, cmd = a.leaderboardModal.Update(msg)
		return a, cmd
	case ModalWargameRules:
		var cmd tea.Cmd
		a.wargameRulesModal, cmd = a.wargameRulesModal.Update(msg)
		return a, cmd
	}
	return a, nil
}

func (a App) applyNickChange(nick string) (tea.Model, tea.Cmd) {
	cleaned, err := sanitize.CleanNick(nick)
	if err != nil {
		a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, err.Error()))
		a.modal = ModalNone
		return a, nil
	}
	if err := a.store.SetNickname(a.session.Fingerprint, cleaned); err != nil {
		a.chat.AddMessage(chat.NewSystemMessage(a.session.Room,
			"That name is already claimed."))
		a.modal = ModalNone
		return a, nil
	}
	oldNick := a.session.Nickname
	a.session.Nickname = cleaned
	a.chat.SetOwnNickname(cleaned)
	a.modal = ModalNone
	a.onSend(session.Msg{
		Type: session.MsgSystem,
		Text: fmt.Sprintf("%s is now known as %s", oldNick, cleaned),
		Room: a.session.Room,
	})
	return a, nil
}

func (a App) shareFeedPost(post *reddit.Post) (tea.Model, tea.Cmd) {
	thumb := ""
	if a.redditClient != nil {
		if t, ok := a.redditClient.GetThumb(post.ID); ok {
			thumb = t
		}
	}
	a.onSend(session.Msg{
		Type:           session.MsgRedditShare,
		Nickname:       a.session.Nickname,
		Fingerprint:    a.session.Fingerprint,
		ColorIndex:     a.session.ColorIndex,
		Room:           a.session.Room,
		RedditTitle:    post.Title,
		RedditSub:      post.Subreddit,
		RedditScore:    post.Score,
		RedditComments: post.NumComments,
		RedditURL:      "https://reddit.com" + post.Permalink,
		RedditThumb:    thumb,
	})
	return a, nil
}

func (a App) loadThumbnail(postID, previewURL string) tea.Cmd {
	rc := a.redditClient
	// Render at feed panel width minus card border/padding
	thumbW := a.feed.width - 6
	if thumbW < 20 {
		thumbW = 20
	}
	if thumbW > 90 {
		thumbW = 90
	}
	return func() tea.Msg {
		img, err := rc.FetchImage(previewURL)
		if err != nil || img == nil {
			rc.SetThumb(postID, " ") // mark as failed, don't retry
			return feedThumbnailMsg{postID: postID, rendered: ""}
		}
		rendered := gif.RenderHalfBlocksClean(img, thumbW)
		rc.SetThumb(postID, rendered) // server-wide cache
		return feedThumbnailMsg{postID: postID, rendered: rendered}
	}
}

func (a App) handleInput() (tea.Model, tea.Cmd) {
	input := a.chat.InputValue()
	if input == "" {
		return a, nil
	}

	cleaned := sanitize.CleanChat(input)
	if cleaned == "" {
		return a, nil
	}

	parsed := chat.ParseInput(cleaned)
	if parsed.IsCommand {
		if cmd := a.handleCommand(parsed); cmd != nil {
			return a, cmd
		}
	} else {
		if a.session.ChatLimiter.Allow() {
			a.onSend(session.Msg{
				Type:        session.MsgChat,
				Nickname:    a.session.Nickname,
				Fingerprint: a.session.Fingerprint,
				ColorIndex:  a.session.ColorIndex,
				Text:        parsed.Text,
				Room:        a.session.Room,
			})
		} else {
			a.session.ChatLimiter.RecordViolation()
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room,
				"Slow down! You're sending too fast."))
		}
	}
	return a, nil
}

func (a *App) handleCommand(parsed chat.ParseResult) tea.Cmd {
	switch parsed.Command {
	case "poll":
		a.modal = ModalPoll
		a.pollModal = NewPollModal()
	case "vote":
		polls := a.pollStore.RoomPolls(a.session.Room)
		if len(polls) == 0 {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "No polls in this room."))
			return nil
		}
		a.modal = ModalPollVote
		a.pollVoteOverlay = NewPollVoteOverlay(polls, a.session.Fingerprint)
	case "endpoll":
		p := a.pollStore.LatestByCreator(a.session.Room, a.session.Fingerprint)
		if p == nil {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "You have no active polls here."))
			return nil
		}
		a.pollStore.Close(p.ID, a.session.Fingerprint)
		a.chat.AddMessage(chat.NewSystemMessage(a.session.Room,
			fmt.Sprintf("Poll closed: %s", p.Title)))
		a.onSend(session.Msg{
			Type:        session.MsgPollClose,
			Room:        a.session.Room,
			Fingerprint: a.session.Fingerprint,
			PollID:      p.ID,
		})
	case "gif":
		if a.gifClient == nil {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "GIF search is not enabled."))
			return nil
		}
		query := strings.TrimSpace(parsed.Args)
		if query == "" {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Usage: /gif <search>"))
			return nil
		}
		a.modal = ModalGif
		a.gifModal = NewGifModal(query, a.gifClient)
		return a.gifModal.Init()
	case "addssh":
		if !identity.IsOwnerFingerprint(a.session.Fingerprint, a.ownerFingerprint) {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Only the tavern owner can do that."))
			return nil
		}
		addr := strings.TrimSpace(parsed.Args)
		if addr == "" {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Usage: /addssh <address>"))
			return nil
		}
		if err := a.store.AddSSHLink(addr); err != nil {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Failed to add SSH link."))
			return nil
		}
		a.chat.AddMessage(chat.NewSystemMessage(a.session.Room,
			fmt.Sprintf("Added: %s", addr)))
	case "rmssh":
		if !identity.IsOwnerFingerprint(a.session.Fingerprint, a.ownerFingerprint) {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Only the tavern owner can do that."))
			return nil
		}
		addr := strings.TrimSpace(parsed.Args)
		if addr == "" {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Usage: /rmssh <address>"))
			return nil
		}
		if err := a.store.RemoveSSHLink(addr); err != nil {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Failed to remove SSH link."))
			return nil
		}
		a.chat.AddMessage(chat.NewSystemMessage(a.session.Room,
			fmt.Sprintf("Removed: %s", addr)))
	case "submit":
		if a.wargameStore == nil {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Wargame system not available."))
			return nil
		}
		if a.roomTypes[a.session.Room] != "wargame" {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Use /submit in a wargame room."))
			return nil
		}
		if !a.wargameStore.IsParticipant(a.session.Fingerprint) {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Sign up first — press Y in the rules modal (enter any wargame room)."))
			return nil
		}
		currentLevel := 0
		progress := a.wargameStore.UserProgress(a.session.Fingerprint, a.wargameRoomNames()...)
		for _, p := range progress {
			if p.Wargame == a.session.Room {
				currentLevel = p.Level
				break
			}
		}
		maxLevel := a.wargameStore.MaxLevel(a.session.Room)
		if currentLevel >= maxLevel && maxLevel > 0 {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "You've cleared all available levels."))
			return nil
		}
		a.modal = ModalSubmitFlag
		a.submitModal = NewSubmitModal(a.session.Room, currentLevel, maxLevel)
	case "dm":
		if a.dmStore == nil {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "DMs are not enabled."))
			return nil
		}
		args := strings.TrimSpace(parsed.Args)
		if args == "" {
			a.toggleDMMode()
			return nil
		}
		// Split into name and optional message: /dm @name hello there
		parts := strings.SplitN(args, " ", 2)
		name := strings.TrimPrefix(parts[0], "@")
		var firstMsg string
		if len(parts) > 1 {
			firstMsg = strings.TrimSpace(parts[1])
		}
		peerFP, err := a.store.FingerprintByNickname(name)
		if err != nil {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room,
				fmt.Sprintf("User '%s' not found.", name)))
			return nil
		}
		if peerFP == a.session.Fingerprint {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "You can't DM yourself."))
			return nil
		}
		a.dmMode = true
		a.dmInConvo = true
		a.dmPeerFP = peerFP
		a.dmPeerNick = name
		a.openDMConvo(peerFP, name)
		if firstMsg != "" {
			a.dmStore.Send(a.session.Fingerprint, peerFP, a.session.Nickname, firstMsg)
			a.dmChatView.AddMessage(chat.NewUserMessage(
				a.session.Fingerprint, a.session.Nickname, "dm", firstMsg, a.session.ColorIndex))
			a.onSend(session.Msg{
				Type:        session.MsgDM,
				Fingerprint: a.session.Fingerprint,
				Nickname:    a.session.Nickname,
				ColorIndex:  a.session.ColorIndex,
				Text:        peerFP + "\x00" + firstMsg,
				Room:        "dm",
			})
		}
	case "leaderboard", "lb":
		if a.wargameStore == nil {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Wargame system not available."))
			return nil
		}
		entries := a.wargameStore.Leaderboard(10)
		progress := a.wargameStore.UserProgress(a.session.Fingerprint, a.wargameRoomNames()...)
		a.modal = ModalLeaderboard
		a.leaderboardModal = NewLeaderboardModal(entries, progress, a.session.Fingerprint)
	default:
		a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Use F1 for help with keybinds."))
	}
	return nil
}

func (a *App) handleHubMsg(msg session.Msg) {
	switch msg.Type {
	case session.MsgChat:
		ts := msg.Timestamp
		if ts.IsZero() {
			ts = time.Now()
		}
		chatMsg := chat.Message{
			Nickname:   msg.Nickname,
			ColorIndex: msg.ColorIndex,
			Text:       msg.Text,
			Room:       msg.Room,
			Timestamp:  ts,
		}
		if a.roomTypes[a.session.Room] == "games" && a.sudokuView != nil {
			a.sudokuView.AddMessage(chatMsg)
		} else {
			a.chat.AddMessage(chatMsg)
		}
		a.detectMentions(msg)
	case session.MsgBanner:
		a.chat.AddMessage(chat.NewBannerMessage(a.session.Room, msg.Text))
	case session.MsgSystem, session.MsgUserJoined, session.MsgUserLeft:
		a.chat.AddMessage(chat.NewSystemMessage(msg.Room, msg.Text))
	case session.MsgPurge:
		a.chat = NewChatView()
		a.doLayout()
		a.chat.SetOwnNickname(a.session.Nickname)
		a.chat.AddMessage(chat.NewSystemMessage(a.session.Room,
			"The tavern has been swept clean."))
	case session.MsgTyping:
		if msg.Nickname != a.session.Nickname {
			a.chat.SetTyping(msg.Nickname)
		}
	case session.MsgNoteCreate:
		if msg.Note != nil && msg.Fingerprint != a.session.Fingerprint {
			a.gallery.AddNote(GalleryNote{
				ID: msg.Note.ID, X: msg.Note.X, Y: msg.Note.Y,
				Text: msg.Note.Text, Nickname: msg.Note.Nick,
				Fingerprint: msg.Fingerprint, ColorIndex: msg.Note.Color,
			})
		}
	case session.MsgNoteMove:
		if msg.Note != nil {
			a.gallery.MoveNote(msg.Note.ID, msg.Note.X, msg.Note.Y)
		}
	case session.MsgNoteDelete:
		if msg.Note != nil {
			a.gallery.RemoveNote(msg.Note.ID)
		}
	case session.MsgRoomAdded:
		a.chat.AddMessage(chat.NewSystemMessage(a.session.Room,
			fmt.Sprintf("New room available: #%s", msg.Text)))
	case session.MsgRoomRenamed:
		oldName := msg.Text
		newName := msg.Room
		if a.session.Room == oldName {
			// We're in the renamed room — update session and reload
			a.session.Room = newName
			a.topBar.Room = newName
			a.chat.AddMessage(chat.NewSystemMessage(newName,
				fmt.Sprintf("This room was renamed to #%s", newName)))
		} else {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room,
				fmt.Sprintf("Room #%s renamed to #%s", oldName, newName)))
		}
	case session.MsgRoomRemoved:
		removedRoom := msg.Text
		if a.session.Room == removedRoom {
			a.switchRoom(a.firstRoom)
			a.chat.AddMessage(chat.NewSystemMessage(a.firstRoom,
				fmt.Sprintf("Room #%s was removed. You've been moved to #%s.", removedRoom, a.firstRoom)))
		} else {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room,
				fmt.Sprintf("Room #%s has been removed", removedRoom)))
		}
	case session.MsgPollCreate:
		if msg.Fingerprint != a.session.Fingerprint {
			p := a.pollStore.Get(msg.PollID)
			if p != nil {
				a.chat.AddMessage(chat.Message{
					Room:     msg.Room,
					Text:     RenderPollCard(p),
					IsSystem: true,
				})
			}
		}
	case session.MsgPollVote:
		// Refresh vote overlay if open
		if a.modal == ModalPollVote {
			polls := a.pollStore.RoomPolls(a.session.Room)
			a.pollVoteOverlay.SetPolls(polls)
		}
	case session.MsgPollClose:
		if msg.Fingerprint != a.session.Fingerprint {
			p := a.pollStore.Get(msg.PollID)
			if p != nil {
				a.chat.AddMessage(chat.Message{
					Room:     msg.Room,
					Text:     RenderPollCard(p),
					IsSystem: true,
				})
			}
		}
		// Refresh vote overlay if open
		if a.modal == ModalPollVote {
			polls := a.pollStore.RoomPolls(a.session.Room)
			a.pollVoteOverlay.SetPolls(polls)
		}
	case session.MsgGif:
		a.chat.AddMessage(chat.Message{
			Room:       msg.Room,
			Nickname:   msg.Nickname,
			ColorIndex: msg.ColorIndex,
			Text:       msg.GifTitle,
			IsGif:      true,
			GifFrames:  msg.GifFrames,
			GifDelays:  msg.GifDelays,
			GifTitle:   msg.GifTitle,
			GifURL:     msg.GifURL,
			Timestamp:  msg.Timestamp,
		})
	case session.MsgRedditShare:
		a.chat.AddMessage(chat.Message{
			Room:           msg.Room,
			Nickname:       msg.Nickname,
			ColorIndex:     msg.ColorIndex,
			Timestamp:      time.Now(),
			IsReddit:       true,
			RedditTitle:    msg.RedditTitle,
			RedditSub:      msg.RedditSub,
			RedditScore:    msg.RedditScore,
			RedditComments: msg.RedditComments,
			RedditURL:      msg.RedditURL,
			RedditThumb:    msg.RedditThumb,
		})
	case session.MsgDM:
		parts := strings.SplitN(msg.Text, "\x00", 2)
		if len(parts) != 2 {
			break
		}
		text := parts[1]
		if a.dmMode && a.dmInConvo && a.dmPeerFP == msg.Fingerprint {
			a.dmChatView.AddMessage(chat.NewUserMessage(
				msg.Fingerprint, msg.Nickname, "dm", text, msg.ColorIndex))
			if a.dmStore != nil {
				a.dmStore.MarkRead(a.session.Fingerprint, msg.Fingerprint)
			}
		} else {
			// Show notification in tavern chat
			a.chat.AddSystemLog(fmt.Sprintf("DM from %s", msg.Nickname))
		}
	}
}

// detectMentions checks if an incoming message mentions this user.
func (a *App) detectMentions(msg session.Msg) {
	if msg.Type != session.MsgChat {
		return
	}
	if msg.Fingerprint == a.session.Fingerprint {
		return // don't notify yourself
	}
	if !mention.IsMentioned(msg.Text, a.session.Nickname) {
		return
	}

	m := mention.Mention{
		Room:       msg.Room,
		Author:     msg.Nickname,
		ColorIndex: msg.ColorIndex,
		Text:       msg.Text,
		Timestamp:  msg.Timestamp,
	}
	if m.Timestamp.IsZero() {
		m.Timestamp = time.Now()
	}
	a.mentions = append(a.mentions, m)

	// Toast notification if not currently viewing that room's chat
	if msg.Room != a.session.Room || a.roomTypes[a.session.Room] == "gallery" || a.roomTypes[a.session.Room] == "games" {
		a.chat.AddMessage(chat.NewSystemMessage(a.session.Room,
			fmt.Sprintf("%s mentioned you in #%s", msg.Nickname, msg.Room)))
	}
}

// unreadMentionCount returns the number of unread mentions for a room.
// If room is empty, returns total unread count.
const mentionTTL = 30 * time.Minute

// pruneExpiredMentions removes mentions older than 30 minutes.
func (a *App) pruneExpiredMentions() {
	now := time.Now()
	n := 0
	for _, m := range a.mentions {
		if now.Sub(m.Timestamp) < mentionTTL {
			a.mentions[n] = m
			n++
		}
	}
	a.mentions = a.mentions[:n]
}

func (a *App) unreadMentionCount(room string) int {
	count := 0
	for _, m := range a.mentions {
		if !m.Read && (room == "" || m.Room == room) {
			count++
		}
	}
	return count
}

// unreadMentions returns all unread mentions.
func (a *App) unreadMentions() []mention.Mention {
	var unread []mention.Mention
	for _, m := range a.mentions {
		if !m.Read {
			unread = append(unread, m)
		}
	}
	return unread
}

// buildMentionContexts loads 3 preceding messages for each mention from the store.
func (a *App) buildMentionContexts(mentions []mention.Mention) [][]chat.Message {
	contexts := make([][]chat.Message, len(mentions))
	for i, m := range mentions {
		rows, err := a.store.RecentMessages(m.Room, 50)
		if err != nil {
			continue
		}
		// Find messages just before the mention timestamp
		var before []chat.Message
		for _, row := range rows {
			if row.CreatedAt.Before(m.Timestamp) && !row.IsSystem {
				before = append(before, chat.Message{
					Nickname:   row.Nickname,
					ColorIndex: row.ColorIndex,
					Text:       row.Text,
					Room:       row.Room,
					Timestamp:  row.CreatedAt,
				})
			}
		}
		// Take last 3
		if len(before) > 3 {
			before = before[len(before)-3:]
		}
		contexts[i] = before
	}
	return contexts
}

// markMentionRead marks a mention at the given index (in unread list) as read.
func (a *App) markMentionRead(unreadIdx int) {
	count := 0
	for i := range a.mentions {
		if !a.mentions[i].Read {
			if count == unreadIdx {
				a.mentions[i].Read = true
				return
			}
			count++
		}
	}
}

func (a *App) refreshCaches() {
	a.cachedWeeklyCount, _ = a.store.WeeklyVisitorCount()
	a.cachedAllTimeCount = a.store.AllTimeVisitorCount()

	allRooms := a.store.AllRooms()
	roomInfos := make([]RoomInfo, 0, len(allRooms))
	for _, rName := range allRooms {
		roomInfos = append(roomInfos, RoomInfo{
			Name:  rName,
			Count: len(a.hub.Sessions(rName)),
		})
	}
	a.cachedRoomInfos = roomInfos
	a.cachedActivityCounts = a.store.RecentActivityCounts(10)
	a.cachedSSHLinks = a.store.AllSSHLinks()

	// Online names for current room
	sessions := a.hub.Sessions(a.session.Room)
	var names []string
	for _, s := range sessions {
		name := s.Nickname
		if identity.IsOwnerFingerprint(s.Fingerprint, a.ownerFingerprint) {
			name = identity.OwnerDisplayName(a.ownerName)
		} else if s.Flair {
			name = "~" + name
		}
		if a.wargameStore != nil {
			if lvl := a.wargameStore.UserTotalLevel(s.Fingerprint); lvl > 0 {
				name = fmt.Sprintf("%s %d", name, lvl)
			}
		}
		names = append(names, name)
	}
	if a.session.Room == a.firstRoom {
		names = append(names, "◆ bartender")
	}
	sort.Strings(names)
	a.cachedOnlineNames = names

	// Leaderboard
	if a.wargameStore != nil {
		entries := a.wargameStore.Leaderboard(3)
		mini := make([]LeaderboardMini, 0, len(entries))
		for _, e := range entries {
			mini = append(mini, LeaderboardMini{
				Name:   e.Nickname,
				Level:  e.TotalLevel,
				Points: e.TotalPoints,
			})
		}
		a.cachedLeaderboard = mini
	}

	// DM unread
	if a.dmStore != nil {
		a.dmUnreadCache = a.dmStore.UnreadCount(a.session.Fingerprint)
	}
}

func (a *App) openDMConvo(peerFP, peerNick string) {
	a.dmChatView = NewChatView()
	a.dmChatView.SetOwnNickname(a.session.Nickname)
	a.doLayout()
	if a.dmStore != nil {
		msgs, _ := a.dmStore.Messages(a.session.Fingerprint, peerFP, 50)
		for _, m := range msgs {
			a.dmChatView.AddMessage(chat.Message{
				Fingerprint: m.FromFP,
				Nickname:    m.FromNick,
				Room:        "dm",
				Text:        m.Text,
				Timestamp:   m.CreatedAt,
			})
		}
		a.dmStore.MarkRead(a.session.Fingerprint, peerFP)
	}
}

func (a App) handleDMInput() (tea.Model, tea.Cmd) {
	input := a.dmChatView.InputValue()
	if input == "" {
		return a, nil
	}
	text := sanitize.CleanChat(input)
	if text == "" {
		return a, nil
	}
	if a.dmStore != nil && a.dmPeerFP != "" {
		a.dmStore.Send(a.session.Fingerprint, a.dmPeerFP, a.session.Nickname, text)
		a.dmChatView.AddMessage(chat.NewUserMessage(
			a.session.Fingerprint, a.session.Nickname, "dm", text, a.session.ColorIndex))
		a.onSend(session.Msg{
			Type:        session.MsgDM,
			Fingerprint: a.session.Fingerprint,
			Nickname:    a.session.Nickname,
			ColorIndex:  a.session.ColorIndex,
			Text:        a.dmPeerFP + "\x00" + text,
			Room:        "dm",
		})
	}
	return a, nil
}

func (a *App) toggleDMMode() {
	a.dmMode = !a.dmMode
	if a.dmMode {
		a.dmInConvo = false
		a.dmPeerFP = ""
		a.dmPeerNick = ""
		if a.dmStore != nil {
			convos := a.dmStore.Conversations(a.session.Fingerprint)
			a.dmInbox = NewDMInbox(convos)
		}
		a.doLayout()
	}
}

func (a *App) switchRoom(target string) {
	oldRoom := a.session.Room

	// Announce leave in old room
	a.onSend(session.Msg{
		Type: session.MsgUserLeft,
		Text: fmt.Sprintf("%s left for #%s", a.session.Nickname, target),
		Room: oldRoom,
	})

	// Switch
	a.session.Room = target

	// Clear and load new room content
	a.chat = NewChatView()
	a.chat.OwnerName = a.ownerName
	a.chat.OwnerFingerprint = a.ownerFingerprint
	a.doLayout()
	a.chat.SetOwnNickname(a.session.Nickname)

	if a.roomTypes[target] == "gallery" {
		// Load gallery notes
		notes, _ := a.store.AllNotes()
		a.gallery = NewGalleryView(a.session.Fingerprint)
		a.doLayout() // sets size and screen offset
		a.gallery.LoadNotes(notes)
	} else if a.roomTypes[target] == "games" && a.sudokuGame != nil {
		sv := NewSudokuView(a.sudokuGame, a.session.Fingerprint, a.session.Nickname, a.session.ColorIndex)
		a.sudokuView = &sv
		a.doLayout()
		a.sudokuGame.RegisterNickname(a.session.Fingerprint, a.session.Nickname)
		a.sudokuGame.SetCursor(a.session.Fingerprint, 0, 0)
		// Load chat history into the game chat
		history, _ := a.store.RecentMessages(target, 50)
		for _, m := range history {
			a.sudokuView.AddMessage(chat.Message{
				Nickname:   m.Nickname,
				ColorIndex: m.ColorIndex,
				Text:       m.Text,
				Room:       m.Room,
				Timestamp:  m.CreatedAt,
				IsSystem:   m.IsSystem,
			})
		}
	} else {
		// Load chat history
		history, _ := a.store.RecentMessages(target, 50)
		gifCount := 0
		for _, m := range history {
			if m.GifURL != "" && a.gifClient != nil && gifCount < 10 {
				data, err := a.gifClient.FetchGIF(m.GifURL)
				if err == nil {
					decoded, decErr := gif.Decode(data)
					if decErr == nil {
						gifW := a.chatWidth - 10
						if gifW < 20 {
							gifW = 20
						}
						if gifW > 60 {
							gifW = 60
						}
						frames := gif.RenderFrames(decoded.Frames, gifW)
						a.chat.AddMessage(chat.Message{
							Nickname:   m.Nickname,
							ColorIndex: m.ColorIndex,
							Text:       m.Text,
							Room:       m.Room,
							Timestamp:  m.CreatedAt,
							IsGif:      true,
							GifFrames:  frames,
							GifDelays:  decoded.Delays,
							GifTitle:   m.Text,
							GifURL:     m.GifURL,
						})
						gifCount++
						continue
					}
				}
			}
			msg := chat.Message{
				Nickname:   m.Nickname,
				ColorIndex: m.ColorIndex,
				Text:       m.Text,
				Room:       m.Room,
				Timestamp:  m.CreatedAt,
				IsSystem:   m.IsSystem,
			}
			a.chat.AddMessage(msg)
		}
		a.chat.AddMessage(chat.NewSystemMessage(target,
			fmt.Sprintf("You joined #%s", target)))
		// Restore active poll cards
		for _, p := range a.pollStore.RoomPolls(target) {
			p := p
			a.chat.AddMessage(chat.Message{
				Room:   target,
				Text:   RenderPollCard(&p),
				IsPoll: true,
			})
		}
	}

	// Announce join in new room
	a.onSend(session.Msg{
		Type: session.MsgUserJoined,
		Text: fmt.Sprintf("%s joined from #%s", a.session.Nickname, oldRoom),
		Room: target,
	})

	// Update top bar
	a.topBar.Room = target

	// Show wargame rules on first visit
	if a.roomTypes[target] == "wargame" && !a.seenWargameRooms[target] {
		a.seenWargameRooms[target] = true
		a.modal = ModalWargameRules
		isParticipant := a.wargameStore != nil && a.wargameStore.IsParticipant(a.session.Fingerprint)
		a.wargameRulesModal = NewWargameRulesModal(target, isParticipant)
	}
}

func (a *App) doLayout() {
	roomsWidth := 0
	onlineWidth := 0
	if a.width >= 140 {
		roomsWidth = 18
		onlineWidth = 20
	} else if a.width >= 120 {
		roomsWidth = 16
		onlineWidth = 18
	} else if a.width >= 100 {
		// 2-column: rooms | chat
		roomsWidth = 16
		onlineWidth = 0
	}
	chatWidth := a.width - roomsWidth - onlineWidth
	if chatWidth < 40 {
		roomsWidth = 0
		onlineWidth = 0
		chatWidth = a.width
	}

	// When feed is active, hide the online sidebar and reclaim the space
	feedWidth := 0
	if a.feedActive && a.session.Room == a.firstRoom {
		chatWidth = chatWidth + onlineWidth // reclaim online sidebar space
		onlineWidth = 0
		feedWidth = chatWidth * 2 / 3
		if feedWidth < 30 {
			feedWidth = 30
		}
		if feedWidth > chatWidth-25 {
			feedWidth = chatWidth - 25
		}
		chatWidth = chatWidth - feedWidth
	}

	a.chatWidth = chatWidth

	topBarHeight := 3
	bottomBarHeight := 2
	mainHeight := a.height - topBarHeight - bottomBarHeight
	if mainHeight < 6 {
		mainHeight = 6
	}

	a.topBar.Width = a.width
	a.bottomBar.Width = a.width
	a.bottomBar.IsGallery = (a.roomTypes[a.session.Room] == "gallery")
	a.rooms.Width = roomsWidth
	a.rooms.Height = mainHeight
	a.online.Width = onlineWidth
	a.online.Height = mainHeight
	chatHeight := mainHeight
	if a.roomTypes[a.session.Room] == "wargame" {
		chatHeight -= 4 // wargame header takes 4 lines
	}
	a.chat.SetSize(chatWidth, chatHeight)
	a.gallery.SetSize(chatWidth, mainHeight)
	a.gallery.SetScreenOffset(roomsWidth, topBarHeight)
	if a.sudokuView != nil {
		a.sudokuView.SetSize(chatWidth, mainHeight)
	}
	if a.dmMode {
		a.dmInbox.SetSize(chatWidth, mainHeight)
		if a.dmInConvo {
			a.dmChatView.SetSize(chatWidth, mainHeight)
		}
	}
	if feedWidth > 0 {
		a.feed.SetSize(feedWidth, mainHeight)
	}
}

func (a App) View() tea.View {
	if a.state == stateSplash {
		v := a.splash.View()
		if a.modal == ModalChangelog {
			modalBox := a.changelogModal.View(a.width, a.height)
			content := v.Content
			content = Overlay(content, modalBox, a.width, a.height)
			v = tea.NewView(content)
			v.AltScreen = true
		}
		return v
	}

	if a.width == 0 {
		v := tea.NewView("Loading...")
		v.AltScreen = true
		return v
	}

	// Use cached values — refreshed on tick, never query DB in View()
	a.topBar.OnlineCount = a.hub.OnlineCount()
	a.topBar.WeeklyCount = a.cachedWeeklyCount
	a.topBar.AllTimeCount = a.cachedAllTimeCount

	a.online.Users = a.cachedOnlineNames
	a.online.Tankard = &a.tankard
	a.online.Leaderboard = a.cachedLeaderboard

	a.rooms.CurrentRoom = a.session.Room
	a.rooms.Rooms = a.cachedRoomInfos
	a.rooms.RoomTypes = a.roomTypes
	a.rooms.ActivityCounts = a.cachedActivityCounts
	a.rooms.SSHLinks = a.cachedSSHLinks

	mentionCounts := make(map[string]int)
	for _, m := range a.mentions {
		if !m.Read {
			mentionCounts[m.Room]++
		}
	}
	a.rooms.MentionCounts = mentionCounts
	a.rooms.DMUnread = a.dmUnreadCache
	a.bottomBar.DMUnread = a.dmUnreadCache

	a.bottomBar.MentionCount = a.unreadMentionCount("")
	a.bottomBar.IsTankard = a.tankardFocused
	a.bottomBar.IsDMMode = a.dmMode
	a.bottomBar.IsFeed = a.feedActive && a.session.Room == a.firstRoom && !a.dmMode

	topBar := a.topBar.View()
	bottomBar := a.bottomBar.View()

	// Main content: DM mode, gallery, sudoku, wargame, or chat
	var centerView string
	if a.dmMode {
		if a.dmInConvo {
			centerView = a.dmChatView.View()
		} else {
			centerView = a.dmInbox.View()
		}
	} else if a.roomTypes[a.session.Room] == "gallery" {
		centerView = a.gallery.View()
	} else if a.roomTypes[a.session.Room] == "games" && a.sudokuView != nil {
		centerView = a.sudokuView.View()
	} else if a.roomTypes[a.session.Room] == "wargame" && a.wargameStore != nil {
		// Wargame room: progress header + chat
		level := 0
		pts := 0
		progress := a.wargameStore.UserProgress(a.session.Fingerprint, a.wargameRoomNames()...)
		for _, p := range progress {
			if p.Wargame == a.session.Room {
				level = p.Level
				pts = p.Points
				break
			}
		}
		maxLevel := a.wargameStore.MaxLevel(a.session.Room)
		chatW := a.chat.viewport.Width()
		header := WargameHeader(a.session.Room, level, maxLevel, pts, chatW)
		centerView = header + a.chat.View()
	} else {
		if a.feedActive && a.session.Room == a.firstRoom {
			feedView := a.feed.View()
			chatView := a.chat.View()
			centerView = lipgloss.JoinHorizontal(lipgloss.Top, feedView, chatView)
		} else {
			centerView = a.chat.View()
		}
	}

	var mainArea string
	if a.rooms.Width > 0 && a.online.Width > 0 {
		roomsView := a.rooms.View()
		onlineView := a.online.View()
		mainArea = lipgloss.JoinHorizontal(lipgloss.Top, roomsView, centerView, onlineView)
	} else if a.rooms.Width > 0 {
		roomsView := a.rooms.View()
		mainArea = lipgloss.JoinHorizontal(lipgloss.Top, roomsView, centerView)
	} else {
		mainArea = centerView
	}

	base := lipgloss.JoinVertical(lipgloss.Left, topBar, mainArea, bottomBar)

	// If modal is open, overlay it on top of the dimmed base
	if a.modal != ModalNone {
		var modalBox string
		switch a.modal {
		case ModalHelp:
			modalBox = a.helpModal.View(a.width, a.height)
		case ModalNick:
			modalBox = a.nickModal.View(a.width, a.height)
		case ModalJoinRoom:
			modalBox = a.joinRoomModal.View(a.width, a.height)
		case ModalPost:
			modalBox = a.postModal.View(a.width, a.height)
		case ModalExpandNote:
			modalBox = a.expandNoteModal.View(a.width, a.height)
		case ModalMention:
			modalBox = a.mentionModal.View(a.width, a.height)
		case ModalPoll:
			modalBox = a.pollModal.View(a.width, a.height)
		case ModalPollVote:
			modalBox = a.pollVoteOverlay.View(a.width, a.height)
		case ModalGif:
			modalBox = a.gifModal.View(a.width, a.height)
		case ModalSubmitFlag:
			modalBox = a.submitModal.View(a.width, a.height)
		case ModalLeaderboard:
			modalBox = a.leaderboardModal.View(a.width, a.height)
		case ModalWargameRules:
			modalBox = a.wargameRulesModal.View(a.width, a.height)
		}
		base = Overlay(base, modalBox, a.width, a.height)
	}

	// During transition: spring-animated wipe from top to bottom
	if a.state == stateTransition {
		base = a.renderTransition(base)
	}

	v := tea.NewView(base)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = a.tavernDomain
	return v
}

func (a App) nextTickInterval() time.Duration {
	switch a.state {
	case stateSplash:
		return splashTickInterval
	case stateTransition:
		return transitionTickInterval
	}
	if a.chat.HasTypingUsers() {
		return typingTickInterval
	}
	if a.chat.HasActiveLogs() {
		return typingTickInterval
	}
	if a.chat.HasAnimatingGifs() {
		return 80 * time.Millisecond
	}
	return idleTickInterval
}

// renderTransition applies a spring-animated top-down wipe reveal.
// transPos goes 0→1 with spring bounce. Lines above the reveal line
// show normally, lines below are dimmed, the reveal line itself gets
// a gradient fade.
func (a App) renderTransition(content string) string {
	lines := strings.Split(content, "\n")
	total := len(lines)
	if total == 0 {
		return content
	}

	// How many lines to reveal (spring can overshoot past 1.0)
	revealFloat := a.transPos * float64(total)
	revealLine := int(revealFloat)
	if revealLine > total {
		revealLine = total
	}

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("236"))
	fadeColors := []color.Color{
		lipgloss.Color("240"),
		lipgloss.Color("238"),
		lipgloss.Color("236"),
	}

	var result []string
	for i, line := range lines {
		if i < revealLine {
			// Fully revealed
			result = append(result, line)
		} else if i < revealLine+len(fadeColors) {
			// Fade zone
			idx := i - revealLine
			stripped := stripAnsi(line)
			result = append(result, lipgloss.NewStyle().Foreground(fadeColors[idx]).Render(stripped))
		} else {
			// Hidden
			stripped := stripAnsi(line)
			result = append(result, dimStyle.Render(stripped))
		}
	}
	return strings.Join(result, "\n")
}
