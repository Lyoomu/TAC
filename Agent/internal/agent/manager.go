package agent

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Lyoomu/TAC/Agent/internal/config"
	"github.com/Lyoomu/TAC/Agent/internal/model"
	"github.com/Lyoomu/TAC/Agent/internal/models"
	"github.com/Lyoomu/TAC/Agent/internal/models/llm"
	"github.com/Lyoomu/TAC/Agent/internal/role"
	"github.com/Lyoomu/TAC/Agent/internal/tool"
)

type streamClient interface {
	ChatStreamV2(messages []llm.Message, toolDefs []json.RawMessage) (<-chan string, <-chan llm.AssistantMessage, <-chan error,
	)
}

type Manager struct {
	mu           sync.RWMutex
	roleEngine   *role.Engine
	toolEngine   *tool.Engine
	modelsEngine *models.Engine
	config       *config.Config
}

func NewManager(
	roleEngine *role.Engine,
	toolEngine *tool.Engine,
	modelsEngine *models.Engine,
	cfg *config.Config,
) *Manager {
	return &Manager{
		roleEngine:   roleEngine,
		toolEngine:   toolEngine,
		modelsEngine: modelsEngine,
		config:       cfg,
	}
}

func (m *Manager) createClient(role *model.Role) (streamClient, error) {
	if role.Model == "" {
		return nil, fmt.Errorf("role '%s' has no model bound", role.Name)
	}
	modelCfg, err := m.modelsEngine.GetSecure(role.Model)
	if err != nil {
		return nil, fmt.Errorf("get model '%s' for role '%s': %w", role.Model, role.Name, err)
	}

	apiType := role.APIType
	if apiType == "" {
		apiType = modelCfg.APIType
	}

	switch apiType {
	case model.APITypeResponses:
		return llm.NewResponsesClient(modelCfg, m.config), nil
	case model.APITypeAnthropic:
		return llm.NewAnthropicClient(modelCfg, m.config), nil
	default:
		return llm.NewClient(modelCfg, m.config), nil
	}
}

func (m *Manager) RunAgent(
	roleName string,
	env map[string]string,
	message string,
	session []llm.Message,
	defaultMode MessageMode,
) (*AgentOutput, error) {
	role, err := m.roleEngine.Get(roleName)
	if err != nil {
		return nil, err
	}

	mergedEnv := make(map[string]string)
	for k, v := range env {
		mergedEnv[k] = v
	}
	for k, v := range role.Env {
		mergedEnv[k] = v
	}

	prompts, err := m.roleEngine.AssemblePrompts(roleName)
	if err != nil {
		return nil, err
	}

	var toolDefs []json.RawMessage
	for _, toolName := range role.Tools {
		info, err := m.toolEngine.GetInfo(toolName)
		if err != nil {
			continue
		}
		toolDefs = append(toolDefs, info)
	}

	client, err := m.createClient(role)
	if err != nil {
		return nil, err
	}

	id := generateID()
	agent := &Agent{
		ID:         id,
		Role:       role,
		Env:        mergedEnv,
		History:    make([]llm.Message, 0),
		Tools:      toolDefs,
		Status:     StatusRunning,
		MaxHistory: DefaultMaxHistory,

		userMsgCh:  make(chan userMessage, 10),
		toolResCh:  make(chan toolResult),
		streamCh:   make(chan string),
		toolCallCh: make(chan ToolCall),
		errCh:      make(chan error, 10),
		doneCh:     make(chan struct{}),
		stopCh:     make(chan struct{}),
		turnEndCh:  make(chan struct{}, 1),
	}

	agent.History = append(agent.History, session...)
	for _, prompt := range prompts {
		agent.History = append(agent.History, llm.NewTextMessage("system", prompt))
	}

	go m.agentLoop(agent, client)

	if message != "" {
		agent.SendMessage(message, defaultMode)
	}

	return &AgentOutput{
		ID:         id,
		StreamCh:   agent.streamCh,
		ToolCallCh: agent.toolCallCh,
		ErrCh:      agent.errCh,
		DoneCh:     agent.doneCh,
		TurnEndCh:  agent.turnEndCh,
		agent:      agent,
	}, nil
}
