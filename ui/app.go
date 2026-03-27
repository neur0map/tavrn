package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"tavrn/internal/admin"
	"tavrn/internal/chat"
	"tavrn/internal/hub"
	"tavrn/internal/sanitize"
	"tavrn/internal/session"
	"tavrn/internal/store"
)

// HubMsg wraps a session.Msg for the Bubble Tea message loop.
type HubMsg session.Msg

type appState int

const (
	stateSplash appState = iota
	stateTavern
)

type App struct {
	state     appState
	splash    Splash
	session   *session.Session
	chat      ChatView
	topBar    TopBar
	bottomBar BottomBar
	sidebar   Sidebar
	width     int
	height    int
	store     *store.Store
	hub       *hub.Hub
	admin     *admin.Admin
	onSend    func(session.Msg)
}

func NewApp(sess *session.Session, st *store.Store, h *hub.Hub, adm *admin.Admin, onSend func(session.Msg)) App {
	return App{
		state:     stateSplash,
		splash:    NewSplash(sess.Nickname, sess.Fingerprint, sess.Flair),
		session:   sess,
		chat:      NewChatView(),
		topBar:    NewTopBar(),
		bottomBar: NewBottomBar(),
		sidebar:   NewSidebar(),
		store:     st,
		hub:       h,
		admin:     adm,
		onSend:    onSend,
	}
}

// WaitForHubMsg returns a tea.Cmd that waits for the next hub message.
func WaitForHubMsg(ch <-chan session.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return tea.Quit()
		}
		return HubMsg(msg)
	}
}

func (a App) Init() tea.Cmd {
	return WaitForHubMsg(a.session.Send)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		if a.state == stateSplash {
			a.splash.width = msg.Width
			a.splash.height = msg.Height
		} else {
			a.doLayout()
		}
		return a, nil

	case EnterTavernMsg:
		a.state = stateTavern
		a.doLayout()
		return a, WaitForHubMsg(a.session.Send)

	case HubMsg:
		a.handleHubMsg(session.Msg(msg))
		return a, WaitForHubMsg(a.session.Send)
	}

	if a.state == stateSplash {
		var cmd tea.Cmd
		splash, cmd := a.splash.Update(msg)
		a.splash = splash.(Splash)
		return a, cmd
	}

	// Tavern state
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "enter":
			return a.handleInput()
		}
	}

	var cmd tea.Cmd
	a.chat, cmd = a.chat.Update(msg)
	return a, cmd
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
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Slow down! You're sending too fast."))
		}
	}

	return a, nil
}

func (a *App) handleCommand(parsed chat.ParseResult) {
	switch parsed.Command {
	case "help":
		help := "Commands:\n" +
			"  /nick <name>  - change your nickname\n" +
			"  /who          - see who's around\n" +
			"  /help         - show this help\n\n" +
			"All data - nicknames, canvas, chat, identities, votes -\n" +
			"is purged every Sunday at 23:59 UTC.\n" +
			"Nothing is permanent. Draw while you can."
		a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, help))

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

	case "nick":
		if parsed.Args == "" {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Usage: /nick <name>"))
			return
		}
		cleaned, err := sanitize.CleanNick(parsed.Args)
		if err != nil {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, err.Error()))
			return
		}
		if err := a.store.SetNickname(a.session.Fingerprint, cleaned); err != nil {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "That name is already claimed."))
			return
		}
		oldNick := a.session.Nickname
		a.session.Nickname = cleaned
		a.onSend(session.Msg{
			Type: session.MsgSystem,
			Text: fmt.Sprintf("%s is now known as %s", oldNick, cleaned),
			Room: a.session.Room,
		})

	case "ban", "unban", "purge":
		if !a.session.IsAdmin {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Unknown command: /"+parsed.Command))
			return
		}
		result, err := a.admin.HandleCommand(a.session.Fingerprint, parsed.Command, parsed.Args)
		if err != nil {
			a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Error: "+err.Error()))
			return
		}
		a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, result))
		if parsed.Command == "purge" {
			a.onSend(session.Msg{Type: session.MsgPurge, Room: a.session.Room})
		}

	default:
		a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "Unknown command: /"+parsed.Command))
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
		})
	case session.MsgSystem, session.MsgUserJoined, session.MsgUserLeft:
		a.chat.AddMessage(chat.NewSystemMessage(msg.Room, msg.Text))
	case session.MsgPurge:
		a.chat.AddMessage(chat.NewSystemMessage(a.session.Room, "The tavern has been swept clean."))
	}
}

func (a *App) doLayout() {
	sidebarWidth := 24
	if a.width < 70 {
		sidebarWidth = 0
	}
	mainWidth := a.width - sidebarWidth

	topBarHeight := 1
	bottomBarHeight := 1
	mainHeight := a.height - topBarHeight - bottomBarHeight
	if mainHeight < 4 {
		mainHeight = 4
	}
	chatHeight := mainHeight

	a.topBar.Width = a.width
	a.bottomBar.Width = a.width
	a.sidebar.Width = sidebarWidth
	a.sidebar.Height = mainHeight
	a.chat.SetSize(mainWidth, chatHeight)
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

	// Update live counts
	a.topBar.OnlineCount = a.hub.OnlineCount()
	wc, _ := a.store.WeeklyVisitorCount()
	a.topBar.WeeklyCount = wc

	// Update sidebar
	sessions := a.hub.Sessions(a.session.Room)
	var onlineNames []string
	for _, s := range sessions {
		name := s.Nickname
		if s.Flair {
			name = "~" + name
		}
		onlineNames = append(onlineNames, name)
	}
	a.sidebar.OnlineUsers = onlineNames
	a.sidebar.Rooms = []RoomInfo{{Name: "lounge", Count: a.hub.OnlineCount()}}

	sidebarWidth := 24
	if a.width < 70 {
		sidebarWidth = 0
	}

	topBar := a.topBar.View()
	chatView := a.chat.View()

	var mainArea string
	if sidebarWidth > 0 {
		sidebar := a.sidebar.View()
		mainArea = lipgloss.JoinHorizontal(lipgloss.Top, chatView, sidebar)
	} else {
		mainArea = chatView
	}

	bottomBar := a.bottomBar.View()

	content := lipgloss.JoinVertical(lipgloss.Left, topBar, mainArea, bottomBar)
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}
