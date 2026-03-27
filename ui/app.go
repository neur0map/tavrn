package ui

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/harmonica"
	"tavrn/internal/admin"
	"tavrn/internal/room"
	"tavrn/internal/chat"
	"tavrn/internal/hub"
	"tavrn/internal/sanitize"
	"tavrn/internal/session"
	"tavrn/internal/store"
)

type HubMsg session.Msg
type tickMsg time.Time

type appState int

const (
	stateSplash appState = iota
	stateTransition
	stateTavern
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
	store          *store.Store
	hub            *hub.Hub
	admin          *admin.Admin
	onSend         func(session.Msg)
	lastTypingSent time.Time

	// Gallery
	gallery GalleryView

	// Modal
	modal         ModalType
	helpModal     HelpModal
	nickModal     NickModal
	joinRoomModal JoinRoomModal
	postModal     PostModal

	// Transition animation
	transSpring harmonica.Spring
	transPos    float64 // 0.0 = fully hidden, 1.0 = fully revealed
	transVel    float64
}

func NewApp(sess *session.Session, st *store.Store, h *hub.Hub, adm *admin.Admin, onSend func(session.Msg)) App {
	return App{
		state:     stateSplash,
		splash:    NewSplash(sess.Nickname, sess.Fingerprint, sess.Flair),
		session:   sess,
		chat:      NewChatView(),
		topBar:    NewTopBar(),
		bottomBar: NewBottomBar(),
		rooms:     NewRoomsPanel(),
		online:    NewOnlinePanel(),
		gallery:   NewGalleryView(sess.Fingerprint),
		store:     st,
		hub:       h,
		admin:     adm,
		onSend:    onSend,
		modal:     ModalNone,
	}
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

func doTick() tea.Cmd {
	return tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (a App) Init() tea.Cmd {
	return tea.Batch(WaitForHubMsg(a.session.Send), doTick())
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
		a.topBar.Frame++
		a.online.Frame++
		a.chat.Tick()
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
		return a, doTick()

	case HubMsg:
		a.handleHubMsg(session.Msg(msg))
		return a, WaitForHubMsg(a.session.Send)

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
		if a.session.Room != "gallery" {
			// Auto-join gallery
			a.switchRoom("gallery")
		}
		text := msg.Text
		if len(text) > 60 {
			text = text[:60]
		}
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
			Room: "gallery",
			Note: &session.NoteData{
				ID: noteID, X: x, Y: y,
				Text: text, Nick: a.session.Nickname, Color: a.session.ColorIndex,
			},
			Fingerprint: a.session.Fingerprint,
		})
		return a, nil

	case GalleryDeleteMsg:
		a.store.DeleteNote(msg.NoteID, a.session.Fingerprint)
		a.gallery.RemoveNote(msg.NoteID)
		a.onSend(session.Msg{
			Type: session.MsgNoteDelete,
			Room: "gallery",
			Note: &session.NoteData{ID: msg.NoteID},
		})
		return a, nil

	case GalleryMoveMsg:
		a.store.MoveNote(msg.NoteID, msg.X, msg.Y, a.session.Fingerprint)
		a.onSend(session.Msg{
			Type: session.MsgNoteMove,
			Room: "gallery",
			Note: &session.NoteData{ID: msg.NoteID, X: msg.X, Y: msg.Y},
		})
		return a, nil
	}

	// Splash state — handle keys directly (tick/resize handled above)
	if a.state == stateSplash {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			switch keyMsg.String() {
			case "enter", "y":
				a.state = stateTransition
				a.transSpring = harmonica.NewSpring(harmonica.FPS(30), 6.0, 0.8)
				a.transPos = 0.0
				a.transVel = 0.0
				a.doLayout()
				a.chat.AddMessage(chat.NewSystemMessage(a.session.Room,
					"Welcome to the tavern. Type /help for commands."))
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
		case "f1", "?":
			if a.session.Room == "gallery" || !a.chat.HasInput() {
				a.modal = ModalHelp
				a.helpModal = NewHelpModal()
				return a, nil
			}
		case "f2":
			a.modal = ModalNick
			a.nickModal = NewNickModal(a.session.Nickname)
			return a, nil
		case "f3":
			var counts []int
			for _, rName := range room.All {
				counts = append(counts, len(a.hub.Sessions(rName)))
			}
			a.modal = ModalJoinRoom
			a.joinRoomModal = NewJoinRoomModal(room.All, counts, a.session.Room)
			return a, nil
		case "f4":
			a.modal = ModalPost
			a.postModal = NewPostModal()
			return a, nil
		}
	}

	// Gallery room: single-key shortcuts + mouse
	if a.session.Room == "gallery" {
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
				var counts []int
				for _, rName := range room.All {
					counts = append(counts, len(a.hub.Sessions(rName)))
				}
				a.modal = ModalJoinRoom
				a.joinRoomModal = NewJoinRoomModal(room.All, counts, a.session.Room)
				return a, nil
			case "h":
				a.modal = ModalHelp
				a.helpModal = NewHelpModal()
				return a, nil
			}
			// Forward d/delete/tab to gallery
			if msg.String() == "d" || msg.String() == "delete" || msg.String() == "backspace" || msg.String() == "tab" {
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

	// Normal chat rooms
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "enter":
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
	case ModalHelp:
		// Help modal only responds to ESC (handled above)
		return a, nil
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
	a.modal = ModalNone
	a.onSend(session.Msg{
		Type: session.MsgSystem,
		Text: fmt.Sprintf("%s is now known as %s", oldNick, cleaned),
		Room: a.session.Room,
	})
	return a, nil
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
		a.handleCommand(parsed)
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

func (a *App) handleCommand(parsed chat.ParseResult) {
	switch parsed.Command {
	case "help":
		a.modal = ModalHelp
		a.helpModal = NewHelpModal()

	case "nick":
		a.modal = ModalNick
		a.nickModal = NewNickModal(a.session.Nickname)

	case "who":
		sessions := a.hub.Sessions(a.session.Room)
		var names []string
		for _, s := range sessions {
			name := s.Nickname
			if s.Flair {
				name = "~" + name
			}
			names = append(names, name)
		}
		text := fmt.Sprintf("In #%s: %s", a.session.Room, strings.Join(names, ", "))
		a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, text))

	case "join":
		// If arg provided, join directly
		target := strings.TrimPrefix(strings.TrimSpace(parsed.Args), "#")
		if target != "" {
			if !room.IsValid(target) {
				a.chat.AddMessage(chat.NewSystemMessage(a.session.Room,
					fmt.Sprintf("Unknown room. Available: %s", strings.Join(room.All, ", "))))
				return
			}
			if target != a.session.Room {
				a.switchRoom(target)
			}
			return
		}
		// No arg: open room picker modal
		var counts []int
		for _, rName := range room.All {
			counts = append(counts, len(a.hub.Sessions(rName)))
		}
		a.modal = ModalJoinRoom
		a.joinRoomModal = NewJoinRoomModal(room.All, counts, a.session.Room)

	case "post":
		if a.session.Room != "gallery" {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room,
				"You can only /post in #gallery. Use /join gallery first."))
			return
		}
		text := strings.TrimSpace(parsed.Args)
		if text == "" {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Usage: /post <message>"))
			return
		}
		if len(text) > 60 {
			text = text[:60]
		}
		x, y := a.gallery.RandomPosition()
		noteID, err := a.store.CreateNote(x, y, text, a.session.Fingerprint, a.session.Nickname, a.session.ColorIndex)
		if err != nil {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Error creating note."))
			return
		}
		note := GalleryNote{
			ID: noteID, X: x, Y: y,
			Text: text, Nickname: a.session.Nickname,
			Fingerprint: a.session.Fingerprint, ColorIndex: a.session.ColorIndex,
		}
		a.gallery.AddNote(note)
		a.onSend(session.Msg{
			Type: session.MsgNoteCreate,
			Room: "gallery",
			Note: &session.NoteData{
				ID: noteID, X: x, Y: y,
				Text: text, Nick: a.session.Nickname, Color: a.session.ColorIndex,
			},
			Fingerprint: a.session.Fingerprint,
		})

	case "ban", "unban", "purge":
		if !a.session.IsAdmin {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room,
				"Unknown command: /"+parsed.Command))
			return
		}
		result, err := a.admin.HandleCommand(
			a.session.Fingerprint, parsed.Command, parsed.Args)
		if err != nil {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room,
				"Error: "+err.Error()))
			return
		}
		a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, result))
		if parsed.Command == "purge" {
			a.onSend(session.Msg{Type: session.MsgPurge, Room: a.session.Room})
		}

	default:
		a.chat.AddMessage(chat.NewSystemMessage(a.session.Room,
			"Unknown command: /"+parsed.Command))
	}
}

