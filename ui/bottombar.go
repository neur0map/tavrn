package ui

type BottomBar struct {
	Width int
}

func NewBottomBar() BottomBar {
	return BottomBar{}
}

func (b BottomBar) View() string {
	content := " /help * CTRL+C: exit * ENTER: send"
	return BottomBarStyle.Width(b.Width).MaxWidth(b.Width).Render(content)
}
