package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tabID int

const (
	tabDashboard tabID = iota
	tabWorkspace
	tabServers
	tabTriggers
	tabEvents
	tabEnvPresets
	tabDaemon
)

var tabNames = []string{"Dashboard", "Workspace", "Servers", "Triggers", "Events", "Env Presets", "Daemon"}

// Layout constants
const (
	tabBarHeight    = 2 // tab content + border-bottom
	statusBarHeight = 2 // border-top + status content
)

type viewMode int

const (
	viewMain viewMode = iota
	viewChat
)

type triggerModel struct {
	ctx       *AppContext
	activeTab tabID
	viewMode  viewMode
	width     int
	height    int

	// Computed content area dimensions
	contentWidth  int
	contentHeight int

	dashboard   *dashboardModel
	workspace   *workspaceViewModel
	serversTab  *serversViewModel
	triggersTab *triggersViewModel
	eventsTab   *eventsViewModel
	envTab      *envPresetsViewModel
	daemonTab   *daemonViewModel

	chatView *chatViewModel

	help *helpModel

	replInput textinput.Model
	replFocus bool

	statusMsg string
}

func newModel(ctx *AppContext) triggerModel {
	ti := textinput.New()
	ti.Placeholder = "Type command or press Tab/Shift+Tab to switch views..."
	ti.CharLimit = 256

	return triggerModel{
		ctx:         ctx,
		activeTab:   tabDashboard,
		viewMode:    viewMain,
		dashboard:   newDashboardModel(ctx),
		workspace:   newWorkspaceViewModel(ctx),
		serversTab:  newServersViewModel(ctx),
		triggersTab: newTriggersViewModel(ctx),
		eventsTab:   newEventsViewModel(ctx),
		envTab:      newEnvPresetsViewModel(ctx),
		daemonTab:   newDaemonViewModel(ctx),
		chatView:    newChatViewModel(ctx),
		help:        newHelpModel(),
		replInput:   ti,
	}
}

func (m *triggerModel) recalcLayout() {
	m.contentHeight = m.height - tabBarHeight - statusBarHeight
	if m.contentHeight < 1 {
		m.contentHeight = 1
	}
	m.contentWidth = m.width
	if m.contentWidth < 20 {
		m.contentWidth = 20
	}
}

func (m triggerModel) Init() tea.Cmd {
	return tea.Batch(
		m.dashboard.Init(),
		textinput.Blink,
	)
}

