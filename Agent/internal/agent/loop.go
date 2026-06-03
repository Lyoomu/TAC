package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Lyoomu/TAC/Agent/internal/models/llm"
)

func trimHistory(agent *Agent) {
	if agent.MaxHistory <= 0 {
		return
	}

	var systemMsgs []llm.Message
	var convMsgs []llm.Message
	for _, msg := range agent.History {
		if msg.Role == "system" {
			systemMsgs = append(systemMsgs, msg)
		} else {
			convMsgs = append(convMsgs, msg)
		}
	}

	if len(convMsgs) <= agent.MaxHistory {
		return
	}

	trimmed := convMsgs[len(convMsgs)-agent.MaxHistory:]
	agent.History = append(systemMsgs, trimmed...)
}

func (m *Manager) agentLoop(agent *Agent, client streamClient) {
	defer close(agent.streamCh)
	defer close(agent.toolCallCh)
	defer close(agent.errCh)
	defer close(agent.doneCh)
	defer func() {
		agent.mu.Lock()
		agent.Status = StatusStopped
		agent.mu.Unlock()
	}()

	for {
		var um userMessage
		select {
		case um = <-agent.userMsgCh:
		case <-agent.stopCh:
			return
		}

		if um.content != "" {
			agent.History = append(agent.History, llm.NewTextMessage("user", um.content))
		}

		pendingUserMsg := m.processTurn(agent, um.mode, client)
		if pendingUserMsg.content != "" {
			agent.History = append(agent.History, llm.NewTextMessage("user", pendingUserMsg.content))
		}

		select {
		case agent.turnEndCh <- struct{}{}:
		default:
		}
	}
}

func (m *Manager) processTurn(agent *Agent, mode MessageMode, client streamClient) (pendingUserMsg userMessage) {
	for {
		trimHistory(agent)
		streamCh, resultCh, errCh := client.ChatStreamV2(agent.History, agent.Tools)

		var content strings.Builder
		var assistantMsg llm.AssistantMessage
		interrupted := false

		for !allNil(streamCh, resultCh, errCh) {
			if mode == ModeQueue {
				select {
				case <-agent.stopCh:
					return userMessage{}
				case chunk, ok := <-streamCh:
					if !ok {
						streamCh = nil
					} else {
						content.WriteString(chunk)
						agent.streamCh <- chunk
					}
				case result, ok := <-resultCh:
					if !ok {
						resultCh = nil
					} else {
						assistantMsg = result
					}
				case err, ok := <-errCh:
					if !ok {
						errCh = nil
					} else {
						agent.errCh <- err
					}
				}
			} else {
				select {
				case <-agent.stopCh:
					return userMessage{}
				case newMsg := <-agent.userMsgCh:
					if mode == ModeInterrupt {
						interrupted = true
						pendingUserMsg = newMsg
						agent.streamCh <- "[interrupt]"
						go drain(streamCh, resultCh)
					} else {
						select {
						case agent.errCh <- fmt.Errorf("message rejected: agent is busy processing"):
						default:
						}
					}
				case chunk, ok := <-streamCh:
					if !ok {
						streamCh = nil
					} else if !interrupted {
						content.WriteString(chunk)
						agent.streamCh <- chunk
					}
				case result, ok := <-resultCh:
					if !ok {
						resultCh = nil
					} else {
						assistantMsg = result
					}
				case err, ok := <-errCh:
					if !ok {
						errCh = nil
					} else {
						agent.errCh <- err
					}
				}
			}
		}

		if interrupted {
			interruptedContent := content.String() + "[interrupt]"
			msgContent, _ := json.Marshal(interruptedContent)
			agent.History = append(agent.History, llm.Message{
				Role:    "assistant",
				Content: msgContent,
			})
			return pendingUserMsg
		}

		msgContent, _ := json.Marshal(content.String())
		agent.History = append(agent.History, llm.Message{
			Role:             "assistant",
			Content:          msgContent,
			ToolCalls:        assistantMsg.ToolCalls,
			ReasoningContent: assistantMsg.ReasoningContent,
		})

		if len(assistantMsg.ToolCalls) == 0 {
			return userMessage{}
		}

		for _, tc := range assistantMsg.ToolCalls {
			if mode == ModeQueue {
				select {
				case agent.toolCallCh <- ToolCall{
					ID:   tc.ID,
					Name: tc.Function.Name,
					Args: tc.Function.Arguments,
				}:
				case <-agent.stopCh:
					return userMessage{}
				}
			} else {
				select {
				case agent.toolCallCh <- ToolCall{
					ID:   tc.ID,
					Name: tc.Function.Name,
					Args: tc.Function.Arguments,
				}:
				case newMsg := <-agent.userMsgCh:
					if mode == ModeInterrupt {
						agent.streamCh <- "[interrupt]"
						return newMsg
					}
					select {
					case agent.errCh <- fmt.Errorf("message rejected: agent is busy processing"):
					default:
					}
				case <-agent.stopCh:
					return userMessage{}
				}
			}
		}

		for i := 0; i < len(assistantMsg.ToolCalls); i++ {
			if mode == ModeQueue {
				select {
				case tr := <-agent.toolResCh:
					agent.History = append(agent.History, llm.NewToolResultMessage(tr.ToolCallID, tr.Content))
				case <-agent.stopCh:
					return userMessage{}
				}
			} else {
				select {
				case tr := <-agent.toolResCh:
					agent.History = append(agent.History, llm.NewToolResultMessage(tr.ToolCallID, tr.Content))
				case newMsg := <-agent.userMsgCh:
					if mode == ModeInterrupt {
						agent.streamCh <- "[interrupt]"
						return newMsg
					}
					select {
					case agent.errCh <- fmt.Errorf("message rejected: agent is busy processing"):
					default:
					}
				case <-agent.stopCh:
					return userMessage{}
				}
			}
		}
	}
}
