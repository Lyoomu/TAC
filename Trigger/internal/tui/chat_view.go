package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/Lyoomu/TAC/Trigger/internal/daemon"
	"github.com/Lyoomu/TAC/Trigger/internal/model"
	"github.com/Lyoomu/TAC/Trigger/internal/tool"
	pb "github.com/Lyoomu/TAC/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type chatMessage struct {
	role             string
	content          string
	reasoningContent string
	reasoningFolded  bool
}

type chatViewModel struct {
	ctx        *AppContext
	width      int
	height     int
	ready      bool
	roleKey    string
	serverAddr string

	viewport   viewport.Model
	input      textinput.Model
	spinner    spinner.Model
	messages   []chatMessage
	streaming  bool
	streamBuf  strings.Builder
	turnBuf    strings.Builder
	mdRenderer *glamour.TermRenderer

	stream       pb.ChatService_ChatClient
	streamCtx    context.Context
	streamCancel context.CancelFunc
	msgCh        chan chatEvent
	streamDone   chan struct{} // closed when receiveLoop exits

	isWatching     bool
	pendingWatch   bool
	watchSessionID string
	triggerName    string
}

type chatEvent struct {
	eventType string // "text", "tool_call", "error", "done", "tool_result"
	content   string
	id        string
	name      string
	args      string
	err       error
}

type chatToolResultWatchMsg struct {
	id     string
	result string
}

type chatStreamClosedMsg struct{}

func newChatViewModel(ctx *AppContext) *chatViewModel {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "Type your message..."
	ti.CharLimit = 1024

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	vp := viewport.New(80, 20)
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(78),
	)

	return &chatViewModel{
		ctx:        ctx,
		input:      ti,
		spinner:    sp,
		viewport:   vp,
		messages:   []chatMessage{},
		mdRenderer: renderer,
		msgCh:      make(chan chatEvent, 16),
	}
}

func (c *chatViewModel) setSize(w, h int) {
	c.width = w
	c.height = h
	c.ready = true

	// Layout: header(1) + viewport(h-3) + input_with_border(2) = h
	c.viewport.Width = w
	c.viewport.Height = h - 3
	if c.viewport.Height < 1 {
		c.viewport.Height = 1
	}
	c.input.Width = w - 6 // account for border padding + prefix

	// Recreate markdown renderer with correct word wrap
	wrapWidth := w - 4
	if wrapWidth < 20 {
		wrapWidth = 20
	}
	c.mdRenderer, _ = glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(wrapWidth),
	)
}

func (c *chatViewModel) startChat(roleKey, serverAddr, sessionID, triggerName string) {
	c.roleKey = roleKey
	c.serverAddr = serverAddr
	c.messages = []chatMessage{}
	c.streaming = false
	c.streamBuf.Reset()
	c.turnBuf.Reset()

	c.isWatching = false
	c.pendingWatch = false
	c.watchSessionID = ""
	c.triggerName = triggerName
	if c.triggerName == "" {
		c.triggerName = "chat"
	}

	c.closeStream()
	c.msgCh = make(chan chatEvent, 16)

	parts := strings.SplitN(roleKey, "-", 2)
	if len(parts) == 2 {
		serverName, roleName := parts[0], parts[1]

		// Check if it's active in daemon
		if sessionID != "" {
			if client, err := daemon.NewClient(); err == nil {
				if activeResp, err := client.ListActiveSessions(); err == nil {
					for _, as := range activeResp.Sessions {
						if as.ServerName == serverName && as.RoleName == roleName && as.SessionId == sessionID {
							c.isWatching = true
							c.pendingWatch = true
							c.watchSessionID = sessionID
							break
						}
					}
				}
			}
		}

		sess, err := c.ctx.SessionManager.GetOrCreate(c.triggerName, serverName, roleName, sessionID)
		if err == nil && sess != nil {
			for _, msg := range sess.Messages {
				reasoning, content, _ := parseReasoningAndContent(msg.Content)
				if reasoning != "" {
					c.messages = append(c.messages, chatMessage{
						role:             msg.Role,
						content:          content,
						reasoningContent: reasoning,
						reasoningFolded:  true,
					})
				} else {
					c.messages = append(c.messages, chatMessage{
						role:    msg.Role,
						content: msg.Content,
					})
				}
			}
		}
	}

	if c.isWatching {
		c.input.Placeholder = "[Monitoring Active Session - Read Only]"
		c.input.Blur()
	} else {
		c.input.Placeholder = "Type your message..."
		c.input.Focus()
	}

	c.updateViewport()
}

