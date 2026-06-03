package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Lyoomu/TAC/Agent/internal/agent"
	pb "github.com/Lyoomu/TAC/proto"
	"google.golang.org/grpc/metadata"
)

type mockStream struct {
	mu     sync.Mutex
	sent   []*pb.ChatMessage
	recvCh chan *pb.ChatMessage
	ctx    context.Context
	cancel context.CancelFunc
	closed bool
}

func newMockStream() *mockStream {
	ctx, cancel := context.WithCancel(context.Background())
	return &mockStream{
		recvCh: make(chan *pb.ChatMessage, 10),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (m *mockStream) Context() context.Context        { return m.ctx }
func (m *mockStream) SendHeader(md metadata.MD) error { return nil }
func (m *mockStream) SetHeader(md metadata.MD) error  { return nil }
func (m *mockStream) SetTrailer(md metadata.MD)       {}

func (m *mockStream) SendMsg(msg any) error {
	return m.Send(msg.(*pb.ChatMessage))
}

func (m *mockStream) RecvMsg(msg any) error {
	_, err := m.Recv()
	return err
}

func (m *mockStream) Send(msg *pb.ChatMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return context.Canceled
	}
	m.sent = append(m.sent, msg)
	return nil
}

func (m *mockStream) Recv() (*pb.ChatMessage, error) {
	select {
	case msg := <-m.recvCh:
		return msg, nil
	case <-m.ctx.Done():
		return nil, m.ctx.Err()
	}
}

func (m *mockStream) close() {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	m.cancel()
}

func (m *mockStream) messages() []*pb.ChatMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*pb.ChatMessage, len(m.sent))
	copy(out, m.sent)
	return out
}

func TestForwardAgentOutput_SendsEndOfTurnOnTurnEnd(t *testing.T) {
	streamCh := make(chan string)
	toolCallCh := make(chan agent.ToolCall)
	errCh := make(chan error)
	doneCh := make(chan struct{})
	turnEndCh := make(chan struct{})

	output := &agent.AgentOutput{
		ID:         "test-1",
		StreamCh:   streamCh,
		ToolCallCh: toolCallCh,
		ErrCh:      errCh,
		DoneCh:     doneCh,
		TurnEndCh:  turnEndCh,
	}

	mock := newMockStream()

	cs := &chatServer{}
	go cs.forwardAgentOutput(output, mock)

	streamCh <- "Hello, "
	streamCh <- "world!"
	close(turnEndCh)

	time.Sleep(100 * time.Millisecond)
	sent := mock.messages()

	if len(sent) < 3 {
		t.Fatalf("expected at least 3 messages, got %d", len(sent))
	}
	if sent[0].MessageType != "text" || sent[0].Content != "Hello, " {
		t.Errorf("msg[0] = %+v, want text 'Hello, '", sent[0])
	}
	if sent[1].MessageType != "text" || sent[1].Content != "world!" {
		t.Errorf("msg[1] = %+v, want text 'world!'", sent[1])
	}
	if sent[2].MessageType != "text" || sent[2].Content != "" || !sent[2].EndOfTurn {
		t.Errorf("msg[2] = %+v, want EndOfTurn=true text with empty content", sent[2])
	}

	close(streamCh)
	close(toolCallCh)
	close(errCh)
	close(doneCh)
	time.Sleep(50 * time.Millisecond)
}

func TestForwardAgentOutput_ExitsOnDoneCh(t *testing.T) {
	streamCh := make(chan string)
	toolCallCh := make(chan agent.ToolCall)
	errCh := make(chan error)
	doneCh := make(chan struct{})
	turnEndCh := make(chan struct{})

	output := &agent.AgentOutput{
		ID:         "test-2",
		StreamCh:   streamCh,
		ToolCallCh: toolCallCh,
		ErrCh:      errCh,
		DoneCh:     doneCh,
		TurnEndCh:  turnEndCh,
	}

	mock := newMockStream()

	cs := &chatServer{}
	exited := make(chan struct{})
	go func() {
		cs.forwardAgentOutput(output, mock)
		close(exited)
	}()

	close(doneCh)

	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("forwardAgentOutput did not exit after doneCh closed")
	}

	sent := mock.messages()
	if len(sent) != 1 {
		t.Fatalf("expected 1 EndOfTurn message on done, got %d", len(sent))
	}
	if !sent[0].EndOfTurn {
		t.Errorf("expected EndOfTurn=true on done, got %+v", sent[0])
	}
}

func TestForwardAgentOutput_ExitsOnClosedStreamCh(t *testing.T) {
	streamCh := make(chan string)
	toolCallCh := make(chan agent.ToolCall)
	errCh := make(chan error)
	doneCh := make(chan struct{})
	turnEndCh := make(chan struct{})

	output := &agent.AgentOutput{
		ID:         "test-3",
		StreamCh:   streamCh,
		ToolCallCh: toolCallCh,
		ErrCh:      errCh,
		DoneCh:     doneCh,
		TurnEndCh:  turnEndCh,
	}

	mock := newMockStream()

	cs := &chatServer{}
	exited := make(chan struct{})
	go func() {
		cs.forwardAgentOutput(output, mock)
		close(exited)
	}()

	close(streamCh)

	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("forwardAgentOutput did not exit after streamCh closed")
	}

	sent := mock.messages()
	if len(sent) != 0 {
		t.Fatalf("expected 0 messages when streamCh closes without done, got %d", len(sent))
	}
}
