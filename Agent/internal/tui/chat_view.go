package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/Lyoomu/TAC/Agent/internal/agent"
)

type chatMessage struct {
	role             string // "user", "assistant", "system", "tool"
	content          string
	reasoningContent string
	reasoningFolded  bool
}

type chatViewModel struct {
	ctx      *AppContext
	width    int
	height   int
	ready    bool
	roleName string

	viewport viewport.Model
	input    textinput.Model
	spinner  spinner.Model

	messages   []chatMessage
	streaming  bool
	streamBuf  strings.Builder
	agentOut   *agent.AgentOutput
	mdRenderer *glamour.TermRenderer
}

func newChatViewModel(ctx *AppContext) *chatViewModel {
	ti := textinput.New()
	ti.Placeholder = "Type your message..."
	ti.CharLimit = 1024

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	vp := viewport.New(80, 20)

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
	}
}

func (c *chatViewModel) setSize(w, h int) {
	c.width = w
	c.height = h
	c.ready = true
	c.viewport.Width = w - 2
	c.viewport.Height = h - 5
	c.input.Width = w - 4
}

func (c *chatViewModel) startChat(roleName string) {
	c.roleName = roleName
	c.messages = []chatMessage{}
	c.streaming = false
	c.streamBuf.Reset()
	c.input.Focus()
}

func (c *chatViewModel) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		c.spinner.Tick,
	)
}

func (c *chatViewModel) Update(msg tea.Msg) (*chatViewModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if c.agentOut != nil {
				c.agentOut.Stop()
				c.agentOut = nil
			}
			return c, func() tea.Msg { return chatExitMsg{} }

		case "ctrl+c":

			if c.agentOut != nil && c.streaming {
				c.agentOut.Stop()
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
				c.updateViewport()
			}
			return c, nil

		case "enter":
			text := c.input.Value()
			if text == "" {
				return c, nil
			}
			c.input.SetValue("")

			c.messages = append(c.messages, chatMessage{role: "user", content: text})
			c.updateViewport()

			if c.agentOut == nil {

				return c, c.startAgentSession(text)
			}

			c.agentOut.SendMessage(text, agent.ModeInterrupt)
			c.streaming = true
			return c, c.waitForStream()

		case "pgup", "pgdown":
			var cmd tea.Cmd
			c.viewport, cmd = c.viewport.Update(msg)
			cmds = append(cmds, cmd)

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

	case agentSessionStartedMsg:
		c.agentOut = msg.output
		c.streaming = true
		return c, c.waitForStream()

	case agentStreamMsg:
		c.streamBuf.WriteString(msg.chunk)
		c.updateViewport()
		return c, c.waitForStream()

	case agentDoneMsg:
		c.streaming = false
		if c.streamBuf.Len() > 0 {
			rawContent := c.streamBuf.String()
			reasoning, content, _ := parseReasoningAndContent(rawContent)
			c.messages = append(c.messages, chatMessage{
				role:             "assistant",
				content:          content,
				reasoningContent: reasoning,
				reasoningFolded:  true,
			})
			c.streamBuf.Reset()
		}
		c.updateViewport()
		return c, nil

	case agentToolCallMsg:
		c.messages = append(c.messages, chatMessage{
			role:    "tool",
			content: fmt.Sprintf("🔧 %s(%s)", msg.name, msg.args),
		})
		c.updateViewport()

		if c.agentOut != nil {
			c.agentOut.SubmitToolResult(msg.id, `{"status":"executed from TUI"}`)
		}
		return c, c.waitForStream()

	case agentErrorMsg:
		c.messages = append(c.messages, chatMessage{
			role:    "system",
			content: "Error: " + msg.err.Error(),
		})
		c.updateViewport()
		return c, c.waitForStream()

	case spinner.TickMsg:
		var cmd tea.Cmd
		c.spinner, cmd = c.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return c, tea.Batch(cmds...)
}

func (c *chatViewModel) View() string {
	var b strings.Builder

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(primaryColor).
		Padding(0, 2).
		Width(c.width).
		Render(fmt.Sprintf(" Chat: %s ", c.roleName))
	b.WriteString(header)
	b.WriteString("\n")

	if c.ready {
		b.WriteString(c.viewport.View())
	} else {
		b.WriteString(hintStyle.Render("  Initializing chat view..."))
	}
	b.WriteString("\n")

	inputPrefix := "> "
	if c.streaming {
		inputPrefix = c.spinner.View() + " "
	}
	inputLine := chatInputStyle.Width(c.width).Render(inputPrefix + c.input.View())
	b.WriteString(inputLine)

	return b.String()
}

