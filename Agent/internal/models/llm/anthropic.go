package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Lyoomu/TAC/Agent/internal/config"
	"github.com/Lyoomu/TAC/Agent/internal/model"
)

type AnthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []AnthropicMessage `json:"messages"`
	Tools     []AnthropicTool    `json:"tools,omitempty"`
	Stream    bool               `json:"stream,omitempty"`
}

type AnthropicMessage struct {
	Role    string             `json:"role"`
	Content []AnthropicContent `json:"content"`
}

type AnthropicContent struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	Thinking  string `json:"thinking,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Input     any    `json:"input,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
}

type AnthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type AnthropicResponse struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	Role         string             `json:"role"`
	Model        string             `json:"model"`
	StopReason   string             `json:"stop_reason"`
	StopSequence string             `json:"stop_sequence"`
	Content      []AnthropicContent `json:"content"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type AnthropicStreamEvent struct {
	Type         string             `json:"type"`
	Index        int                `json:"index,omitempty"`
	Message      *AnthropicResponse `json:"message,omitempty"`
	ContentBlock *AnthropicContent  `json:"content_block,omitempty"`
	Delta        *AnthropicDelta    `json:"delta,omitempty"`
}

type AnthropicDelta struct {
	Type         string `json:"type"`
	Text         string `json:"text,omitempty"`
	Thinking     string `json:"thinking,omitempty"`
	PartialJSON  string `json:"partial_json,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

type AnthropicClient struct {
	modelConfig *model.Model
	config      *config.Config
	httpClient  *http.Client
	maxTokens   int
}

func NewAnthropicClient(m *model.Model, cfg *config.Config) *AnthropicClient {
	return &AnthropicClient{
		modelConfig: m,
		config:      cfg,
		httpClient:  &http.Client{Timeout: 120 * time.Second},
		maxTokens:   8192,
	}
}

func (c *AnthropicClient) SetHTTPClient(client *http.Client) {
	c.httpClient = client
}

func (c *AnthropicClient) modelID() string {
	if c.modelConfig.Model != "" {
		return c.modelConfig.Model
	}
	return c.modelConfig.Name
}

func (c *AnthropicClient) ChatStreamV2(messages []Message, toolDefs []json.RawMessage) (
	<-chan string, <-chan AssistantMessage, <-chan error,
) {
	textCh := make(chan string)
	resultCh := make(chan AssistantMessage, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(textCh)
		defer close(resultCh)
		defer close(errCh)

		system, anthropicMessages := c.messagesToAnthropic(messages)
		anthropicTools := c.convertTools(toolDefs)

		req := &AnthropicRequest{
			Model:     c.modelID(),
			MaxTokens: c.maxTokens,
			System:    system,
			Messages:  anthropicMessages,
			Tools:     anthropicTools,
			Stream:    true,
		}

		body, err := json.Marshal(req)
		if err != nil {
			errCh <- err
			return
		}

		httpReq, err := http.NewRequest("POST", c.modelConfig.BaseURL+"/messages", bytes.NewReader(body))
		if err != nil {
			errCh <- err
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")
		if c.modelConfig.APIKey != "" {
			httpReq.Header.Set("x-api-key", c.modelConfig.APIKey)
			httpReq.Header.Set("anthropic-version", "2023-06-01")
		}

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			errCh <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errCh <- fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
			return
		}

		var content strings.Builder
		var toolUses []struct {
			id    string
			name  string
			input strings.Builder
		}

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				errCh <- err
				return
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if line == "event: message_stop" {

				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			var event AnthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			switch event.Type {
			case "content_block_start":
				if event.ContentBlock != nil {
					switch event.ContentBlock.Type {
					case "tool_use":
						toolUses = append(toolUses, struct {
							id    string
							name  string
							input strings.Builder
						}{
							id:   event.ContentBlock.ID,
							name: event.ContentBlock.Name,
						})
					}
				}

			case "content_block_delta":
				if event.Delta != nil {
					switch event.Delta.Type {
					case "text_delta":
						if event.Delta.Text != "" {
							content.WriteString(event.Delta.Text)
							textCh <- event.Delta.Text
						}
					case "thinking_delta":
						if event.Delta.Thinking != "" {
							content.WriteString(event.Delta.Thinking)
							textCh <- event.Delta.Thinking
						}
					case "input_json_delta":
						if len(toolUses) > 0 && event.Index < len(toolUses) {
							toolUses[event.Index].input.WriteString(event.Delta.PartialJSON)
						}
					}
				}

			case "message_stop":

				var msg AssistantMessage
				msg.Content = content.String()
				for _, tu := range toolUses {
					msg.ToolCalls = append(msg.ToolCalls, ToolCall{
						ID:   tu.id,
						Type: "function",
						Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{
							Name:      tu.name,
							Arguments: tu.input.String(),
						},
					})
				}
				resultCh <- msg
				return
			}
		}

		var msg AssistantMessage
		msg.Content = content.String()
		for _, tu := range toolUses {
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID:   tu.id,
				Type: "function",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      tu.name,
					Arguments: tu.input.String(),
				},
			})
		}
		resultCh <- msg
	}()

	return textCh, resultCh, errCh
}

