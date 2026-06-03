package server

import (
	"fmt"
	"io"
	"time"

	"github.com/Lyoomu/TAC/Agent/internal/agent"
	"github.com/Lyoomu/TAC/Agent/internal/models/llm"
	pb "github.com/Lyoomu/TAC/proto"
)

type chatServer struct {
	server *Server
	pb.UnimplementedChatServiceServer
}

func (c *chatServer) Chat(stream pb.ChatService_ChatServer) error {
	var currentOutput *agent.AgentOutput
	var currentRoleName string
	var currentMode agent.MessageMode
	var pendingHistory []llm.Message // 缓存 Trigger 发送的历史消息

	defer func() {
		if currentOutput != nil {
			currentOutput.Stop()
			currentOutput = nil
		}
	}()

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("recv: %w", err)
		}

		switch msg.MessageType {
		case "history":

			pendingHistory = append(pendingHistory, llm.NewTextMessage(msg.Role, msg.Content))

		case "text":
			if currentOutput == nil {

				if msg.RoleName == "" {
					_ = stream.Send(&pb.ChatMessage{
						MessageType:  "error",
						ErrorMessage: "role_name is required for first message",
						EndOfTurn:    true,
						Timestamp:    time.Now().Format(time.RFC3339),
					})
					continue
				}

				currentRoleName = msg.RoleName

				session := make([]llm.Message, len(pendingHistory))
				copy(session, pendingHistory)
				pendingHistory = nil

				defaultMode := agent.ModeInterrupt
				if role, err := c.server.roleEngine.Get(currentRoleName); err == nil && role.MessageMode != "" {
					defaultMode = agent.MessageMode(role.MessageMode)
				}

				if msg.MessageMode != "" {
					currentMode = agent.MessageMode(msg.MessageMode)
				} else {
					currentMode = defaultMode
				}

				output, err := c.server.agentMgr.RunAgent(
					currentRoleName,
					nil,
					msg.Content,
					session,
					currentMode,
				)
				if err != nil {
					_ = stream.Send(&pb.ChatMessage{
						MessageType:  "error",
						ErrorMessage: err.Error(),
						EndOfTurn:    true,
						Timestamp:    time.Now().Format(time.RFC3339),
					})
					continue
				}
				currentOutput = output

				go c.forwardAgentOutput(output, stream)
			} else {

				mode := currentMode
				if msg.MessageMode != "" {
					mode = agent.MessageMode(msg.MessageMode)
				}
				currentOutput.SendMessage(msg.Content, mode)
			}

		case "tool_result":

			if currentOutput != nil {
				currentOutput.SubmitToolResult(msg.ToolCallId, msg.ToolResult)
			}

		case "control":

			switch msg.Content {
			case "interrupt":
				if currentOutput != nil {
					currentOutput.Stop()
					currentOutput = nil
				}
			case "end_session":
				if currentOutput != nil {
					currentOutput.Stop()
					currentOutput = nil
				}
				return nil
			}
		}
	}
}

func (c *chatServer) forwardAgentOutput(output *agent.AgentOutput, stream pb.ChatService_ChatServer) {
	for {
		select {
		case chunk, ok := <-output.StreamCh:
			if !ok {
				return
			}
			_ = stream.Send(&pb.ChatMessage{
				MessageType: "text",
				Content:     chunk,
				Role:        "assistant",
				Timestamp:   time.Now().Format(time.RFC3339),
			})

		case toolCall, ok := <-output.ToolCallCh:
			if !ok {
				return
			}
			_ = stream.Send(&pb.ChatMessage{
				MessageType:   "tool_call",
				ToolName:      toolCall.Name,
				ToolCallId:    toolCall.ID,
				ToolArguments: toolCall.Args,
				Role:          "assistant",
				Timestamp:     time.Now().Format(time.RFC3339),
			})

		case err, ok := <-output.ErrCh:
			if !ok {
				return
			}
			_ = stream.Send(&pb.ChatMessage{
				MessageType:  "error",
				ErrorMessage: err.Error(),
				Role:         "assistant",
				Timestamp:    time.Now().Format(time.RFC3339),
			})

		case <-output.TurnEndCh:
			_ = stream.Send(&pb.ChatMessage{
				MessageType: "text",
				Content:     "",
				Role:        "assistant",
				EndOfTurn:   true,
				Timestamp:   time.Now().Format(time.RFC3339),
			})

		case <-output.DoneCh:
			_ = stream.Send(&pb.ChatMessage{
				MessageType: "text",
				Content:     "",
				Role:        "assistant",
				EndOfTurn:   true,
				Timestamp:   time.Now().Format(time.RFC3339),
			})
			return
		}
	}
}
