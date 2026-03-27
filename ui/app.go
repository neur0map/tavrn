package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"tavrn/internal/admin"
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

	// Modal
	modal     ModalType
	helpModal HelpModal
	nickModal NickModal
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
		// Drive splash animations from the same tick
		if a.state == stateSplash {
			a.splash.frame++
			a.splash.tickSparks()
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
	}

	// Splash state — handle keys directly (tick/resize handled above)
	if a.state == stateSplash {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			switch keyMsg.String() {
			case "enter", "y":
				a.state = stateTavern
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

	// Tavern state
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
			// Typing notification (throttled)
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
	}
}

func (a *App) doLayout() {
	roomsWidth := 16
	onlineWidth := 18
	if a.width < 90 {
		roomsWidth = 0
		onlineWidth = 0
	} else if a.width < 110 {
		roomsWidth = 14
		onlineWidth = 16
	}
	chatWidth := a.width - roomsWidth - onlineWidth

	topBarHeight := 3
	bottomBarHeight := 2
	mainHeight := a.height - topBarHeight - bottomBarHeight
	if mainHeight < 6 {
		mainHeight = 6
	}

	a.topBar.Width = a.width
	a.bottomBar.Width = a.width
	a.rooms.Width = roomsWidth
	a.rooms.Height = mainHeight
	a.online.Width = onlineWidth
	a.online.Height = mainHeight
	a.chat.SetSize(chatWidth, mainHeight)
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

	// Always render the base view
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
	a.rooms.Rooms = []RoomInfo{{Name: "lounge", Count: a.hub.OnlineCount()}}

	topBar := a.topBar.View()
	chatView := a.chat.View()
	bottomBar := a.bottomBar.View()

	var mainArea string
	if a.rooms.Width > 0 {
		roomsView := a.rooms.View()
		onlineView := a.online.View()
		mainArea = lipgloss.JoinHorizontal(lipgloss.Top, roomsView, chatView, onlineView)
	} else {
		mainArea = chatView
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
		}
		base = Overlay(base, modalBox, a.width, a.height)
	}

	v := tea.NewView(base)
	v.AltScreen = true
	v.WindowTitle = TabTitle(a.topBar.Frame, a.hub.OnlineCount())
	return v
}