func (a *App) handleHubMsg(msg session.Msg) {
	switch msg.Type {
	case session.MsgChat:
		a.chat.AddMessage(chat.Message{
			Nickname:   msg.Nickname,
			ColorIndex: msg.ColorIndex,
			Text:       msg.Text,
			Room:       msg.Room,
			Timestamp:  time.Now(),
		})
	case session.MsgSystem, session.MsgUserJoined, session.MsgUserLeft:
		a.chat.AddMessage(chat.NewSystemMessage(msg.Room, msg.Text))
	case session.MsgPurge:
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
	a.doLayout()

	if target == "gallery" {
		// Load gallery notes
		notes, _ := a.store.AllNotes()
		a.gallery = NewGalleryView(a.session.Fingerprint)
		a.gallery.SetSize(a.width, a.height-5) // account for top/bottom bars
		a.gallery.LoadNotes(notes)
	} else {
		// Load chat history
		history, _ := a.store.RecentMessages(target, 50)
		for _, m := range history {
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
	}

	// Announce join in new room
	a.onSend(session.Msg{
		Type: session.MsgUserJoined,
		Text: fmt.Sprintf("%s joined from #%s", a.session.Nickname, oldRoom),
		Room: target,
	})

	// Update top bar
	a.topBar.Room = target
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

	topBarHeight := 3
	bottomBarHeight := 2
	mainHeight := a.height - topBarHeight - bottomBarHeight
	if mainHeight < 6 {
		mainHeight = 6
	}

	a.topBar.Width = a.width
	a.bottomBar.Width = a.width
	a.bottomBar.IsGallery = (a.session.Room == "gallery")
	a.rooms.Width = roomsWidth
	a.rooms.Height = mainHeight
	a.online.Width = onlineWidth
	a.online.Height = mainHeight
	a.chat.SetSize(chatWidth, mainHeight)
	a.gallery.SetSize(chatWidth, mainHeight)
}

func (a App) View() tea.View {
	if a.state == stateSplash {
		return a.splash.View()
	}

	if a.width == 0 {
		v := tea.NewView("Loading...")
		v.AltScreen = true
		return v
	}

	// Render the tavern view (used for both transition and normal)
	a.topBar.OnlineCount = a.hub.OnlineCount()
	wc, _ := a.store.WeeklyVisitorCount()
	a.topBar.WeeklyCount = wc

	sessions := a.hub.Sessions(a.session.Room)
	var onlineNames []string
	for _, s := range sessions {
		name := s.Nickname
		if s.Flair {
			name = "~" + name
		}
		onlineNames = append(onlineNames, name)
	}
	a.online.Users = onlineNames
	a.rooms.CurrentRoom = a.session.Room
	var roomInfos []RoomInfo
	for _, rName := range room.All {
		roomInfos = append(roomInfos, RoomInfo{
			Name:  rName,
			Count: len(a.hub.Sessions(rName)),
		})
	}
	a.rooms.Rooms = roomInfos

	topBar := a.topBar.View()
	bottomBar := a.bottomBar.View()

	// Main content: gallery board or chat depending on room
	var centerView string
	if a.session.Room == "gallery" {
		centerView = a.gallery.View()
	} else {
		centerView = a.chat.View()
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
	v.WindowTitle = "tavrn.sh"
	return v
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