func (c *chatViewModel) flushStreamBufToMessages() {
	if c.streamBuf.Len() > 0 {
		rawContent := c.streamBuf.String()
		reasoning, content, _ := parseReasoningAndContent(rawContent)
		if strings.TrimSpace(content) != "" || strings.TrimSpace(reasoning) != "" {
			c.messages = append(c.messages, chatMessage{
				role:             "assistant",
				content:          content,
				reasoningContent: reasoning,
				reasoningFolded:  true,
			})
		}
		c.streamBuf.Reset()
	}
	if c.turnBuf.Len() > 0 {
		parts := strings.SplitN(c.roleKey, "-", 2)
		if len(parts) == 2 {
			serverName, roleName := parts[0], parts[1]
			sess, err := c.ctx.SessionManager.GetOrCreate(c.triggerName, serverName, roleName, "")
			if err == nil && sess != nil {
				_ = c.ctx.SessionManager.AppendMessage(sess, "assistant", c.turnBuf.String())
			}
		}
		c.turnBuf.Reset()
	}
}

func (c *chatViewModel) closeStream() {
	c.flushStreamBufToMessages()
	if c.streamCancel != nil {
		c.streamCancel()
		c.streamCancel = nil
	}
	c.stream = nil
	c.streamCtx = nil
}

func (c *chatViewModel) resetSession() {
	c.closeStream()
	c.messages = []chatMessage{{
		role:    "system",
		content: "Session reset. The next message will start a new session.",
	}}
	c.streaming = false
	c.streamBuf.Reset()
	c.turnBuf.Reset()
	c.msgCh = make(chan chatEvent, 16)
	c.updateViewport()
}

func (c *chatViewModel) Init() tea.Cmd {
	var cmds []tea.Cmd
	cmds = append(cmds, textinput.Blink, c.spinner.Tick)
	if c.pendingWatch {
		c.pendingWatch = false
		cmds = append(cmds, c.startWatchCmd())
	}
	return tea.Batch(cmds...)
}