func (c *AnthropicClient) messagesToAnthropic(messages []Message) (string, []AnthropicMessage) {
	var systemParts []string
	var anthropicMessages []AnthropicMessage

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			text, _ := c.extractText(msg.Content)
			if text != "" {
				systemParts = append(systemParts, text)
			}

		case "user":
			if msg.ToolCallID != "" {

				result, _ := c.extractText(msg.Content)
				anthropicMessages = append(anthropicMessages, AnthropicMessage{
					Role: "user",
					Content: []AnthropicContent{{
						Type:      "tool_result",
						ToolUseID: msg.ToolCallID,
						Content:   result,
					}},
				})
			} else {

				parts := c.contentToAnthropic(msg.Content)
				anthropicMessages = append(anthropicMessages, AnthropicMessage{
					Role:    "user",
					Content: parts,
				})
			}

		case "assistant":
			var contentBlocks []AnthropicContent
			text, _ := c.extractText(msg.Content)
			if text != "" {
				contentBlocks = append(contentBlocks, AnthropicContent{
					Type: "text",
					Text: text,
				})
			}
			for _, tc := range msg.ToolCalls {
				var inputObj any
				json.Unmarshal([]byte(tc.Function.Arguments), &inputObj)
				contentBlocks = append(contentBlocks, AnthropicContent{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: inputObj,
				})
			}
			anthropicMessages = append(anthropicMessages, AnthropicMessage{
				Role:    "assistant",
				Content: contentBlocks,
			})
		}
	}

	return strings.Join(systemParts, "\n\n"), anthropicMessages
}

func (c *AnthropicClient) contentToAnthropic(raw json.RawMessage) []AnthropicContent {

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return []AnthropicContent{{Type: "text", Text: text}}
	}

	var parts []ContentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return []AnthropicContent{{Type: "text", Text: string(raw)}}
	}

	var blocks []AnthropicContent
	for _, part := range parts {
		switch part.Type {
		case "text":
			blocks = append(blocks, AnthropicContent{
				Type: "text",
				Text: part.Text,
			})
		case "image_url":

			if part.ImageURL != nil {
				blocks = append(blocks, c.imageURLToAnthropic(part.ImageURL.URL))
			}
		case "input_audio":

			blocks = append(blocks, AnthropicContent{
				Type: "text",
				Text: "[音频输入]",
			})
		}
	}
	return blocks
}

func (c *AnthropicClient) imageURLToAnthropic(dataURL string) AnthropicContent {

	if !strings.HasPrefix(dataURL, "data:") {
		return AnthropicContent{Type: "text", Text: "[图片: " + dataURL + "]"}
	}

	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return AnthropicContent{Type: "text", Text: "[图片]"}
	}

	meta := strings.TrimPrefix(parts[0], "data:")
	mediaType := strings.TrimSuffix(meta, ";base64")

	return AnthropicContent{
		Type: "image",

		Text: "[图片: " + mediaType + "]",
	}
}

func (c *AnthropicClient) extractText(raw json.RawMessage) (string, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}
	var parts []ContentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", err
	}
	var texts []string
	for _, part := range parts {
		if part.Type == "text" || part.Type == "" {
			texts = append(texts, part.Text)
		}
	}
	return strings.Join(texts, ""), nil
}

func (c *AnthropicClient) convertTools(toolDefs []json.RawMessage) []AnthropicTool {
	if len(toolDefs) == 0 {
		return nil
	}
	tools := make([]AnthropicTool, 0, len(toolDefs))
	for _, def := range toolDefs {
		var src struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Parameters  json.RawMessage `json:"parameters"`
		}
		if err := json.Unmarshal(def, &src); err != nil {
			continue
		}
		tools = append(tools, AnthropicTool{
			Name:        src.Name,
			Description: src.Description,
			InputSchema: src.Parameters,
		})
	}
	return tools
}
