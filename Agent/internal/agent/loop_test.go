package agent

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Lyoomu/TAC/Agent/internal/models/llm"
)

type mockStreamClient struct {
	streamCh chan string
	resultCh chan llm.AssistantMessage
	errCh    chan error
}

func (m *mockStreamClient) ChatStreamV2(messages []llm.Message, toolDefs []json.RawMessage) (<-chan string, <-chan llm.AssistantMessage, <-chan error) {
	return m.streamCh, m.resultCh, m.errCh
}

func TestAgentLoop_ClosesTurnEndChAfterEachTurn(t *testing.T) {
	streamCh := make(chan string)
	resultCh := make(chan llm.AssistantMessage)
	errCh := make(chan error)

	client := &mockStreamClient{
		streamCh: streamCh,
		resultCh: resultCh,
		errCh:    errCh,
	}

	ag := &Agent{
		ID:         "test-1",
		History:    []llm.Message{},
		Status:     StatusRunning,
		MaxHistory: DefaultMaxHistory,
		userMsgCh:  make(chan userMessage, 10),
		toolResCh:  make(chan toolResult),
		streamCh:   make(chan string),
		toolCallCh: make(chan ToolCall),
		errCh:      make(chan error, 10),
		doneCh:     make(chan struct{}),
		stopCh:     make(chan struct{}),
		turnEndCh:  make(chan struct{}),
	}

	mgr := &Manager{}
	go mgr.agentLoop(ag, client)

	// Drain agent output channels so processTurn does not block
	go func() {
		for range ag.streamCh {
		}
	}()
	go func() {
		for range ag.toolCallCh {
		}
	}()
	go func() {
		for range ag.errCh {
		}
	}()

	// First turn
	ag.SendMessage("Hello", ModeQueue)
	streamCh <- "Hi"
	close(streamCh)
	resultCh <- llm.AssistantMessage{Content: "Hi"}
	close(resultCh)
	close(errCh)

	select {
	case <-ag.turnEndCh:
		// OK, turnEndCh closed after first turn
	case <-time.After(2 * time.Second):
		t.Fatal("turnEndCh was not closed after first turn")
	}

	// Second turn
	streamCh2 := make(chan string)
	resultCh2 := make(chan llm.AssistantMessage)
	errCh2 := make(chan error)
	client.streamCh = streamCh2
	client.resultCh = resultCh2
	client.errCh = errCh2

	ag.SendMessage("How are you?", ModeQueue)
	streamCh2 <- "I'm fine"
	close(streamCh2)
	resultCh2 <- llm.AssistantMessage{Content: "I'm fine"}
	close(resultCh2)
	close(errCh2)

	select {
	case <-ag.turnEndCh:
		// OK, turnEndCh closed after second turn
	case <-time.After(2 * time.Second):
		t.Fatal("turnEndCh was not closed after second turn")
	}

	ag.Stop()
	select {
	case <-ag.doneCh:
		// OK
	case <-time.After(2 * time.Second):
		t.Fatal("doneCh was not closed after Stop")
	}
}