func (m triggerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcLayout()
		m.dashboard.setSize(m.contentWidth, m.contentHeight)
		m.workspace.setSize(m.contentWidth, m.contentHeight)
		m.serversTab.setSize(m.contentWidth, m.contentHeight)
		m.triggersTab.setSize(m.contentWidth, m.contentHeight)
		m.eventsTab.setSize(m.contentWidth, m.contentHeight)
		m.envTab.setSize(m.contentWidth, m.contentHeight)
		m.daemonTab.setSize(m.contentWidth, m.contentHeight)
		m.chatView.setSize(msg.Width, msg.Height)
		m.help.setSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:

		// When in chat mode, delegate ALL key events to chatView
		if m.viewMode == viewChat {
			newChat, cmd := m.chatView.Update(msg)
			m.chatView = newChat
			return m, cmd
		}

		// Help overlay intercepts keys
		if m.help.IsVisible() {
			switch msg.String() {
			case "?", "esc", "q":
				m.help.Hide()
			case "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}

		if !m.replFocus && m.isFormActive() {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			cmd := m.updateActiveTab(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "q":
			if m.replFocus {
				var cmd tea.Cmd
				m.replInput, cmd = m.replInput.Update(msg)
				cmds = append(cmds, cmd)
				return m, tea.Batch(cmds...)
			}
			return m, tea.Quit

		case "esc":
			if m.replFocus {
				m.replFocus = false
				m.replInput.Blur()
				return m, nil
			}

		case "?":
			if !m.replFocus {
				m.help.Toggle()
				return m, nil
			}

		case "tab":
			if !m.replFocus {
				m.activeTab = (m.activeTab + 1) % tabID(len(tabNames))
				return m, m.refreshActiveTab()
			}

		case "shift+tab":
			if !m.replFocus {
				m.activeTab = (m.activeTab - 1 + tabID(len(tabNames))) % tabID(len(tabNames))
				return m, m.refreshActiveTab()
			}

		case "/", ":":
			if !m.replFocus {
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

		if m.replFocus {
			var cmd tea.Cmd
			m.replInput, cmd = m.replInput.Update(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		cmd := m.updateActiveTab(msg)
		cmds = append(cmds, cmd)

	case triggerStatusMsg:
		m.statusMsg = string(msg)
		return m, nil

	case switchToChatMsg:
		m.viewMode = viewChat
		m.help.Hide()
		m.chatView.startChat(msg.roleKey, msg.serverAddr, msg.sessionID, msg.triggerName)
		return m, m.chatView.Init()

	case chatExitMsg:
		m.viewMode = viewMain
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

func (m triggerModel) View() string {
	if m.viewMode == viewChat {
		return m.chatView.View()
	}

	// Help overlay takes over entire screen
	if m.help.IsVisible() {
		return m.help.View()
	}

	tabBar := m.renderTabBar()
	content := padToHeight(m.renderActiveTab(), m.contentHeight)
	statusBar := m.renderStatusBar()

	return tabBar + "\n" + content + "\n" + statusBar
}

func (m triggerModel) renderTabBar() string {
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

func (m triggerModel) renderActiveTab() string {
	switch m.activeTab {
	case tabDashboard:
		return m.dashboard.View()
	case tabWorkspace:
		return m.workspace.View()
	case tabServers:
		return m.serversTab.View()
	case tabTriggers:
		return m.triggersTab.View()
	case tabEvents:
		return m.eventsTab.View()
	case tabEnvPresets:
		return m.envTab.View()
	case tabDaemon:
		return m.daemonTab.View()
	}
	return ""
}

func (m triggerModel) renderStatusBar() string {
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

func (m triggerModel) isFormActive() bool {
	switch m.activeTab {
	case tabWorkspace:
		return m.workspace.form.IsActive()
	case tabServers:
		return m.serversTab.form.IsActive() || m.serversTab.selector.IsActive()
	case tabTriggers:
		return m.triggersTab.form.IsActive()
	case tabEvents:
		return m.eventsTab.form.IsActive()
	case tabEnvPresets:
		return m.envTab.form.IsActive()
	}
	return false
}

func (m *triggerModel) updateActiveTab(msg tea.Msg) tea.Cmd {
	switch m.activeTab {
	case tabDashboard:
		return m.dashboard.Update(msg)
	case tabWorkspace:
		return m.workspace.Update(msg)
	case tabServers:
		return m.serversTab.Update(msg)
	case tabTriggers:
		return m.triggersTab.Update(msg)
	case tabEvents:
		return m.eventsTab.Update(msg)
	case tabEnvPresets:
		return m.envTab.Update(msg)
	case tabDaemon:
		return m.daemonTab.Update(msg)
	}
	return nil
}

func (m triggerModel) refreshActiveTab() tea.Cmd {
	switch m.activeTab {
	case tabDashboard:
		return m.dashboard.refresh()
	case tabWorkspace:
		return m.workspace.refresh()
	case tabServers:
		return m.serversTab.refresh()
	case tabTriggers:
		return m.triggersTab.refresh()
	case tabEvents:
		return m.eventsTab.refresh()
	case tabEnvPresets:
		return m.envTab.refresh()
	case tabDaemon:
		return m.daemonTab.refresh()
	}
	return nil
}

func (m triggerModel) executeCommand(cmd string) tea.Cmd {
	return func() tea.Msg {
		if cmd == "" {
			return nil
		}
		if m.ctx.CommandFunc != nil {
			out, err := m.ctx.CommandFunc(cmd)
			if err != nil {
				return triggerStatusMsg("Error: " + err.Error())
			}
			if out != "" {
				return triggerStatusMsg(out)
			}
		}
		return triggerStatusMsg("Executed: " + cmd)
	}
}

type triggerStatusMsg string

type switchToChatMsg struct {
	roleKey     string
	serverAddr  string
	sessionID   string
	triggerName string
}

type chatExitMsg struct{}