func (c *chatViewModel) Update(msg tea.Msg) (*chatViewModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			c.closeStream()
			return c, func() tea.Msg { return chatExitMsg{} }

		case "ctrl+c":

			if c.stream != nil {
				_ = c.stream.Send(&pb.ChatMessage{
					MessageType: "control",
					Content:     "interrupt",
					RoleName:    c.roleKey,
				})
				c.streaming = false
				if c.streamBuf.Len() > 0 {
					rawContent := c.streamBuf.String()
					reasoning, content, _ := parseReasoningAndContent(rawContent)
					c.messages = append(c.messages, chatMessage{
						role:             "assistant",
						content:          content + " [interrupted]",
						reasoningContent: reasoning,
						reasoningFolded:  true,
					})
					c.streamBuf.Reset()
				}

				// Save full turn content to session history
				if c.turnBuf.Len() > 0 {
					parts := strings.SplitN(c.roleKey, "-", 2)
					if len(parts) == 2 {
						serverName, roleName := parts[0], parts[1]
						sess, err := c.ctx.SessionManager.GetOrCreate(c.triggerName, serverName, roleName, "")
						if err == nil && sess != nil {
							_ = c.ctx.SessionManager.AppendMessage(sess, "assistant", c.turnBuf.String()+" [interrupted]")
						}
					}
					c.turnBuf.Reset()
				}
				c.updateViewport()
			}
			return c, nil

		case "enter":
			if c.isWatching {
				return c, nil
			}
			text := c.input.Value()
			if text == "" {
				return c, nil
			}
			c.input.SetValue("")
			if strings.TrimSpace(text) == "/session" {
				c.resetSession()
				return c, nil
			}

			c.messages = append(c.messages, chatMessage{role: "user", content: text})
			c.updateViewport()

			return c, c.sendMessage(text)

		case "pgup":
			halfPage := c.viewport.Height / 2
			if halfPage < 1 {
				halfPage = 1
			}
			c.viewport.SetYOffset(c.viewport.YOffset - halfPage)
			return c, nil
		case "pgdown":
			halfPage := c.viewport.Height / 2
			if halfPage < 1 {
				halfPage = 1
			}
			c.viewport.SetYOffset(c.viewport.YOffset + halfPage)
			return c, nil

		case "ctrl+f":
			for i := len(c.messages) - 1; i >= 0; i-- {
				if c.messages[i].role == "assistant" && c.messages[i].reasoningContent != "" {
					c.messages[i].reasoningFolded = !c.messages[i].reasoningFolded
					c.updateViewport()
					break
				}
			}
			return c, nil
		}

		var cmd tea.Cmd
		c.input, cmd = c.input.Update(msg)
		cmds = append(cmds, cmd)

	case chatConnectedMsg:
		c.streaming = true
		c.streamBuf.Reset()
		c.turnBuf.Reset()
		c.updateViewport()
		return c, c.waitForStream()

	case chatStreamMsg:
		c.streamBuf.WriteString(msg.chunk)
		c.turnBuf.WriteString(msg.chunk)
		c.updateViewport()
		return c, c.waitForStream()

	case chatToolCallMsg:
		c.flushStreamBufToMessages()
		c.messages = append(c.messages, chatMessage{
			role:    "system",
			content: fmt.Sprintf("Tool call: %s(%s)", msg.name, truncate(msg.args, 60)),
		})
		c.updateViewport()

		if c.isWatching {
			return c, c.waitForStream()
		}
		// Execute tool asynchronously to avoid blocking the UI
		return c, c.executeToolAsync(msg.id, msg.name, msg.args)

	case chatToolResultMsg:
		if c.stream != nil {
			_ = c.stream.Send(&pb.ChatMessage{
				MessageType: "tool_result",
				ToolCallId:  msg.id,
				ToolResult:  msg.result,
				RoleName:    c.roleKey,
			})
		}
		return c, c.waitForStream()

	case chatStreamReadyMsg:
		c.streaming = true
		c.streamBuf.Reset()
		c.turnBuf.Reset()
		c.updateViewport()
		return c, c.waitForStream()

	case chatDoneMsg:
		c.streaming = false
		c.flushStreamBufToMessages()
		c.updateViewport()
		// Keep stream alive for multi-turn conversation
		return c, nil

	case chatStreamClosedMsg:
		c.streaming = false
		c.stream = nil
		if c.streamCancel != nil {
			c.streamCancel()
			c.streamCancel = nil
		}
		c.streamCtx = nil
		c.streamDone = nil
		c.flushStreamBufToMessages()
		c.updateViewport()
		return c, nil

	case chatErrorMsg:
		c.streaming = false
		c.messages = append(c.messages, chatMessage{
			role:    "system",
			content: "Error: " + msg.err.Error(),
		})
		c.updateViewport()
		return c, nil

	case tea.MouseMsg:
		var cmd tea.Cmd
		c.viewport, cmd = c.viewport.Update(msg)
		return c, cmd

	case spinner.TickMsg:
		var cmd tea.Cmd
		c.spinner, cmd = c.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return c, tea.Batch(cmds...)
}

func (c *chatViewModel) View() string {
	var b strings.Builder

	// Header bar
	headerText := fmt.Sprintf(" Chat: %s  [Esc] Back  [PgUp/PgDn] Scroll  [Ctrl+C] Interrupt  [Ctrl+F] Fold", c.roleKey)
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(primaryColor).
		Width(c.width).
		Padding(0, 1).
		Render(headerText)
	b.WriteString(header)
	b.WriteString("\n")

	// Message viewport
	if c.ready {
		b.WriteString(c.viewport.View())
	} else {
		b.WriteString(padToHeight(hintStyle.Render("  Initializing chat view..."), c.viewport.Height))
	}
	b.WriteString("\n")

	// Input line with border-top
	inputPrefix := "> "
	if c.streaming {
		inputPrefix = c.spinner.View() + " "
	}
	inputLine := chatInputStyle.Width(c.width).Render(inputPrefix + c.input.View())
	b.WriteString(inputLine)

	return b.String()
}

