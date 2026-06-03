package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	srv "github.com/Lyoomu/TAC/Trigger/internal/server"
	"github.com/Lyoomu/TAC/Trigger/internal/session"
	"github.com/Lyoomu/TAC/Trigger/internal/workspace"
)

type AppContext struct {
	ServerEngine    *srv.Engine
	SessionManager  *session.Manager
	WorkspaceEngine *workspace.Engine

	CommandFunc func(cmd string) (string, error)
}

func Run(ctx *AppContext) error {
	m := newModel(ctx)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}
