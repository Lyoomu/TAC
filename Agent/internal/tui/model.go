package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tabID int

const (
	tabDashboard tabID = iota
	tabModels
	tabRoles
	tabComponents
	tabTools
	tabServer
)

var tabNames = []string{"Dashboard", "Models", "Roles", "Components", "Tools", "Server"}

type viewMode int

const (
	viewDashboard viewMode = iota
	viewChat
)

type model struct {
	ctx       *AppContext
	activeTab tabID
	viewMode  viewMode
	width     int
	height    int

	dashboard     *dashboardModel
	modelsTab     *modelsViewModel
	rolesTab      *rolesViewModel
	componentsTab *componentsViewModel
	toolsTab      *toolsViewModel
	serverTab     *serverViewModel

	chatView *chatViewModel

	help *helpModel

	replInput textinput.Model
	replFocus bool

	statusMsg string
}

func newModel(ctx *AppContext) model {
	ti := textinput.New()
	ti.Placeholder = "Type command (e.g. 'model list') or press Tab/Shift+Tab to switch views..."
	ti.CharLimit = 256

	return model{
		ctx:           ctx,
		activeTab:     tabDashboard,
		viewMode:      viewDashboard,
		dashboard:     newDashboardModel(ctx),
		modelsTab:     newModelsViewModel(ctx),
		rolesTab:      newRolesViewModel(ctx),
		componentsTab: newComponentsViewModel(ctx),
		toolsTab:      newToolsViewModel(ctx),
		serverTab:     newServerViewModel(ctx),
		chatView:      newChatViewModel(ctx),
		help:          newHelpModel(),
		replInput:     ti,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.dashboard.Init(),
		textinput.Blink,
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.dashboard.setSize(msg.Width, msg.Height-4)
		m.modelsTab.setSize(msg.Width, msg.Height-4)
		m.rolesTab.setSize(msg.Width, msg.Height-4)
		m.componentsTab.setSize(msg.Width, msg.Height-4)
		m.toolsTab.setSize(msg.Width, msg.Height-4)
		m.serverTab.setSize(msg.Width, msg.Height-4)
		m.chatView.setSize(msg.Width, msg.Height)
		m.help.setSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:

		if m.viewMode == viewDashboard && !m.replFocus && m.isFormActive() {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			cmd := m.updateActiveTab(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "ctrl+c", "q":
			if m.viewMode == viewChat {
				m.viewMode = viewDashboard
				return m, nil
			}
			if m.replFocus {
				m.replFocus = false
				m.replInput.Blur()
				return m, nil
			}
			return m, tea.Quit

		case "esc":
			if m.viewMode == viewChat {
				m.viewMode = viewDashboard
				return m, nil
			}
			if m.replFocus {
				m.replFocus = false
				m.replInput.Blur()
				return m, nil
			}

		case "?":
			if m.viewMode == viewDashboard && !m.replFocus {
				m.help.Toggle()
				return m, nil
			}

		case "tab":
			if !m.replFocus && m.viewMode == viewDashboard && !m.help.IsVisible() {
				m.activeTab = (m.activeTab + 1) % tabID(len(tabNames))
				return m, m.refreshActiveTab()
			}

		case "shift+tab":
			if !m.replFocus && m.viewMode == viewDashboard && !m.help.IsVisible() {
				m.activeTab = (m.activeTab - 1 + tabID(len(tabNames))) % tabID(len(tabNames))
				return m, m.refreshActiveTab()
			}

		case "/", ":":
			if !m.replFocus && m.viewMode == viewDashboard && !m.help.IsVisible() {
				m.replFocus = true
				m.replInput.Focus()
				return m, textinput.Blink
			}

		case "enter":
			if m.replFocus {
				cmd := m.replInput.Value()
				m.replInput.SetValue("")
				m.replFocus = false
				m.replInput.Blur()
				return m, m.executeCommand(cmd)
			}
		}

		if m.viewMode == viewChat {
			newChat, cmd := m.chatView.Update(msg)
			m.chatView = newChat
			return m, cmd
		}

		if m.replFocus {
			var cmd tea.Cmd
			m.replInput, cmd = m.replInput.Update(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		cmd := m.updateActiveTab(msg)
		cmds = append(cmds, cmd)

	case statusMsg:
		m.statusMsg = string(msg)
		return m, nil

	case switchToChatMsg:
		m.viewMode = viewChat
		m.help.Hide()
		m.chatView.startChat(msg.roleName)
		return m, m.chatView.Init()

	case chatExitMsg:
		m.viewMode = viewDashboard
		return m, nil

	default:

		if m.viewMode == viewChat {
			newChat, cmd := m.chatView.Update(msg)
			m.chatView = newChat
			return m, cmd
		}
		cmd := m.updateActiveTab(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.viewMode == viewChat {
		return m.chatView.View()
	}

	var b strings.Builder

	b.WriteString(m.renderTabBar())
	b.WriteString("\n")

	content := m.renderActiveTab()
	b.WriteString(content)

	b.WriteString("\n")
	b.WriteString(m.renderStatusBar())

	if m.help.IsVisible() {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			m.help.View(), lipgloss.WithWhitespaceChars(" "))
	}

	return b.String()
}

func (m model) renderTabBar() string {
	var tabs []string
	for i, name := range tabNames {
		if tabID(i) == m.activeTab {
			tabs = append(tabs, activeTabStyle.Render(name))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(name))
		}
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	return tabBarStyle.Width(m.width).Render(row)
}

func (m model) renderActiveTab() string {
	switch m.activeTab {
	case tabDashboard:
		return m.dashboard.View()
	case tabModels:
		return m.modelsTab.View()
	case tabRoles:
		return m.rolesTab.View()
	case tabComponents:
		return m.componentsTab.View()
	case tabTools:
		return m.toolsTab.View()
	case tabServer:
		return m.serverTab.View()
	}
	return ""
}

func (m model) renderStatusBar() string {
	left := ""
	if m.replFocus {
		left = m.replInput.View()
	} else if m.statusMsg != "" {
		left = statusMsgStyle.Render(m.statusMsg)
	} else {
		left = hintStyle.Render("[/] Command  [Tab] Switch  [?] Help  [Q] Quit  [Enter] Select")
	}

	return statusBarStyle.Width(m.width).Render(left)
}

func (m model) isFormActive() bool {
	switch m.activeTab {
	case tabModels:
		return m.modelsTab.form.IsActive() || m.modelsTab.selector.IsActive()
	case tabRoles:
		return m.rolesTab.form.IsActive() || m.rolesTab.selector.IsActive()
	case tabComponents:
		return m.componentsTab.form.IsActive() || m.componentsTab.selector.IsActive()
	case tabTools:
		return m.toolsTab.selector.IsActive()
	}
	return false
}

func (m *model) updateActiveTab(msg tea.Msg) tea.Cmd {
	switch m.activeTab {
	case tabDashboard:
		return m.dashboard.Update(msg)
	case tabModels:
		return m.modelsTab.Update(msg)
	case tabRoles:
		return m.rolesTab.Update(msg)
	case tabComponents:
		return m.componentsTab.Update(msg)
	case tabTools:
		return m.toolsTab.Update(msg)
	case tabServer:
		return m.serverTab.Update(msg)
	}
	return nil
}

func (m model) refreshActiveTab() tea.Cmd {
	switch m.activeTab {
	case tabDashboard:
		return m.dashboard.refresh()
	case tabModels:
		return m.modelsTab.refresh()
	case tabRoles:
		return m.rolesTab.refresh()
	case tabComponents:
		return m.componentsTab.refresh()
	case tabTools:
		return m.toolsTab.refresh()
	case tabServer:
		return m.serverTab.refresh()
	}
	return nil
}

func (m model) executeCommand(cmd string) tea.Cmd {
	return func() tea.Msg {
		if cmd == "" {
			return nil
		}
		if m.ctx.CommandFunc != nil {
			out, err := m.ctx.CommandFunc(cmd)
			if err != nil {
				return statusMsg("Error: " + err.Error())
			}
			if out != "" {
				return statusMsg(out)
			}
		}
		return statusMsg("Executed: " + cmd)
	}
}

type statusMsg string

type switchToChatMsg struct {
	roleName string
}

type chatExitMsg struct{}