func (c *chatViewModel) sendMessage(text string) tea.Cmd {
	return func() tea.Msg {
		if c.ctx.ServerEngine == nil {
			return chatErrorMsg{err: fmt.Errorf("server engine not initialized")}
		}

		var matchedRole model.LoadedRole
		for _, r := range c.ctx.ServerEngine.GetLoadedRoles() {
			key := r.ServerName + "-" + r.RoleName
			if key == c.roleKey {
				matchedRole = r
				break
			}
		}
		if matchedRole.ServerName == "" {
			return chatErrorMsg{err: fmt.Errorf("role '%s' not found in loaded roles", c.roleKey)}
		}

		serverConn, err := c.ctx.ServerEngine.GetByDisplayName(matchedRole.ServerName)
		if err != nil {
			return chatErrorMsg{err: err}
		}

		sess, err := c.ctx.SessionManager.GetOrCreate(c.triggerName, matchedRole.ServerName, matchedRole.RoleName, "")
		if err != nil {
			return chatErrorMsg{err: err}
		}

		// Check if existing stream is still alive
		if c.stream != nil && c.streamDone != nil {
			select {
			case <-c.streamDone:
				// receiveLoop exited - stream is dead, need to reconnect
				c.stream = nil
				if c.streamCancel != nil {
					c.streamCancel()
					c.streamCancel = nil
				}
				c.streamCtx = nil
				c.streamDone = nil
				c.msgCh = make(chan chatEvent, 16)
			default:
				// Stream is alive - drain any stale events before new turn
				for {
					select {
					case <-c.msgCh:
					default:
						goto drained
					}
				}
			drained:
			}
		}

		isNewStream := c.stream == nil
		if isNewStream {
			client, err := c.ctx.ServerEngine.GetClient(serverConn.Address)
			if err != nil {
				return chatErrorMsg{err: err}
			}

			ctx, cancel := context.WithCancel(context.Background())
			c.streamCtx = ctx
			c.streamCancel = cancel
			stream, err := client.Chat.Chat(ctx)
			if err != nil {
				cancel()
				return chatErrorMsg{err: fmt.Errorf("start chat stream: %w", err)}
			}
			c.stream = stream
			c.streamDone = make(chan struct{})

			// Replay session history for new stream
			if len(sess.Messages) > 0 {
				for _, msg := range sess.Messages {
					_ = stream.Send(&pb.ChatMessage{
						Role:        msg.Role,
						Content:     msg.Content,
						MessageType: "history",
						IsHistory:   true,
						RoleName:    matchedRole.RoleName,
						SessionId:   sess.ID,
					})
				}
			}

			go c.receiveLoop(stream, c.msgCh, c.streamDone)
		}

		_ = c.ctx.SessionManager.AppendMessage(sess, "user", text)
		err = c.stream.Send(&pb.ChatMessage{
			MessageType: "text",
			Content:     text,
			Role:        "user",
			RoleName:    matchedRole.RoleName,
			SessionId:   sess.ID,
		})
		if err != nil {
			return chatErrorMsg{err: fmt.Errorf("send: %w", err)}
		}

		if isNewStream {
			return chatConnectedMsg{}
		}
		return chatStreamReadyMsg{}
	}
}

func (c *chatViewModel) receiveLoop(stream pb.ChatService_ChatClient, msgCh chan<- chatEvent, done chan struct{}) {
	defer close(done)
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			// Stream closed by server - don't send "done" (EndOfTurn handles that)
			return
		}
		if err != nil {
			if errors.Is(err, context.Canceled) || status.Code(err) == codes.Canceled {
				return
			}
			msgCh <- chatEvent{eventType: "error", err: err}
			return
		}

		switch resp.MessageType {
		case "text":
			msgCh <- chatEvent{eventType: "text", content: resp.Content}
			if resp.EndOfTurn {
				msgCh <- chatEvent{eventType: "done"}
			}
		case "tool_call":
			msgCh <- chatEvent{eventType: "tool_call", id: resp.ToolCallId, name: resp.ToolName, args: resp.ToolArguments}
		case "error":
			msgCh <- chatEvent{eventType: "error", content: resp.ErrorMessage}
		}
	}
}

