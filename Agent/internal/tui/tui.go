package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Lyoomu/TAC/Agent/internal/agent"
	"github.com/Lyoomu/TAC/Agent/internal/component"
	"github.com/Lyoomu/TAC/Agent/internal/config"
	"github.com/Lyoomu/TAC/Agent/internal/models"
	"github.com/Lyoomu/TAC/Agent/internal/role"
	"github.com/Lyoomu/TAC/Agent/internal/server"
	"github.com/Lyoomu/TAC/Agent/internal/tool"
)

type AppContext struct {
	Config          *config.Config
	ComponentEngine *component.Engine
	ModelsEngine    *models.Engine
	RoleEngine      *role.Engine
	ToolEngine      *tool.Engine
	AgentManager    *agent.Manager
	Server          *server.Server

	CommandFunc func(cmd string) (string, error)
}

func Run(ctx *AppContext) error {
	m := newModel(ctx)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}