func (c *chatViewModel) updateViewport() {
	var content strings.Builder

	for _, msg := range c.messages {
		switch msg.role {
		case "user":
			content.WriteString(chatUserStyle.Render("🧑 You: "))
			content.WriteString(msg.content)
			content.WriteString("\n\n")
		case "assistant":
			content.WriteString(chatAssistantStyle.Render("🤖 Assistant: "))
			content.WriteString("\n")

			if msg.reasoningContent != "" {
				if msg.reasoningFolded {
					content.WriteString(lipgloss.NewStyle().Foreground(mutedColor).Italic(true).Render("💭 [Reasoning Content (folded) - Press 'ctrl+f' to expand]"))
					content.WriteString("\n\n")
				} else {
					content.WriteString(lipgloss.NewStyle().Foreground(mutedColor).Italic(true).Render("💭 Reasoning Content (Press 'ctrl+f' to fold):"))
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
		case "tool":
			content.WriteString(chatToolStyle.Render(msg.content))
			content.WriteString("\n\n")
		case "system":
			content.WriteString(errorStyle.Render(msg.content))
			content.WriteString("\n\n")
		}
	}

	if c.streaming && c.streamBuf.Len() > 0 {
		content.WriteString(chatAssistantStyle.Render("🤖 Assistant: "))
		content.WriteString("\n")

		reasoning, mainText, isStreamingReasoning := parseReasoningAndContent(c.streamBuf.String())
		if reasoning != "" {
			if isStreamingReasoning {
				content.WriteString(lipgloss.NewStyle().Foreground(mutedColor).Italic(true).Render("💭 Reasoning Content..."))
				content.WriteString("\n")
				lines := strings.Split(reasoning, "\n")
				for _, line := range lines {
					content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF")).Italic(true).Render("  " + line))
					content.WriteString("\n")
				}
			} else {
				content.WriteString(lipgloss.NewStyle().Foreground(mutedColor).Italic(true).Render("💭 [Reasoning Content (folded)]"))
				content.WriteString("\n\n")
			}
		}

		if mainText != "" {
			content.WriteString(mainText)
		}
		content.WriteString("▊")
		content.WriteString("\n")
	}

	c.viewport.SetContent(content.String())
	c.viewport.GotoBottom()
}

func parseReasoningAndContent(text string) (string, string, bool) {
	if !strings.Contains(text, "<reasoning>") {
		return "", text, false
	}

	parts := strings.SplitN(text, "<reasoning>", 2)
	prefix := parts[0]
	rest := parts[1]

	if !strings.Contains(rest, "</reasoning>") {
		return rest, prefix, true
	}

	subparts := strings.SplitN(rest, "</reasoning>", 2)
	reasoning := subparts[0]
	content := prefix + subparts[1]
	return reasoning, content, false
}

func (c *chatViewModel) startAgentSession(initMsg string) tea.Cmd {
	return func() tea.Msg {
		output, err := c.ctx.AgentManager.RunAgent(
			c.roleName,
			nil,
			initMsg,
			nil,
			agent.ModeInterrupt,
		)
		if err != nil {
			return agentErrorMsg{err: err}
		}

		return agentSessionStartedMsg{output: output}
	}
}

func (c *chatViewModel) waitForStream() tea.Cmd {
	if c.agentOut == nil {
		return nil
	}
	out := c.agentOut
	return func() tea.Msg {
		select {
		case chunk, ok := <-out.StreamCh:
			if !ok {
				return agentDoneMsg{}
			}
			return agentStreamMsg{chunk: chunk}
		case tc, ok := <-out.ToolCallCh:
			if !ok {
				return agentDoneMsg{}
			}
			return agentToolCallMsg{id: tc.ID, name: tc.Name, args: tc.Args}
		case err, ok := <-out.ErrCh:
			if !ok {
				return agentDoneMsg{}
			}
			return agentErrorMsg{err: err}
		case <-out.DoneCh:
			return agentDoneMsg{}
		}
	}
}

type agentSessionStartedMsg struct {
	output *agent.AgentOutput
}

type agentStreamMsg struct {
	chunk string
}

type agentDoneMsg struct{}

type agentToolCallMsg struct {
	id   string
	name string
	args string
}

type agentErrorMsg struct {
	err error
}