func (c *chatViewModel) waitForStream() tea.Cmd {
	// Capture references to avoid races if stream is replaced
	ch := c.msgCh
	done := c.streamDone // nil channel blocks forever in select (safe)
	ctx := c.streamCtx
	if ch == nil || ctx == nil {
		return nil
	}
	return func() tea.Msg {
		select {
		case evt, ok := <-ch:
			if !ok {
				return chatStreamClosedMsg{}
			}
			switch evt.eventType {
			case "text":
				return chatStreamMsg{chunk: evt.content}
			case "tool_call":
				return chatToolCallMsg{id: evt.id, name: evt.name, args: evt.args}
			case "tool_result":
				return chatToolResultWatchMsg{id: evt.id, result: evt.content}
			case "error":
				if evt.err != nil {
					return chatErrorMsg{err: evt.err}
				}
				return chatErrorMsg{err: fmt.Errorf("%s", evt.content)}
			case "done":
				return chatDoneMsg{}
			}
		case <-done:
			// receiveLoop exited (EOF or error) - stream is dead
			return chatStreamClosedMsg{}
		case <-ctx.Done():
			return chatDoneMsg{}
		}
		return nil
	}
}

func (c *chatViewModel) executeToolAsync(toolCallID, toolName, toolArgs string) tea.Cmd {
	return func() tea.Msg {
		var result string
		parts := strings.SplitN(c.roleKey, "-", 2)
		if len(parts) == 2 {
			serverName, roleName := parts[0], parts[1]
			toolInfo, ok := tool.FindLoadedTool(c.ctx.ServerEngine.GetLoadedRoles(), serverName, roleName, toolName)
			if ok {
				res, err := tool.Execute(toolInfo, toolArgs)
				if err != nil {
					errJSON, _ := json.Marshal(map[string]string{"error": err.Error()})
					result = string(errJSON)
				} else {
					result = res
				}
			} else {
				errJSON, _ := json.Marshal(map[string]string{"error": "tool not loaded: " + toolName})
				result = string(errJSON)
			}
		} else {
			errJSON, _ := json.Marshal(map[string]string{"error": "invalid role key: " + c.roleKey})
			result = string(errJSON)
		}
		return chatToolResultMsg{id: toolCallID, result: result}
	}
}

func (c *chatViewModel) updateViewport() {
	var content strings.Builder

	for _, msg := range c.messages {
		switch msg.role {
		case "user":
			content.WriteString(chatUserStyle.Render("You: "))
			content.WriteString(msg.content)
			content.WriteString("\n\n")
		case "assistant":
			content.WriteString(chatAssistantStyle.Render("Assistant: "))
			content.WriteString("\n")

			if msg.reasoningContent != "" {
				if msg.reasoningFolded {
					content.WriteString(lipgloss.NewStyle().Foreground(mutedColor).Italic(true).Render("[Reasoning (folded) - Ctrl+F to expand]"))
					content.WriteString("\n")
				} else {
					content.WriteString(lipgloss.NewStyle().Foreground(mutedColor).Italic(true).Render("Reasoning (Ctrl+F to fold):"))
					content.WriteString("\n")
					lines := strings.Split(msg.reasoningContent, "\n")
					for _, line := range lines {
						content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Italic(true).Render("  " + line))
						content.WriteString("\n")
					}
					content.WriteString("\n")
				}
			}

			if c.mdRenderer != nil {
				rendered, err := c.mdRenderer.Render(msg.content)
				if err == nil {
					content.WriteString(rendered)
				} else {
					content.WriteString(msg.content)
				}
			} else {
				content.WriteString(msg.content)
			}
			content.WriteString("\n")
		case "system":
			if strings.HasPrefix(msg.content, "Tool call:") {
				content.WriteString(lipgloss.NewStyle().Foreground(accentColor).Render("⚙ " + msg.content))
				content.WriteString("\n")
			} else if strings.HasPrefix(msg.content, "Error:") {
				content.WriteString(errorStyle.Render("✗ " + msg.content))
				content.WriteString("\n\n")
			} else {
				content.WriteString(hintStyle.Render("● " + msg.content))
				content.WriteString("\n\n")
			}
		}
	}

	if c.streaming && c.streamBuf.Len() > 0 {
		content.WriteString(chatAssistantStyle.Render("Assistant: "))
		content.WriteString("\n")

		reasoning, mainText, isStreamingReasoning := parseReasoningAndContent(c.streamBuf.String())
		if reasoning != "" {
			if isStreamingReasoning {
				content.WriteString(lipgloss.NewStyle().Foreground(mutedColor).Italic(true).Render("Reasoning..."))
				content.WriteString("\n")
				lines := strings.Split(reasoning, "\n")
				for _, line := range lines {
					content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Italic(true).Render("  " + line))
					content.WriteString("\n")
				}
			} else {
				content.WriteString(lipgloss.NewStyle().Foreground(mutedColor).Italic(true).Render("[Reasoning (folded)]"))
				content.WriteString("\n\n")
			}
		}

		if mainText != "" {
			if c.mdRenderer != nil {
				rendered, err := c.mdRenderer.Render(mainText)
				if err == nil {
					content.WriteString(rendered)
				} else {
					content.WriteString(mainText)
				}
			} else {
				content.WriteString(mainText)
			}
		}
		content.WriteString("_")
		content.WriteString("\n")
	}

	c.viewport.SetContent(content.String())
	c.viewport.GotoBottom()
}

