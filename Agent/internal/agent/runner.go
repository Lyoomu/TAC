package agent

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"sync"

	"github.com/Lyoomu/TAC/Agent/internal/model"
	"github.com/Lyoomu/TAC/Agent/internal/models/llm"
)

type MessageMode string

const (
	ModeQueue     MessageMode = "queue"
	ModeReject    MessageMode = "reject"
	ModeInterrupt MessageMode = "interrupt"
)

type Status string

const (
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
)

type ToolCall struct {
	ID   string
	Name string
	Args string
}

type AgentOutput struct {
	ID         string
	StreamCh   <-chan string
	ToolCallCh <-chan ToolCall
	ErrCh      <-chan error
	DoneCh     <-chan struct{}
	TurnEndCh  <-chan struct{}

	agent *Agent
}

func (o *AgentOutput) SendMessage(msg string, mode MessageMode) {
	o.agent.SendMessage(msg, mode)
}

func (o *AgentOutput) SubmitToolResult(toolCallID string, result string) {
	o.agent.SubmitToolResult(toolCallID, result)
}

func (o *AgentOutput) Stop() {
	o.agent.Stop()
}

type userMessage struct {
	content string
	mode    MessageMode
}

type toolResult struct {
	ToolCallID string
	Content    string
}

const DefaultMaxHistory = 50

type Agent struct {
	ID         string
	Role       *model.Role
	Env        map[string]string
	History    []llm.Message
	Tools      []json.RawMessage
	Status     Status
	MaxHistory int

	userMsgCh  chan userMessage
	toolResCh  chan toolResult
	streamCh   chan string
	toolCallCh chan ToolCall
	errCh      chan error
	doneCh     chan struct{}
	stopCh     chan struct{}
	turnEndCh  chan struct{}

	mu sync.RWMutex
}

func (a *Agent) SendMessage(msg string, mode MessageMode) {
	select {
	case a.userMsgCh <- userMessage{content: msg, mode: mode}:
	case <-a.doneCh:
	}
}

func (a *Agent) SubmitToolResult(toolCallID string, result string) {
	select {
	case a.toolResCh <- toolResult{ToolCallID: toolCallID, Content: result}:
	case <-a.doneCh:
	}
}

func (a *Agent) Stop() {
	select {
	case <-a.stopCh:
	default:
		close(a.stopCh)
	}
}

func allNil(streamCh <-chan string, resultCh <-chan llm.AssistantMessage, errCh <-chan error) bool {
	return streamCh == nil && resultCh == nil && errCh == nil
}

func drain(streamCh <-chan string, resultCh <-chan llm.AssistantMessage) {
	for range streamCh {
	}
	for range resultCh {
	}
}

func generateID() string {
	b := make([]byte, 8)
	io.ReadFull(rand.Reader, b)
	return hex.EncodeToString(b)
}