func parseReasoningAndContent(text string) (string, string, bool) {
	if !strings.Contains(text, "<reasoning>") {
		return "", text, false
	}

	var reasoningBuf strings.Builder
	content := text

	for {
		startIdx := strings.Index(content, "<reasoning>")
		if startIdx == -1 {
			break
		}
		endIdx := strings.Index(content[startIdx:], "</reasoning>")
		if endIdx == -1 {
			// Incomplete reasoning tag - still streaming
			block := content[startIdx+len("<reasoning>"):]
			if reasoningBuf.Len() > 0 {
				reasoningBuf.WriteString("\n")
			}
			reasoningBuf.WriteString(block)
			content = content[:startIdx]
			return strings.TrimSpace(reasoningBuf.String()), strings.TrimSpace(content), true
		}
		endIdx += startIdx
		block := content[startIdx+len("<reasoning>") : endIdx]
		if reasoningBuf.Len() > 0 {
			reasoningBuf.WriteString("\n")
		}
		reasoningBuf.WriteString(block)
		content = content[:startIdx] + content[endIdx+len("</reasoning>"):]
	}

	if reasoningBuf.Len() > 0 {
		return strings.TrimSpace(reasoningBuf.String()), strings.TrimSpace(content), false
	}
	return "", content, false
}

type chatStreamMsg struct {
	chunk string
}

type chatDoneMsg struct{}

type chatToolCallMsg struct {
	id   string
	name string
	args string
}

type chatConnectedMsg struct{}

type chatStreamReadyMsg struct{}

type chatToolResultMsg struct {
	id     string
	result string
}

type chatErrorMsg struct {
	err error
}

func (c *chatViewModel) startWatchCmd() tea.Cmd {
	return func() tea.Msg {
		parts := strings.SplitN(c.roleKey, "-", 2)
		if len(parts) != 2 {
			return chatErrorMsg{err: fmt.Errorf("invalid role key")}
		}
		serverName, roleName := parts[0], parts[1]

		client, err := daemon.NewClient()
		if err != nil {
			return chatErrorMsg{err: err}
		}

		ctx, cancel := context.WithCancel(context.Background())
		c.streamCtx = ctx
		c.streamCancel = cancel

		stream, err := client.WatchSession(ctx, serverName, roleName, c.watchSessionID)
		if err != nil {
			cancel()
			return chatErrorMsg{err: fmt.Errorf("watch session: %w", err)}
		}

		c.streamDone = make(chan struct{})
		go c.watchReceiveLoop(stream, c.msgCh, c.streamDone)

		return chatStreamReadyMsg{}
	}
}

func (c *chatViewModel) watchReceiveLoop(stream pb.DaemonService_WatchSessionClient, msgCh chan<- chatEvent, done chan struct{}) {
	defer close(done)
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			return
		}
		if err != nil {
			if errors.Is(err, context.Canceled) || status.Code(err) == codes.Canceled {
				return
			}
			msgCh <- chatEvent{eventType: "error", err: err}
			return
		}

		switch resp.MessageType {
		case "text":
			msgCh <- chatEvent{eventType: "text", content: resp.Content}
			if resp.EndOfTurn {
				msgCh <- chatEvent{eventType: "done"}
			}
		case "tool_call":
			msgCh <- chatEvent{eventType: "tool_call", id: resp.ToolCallId, name: resp.ToolName, args: resp.ToolArguments}
		case "tool_result":
			msgCh <- chatEvent{eventType: "tool_result", id: resp.ToolCallId, content: resp.ToolResult}
		case "error":
			msgCh <- chatEvent{eventType: "error", content: resp.ErrorMessage}
		}
	}
}
