package llm

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Lyoomu/TAC/Agent/internal/config"
	"github.com/Lyoomu/TAC/Agent/internal/model"
)

type ChatCompletionRequest struct {
	Model      string    `json:"model"`
	Messages   []Message `json:"messages"`
	Tools      []Tool    `json:"tools,omitempty"`
	ToolChoice string    `json:"tool_choice,omitempty"`
	Stream     bool      `json:"stream,omitempty"`
}

type Tool struct {
	Type     string          `json:"type"`
	Function json.RawMessage `json:"function"`
}

type Message struct {
	Role             string          `json:"role"`
	Content          json.RawMessage `json:"content"`
	ToolCallID       string          `json:"tool_call_id,omitempty"`
	ToolCalls        []ToolCall      `json:"tool_calls,omitempty"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
}

func NewTextMessage(role, text string) Message {
	raw, _ := json.Marshal(text)
	return Message{Role: role, Content: raw}
}

func NewMultimodalMessage(role string, parts []ContentPart) Message {
	raw, _ := json.Marshal(parts)
	return Message{Role: role, Content: raw}
}

func NewToolResultMessage(toolCallID, result string) Message {
	raw, _ := json.Marshal(result)
	return Message{Role: "tool", Content: raw, ToolCallID: toolCallID}
}

type ContentPart struct {
	Type       string      `json:"type"`
	Text       string      `json:"text,omitempty"`
	ImageURL   *ImageURL   `json:"image_url,omitempty"`
	InputAudio *InputAudio `json:"input_audio,omitempty"`
}

type ImageURL struct {
	URL string `json:"url"`
}

type InputAudio struct {
	Data   string `json:"data"`
	Format string `json:"format"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type AssistantMessage struct {
	Content          string
	ReasoningContent string
	ToolCalls        []ToolCall
}

func ensureToolCallID(tc *ToolCall, index int) {
	if tc.ID != "" {
		return
	}
	tc.ID = fmt.Sprintf("call_%d_%d", time.Now().UnixNano(), index)
}

type ChatCompletionResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Error *APIError `json:"error"`
}

type APIError struct {
	Message string `json:"message"`
}

type Client struct {
	modelConfig *model.Model
	config      *config.Config
	httpClient  *http.Client
}

func NewClient(m *model.Model, cfg *config.Config) *Client {
	return &Client{
		modelConfig: m,
		config:      cfg,
		httpClient:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *Client) modelID() string {
	if c.modelConfig.Model != "" {
		return c.modelConfig.Model
	}
	return c.modelConfig.Name
}

func (c *Client) SetHTTPClient(client *http.Client) {
	c.httpClient = client
}

func (c *Client) Chat(text string, toolDefs []json.RawMessage) (string, error) {
	req := &ChatCompletionRequest{
		Model: c.modelID(),
		Messages: []Message{
			NewTextMessage("user", text),
		},
		Tools: wrapTools(toolDefs),
	}
	return c.doChat(req)
}

func (c *Client) ChatWithImages(text string, imagePaths []string, toolDefs []json.RawMessage) (string, error) {
	parts := []ContentPart{
		{Type: "text", Text: text},
	}
	for _, path := range imagePaths {
		fullPath := c.resolvePath(path, c.config.WorkPath.Source.Pic)
		dataURL, err := c.fileToDataURL(fullPath)
		if err != nil {
			return "", fmt.Errorf("process image %s: %w", path, err)
		}
		parts = append(parts, ContentPart{
			Type:     "image_url",
			ImageURL: &ImageURL{URL: dataURL},
		})
	}
	req := &ChatCompletionRequest{
		Model: c.modelID(),
		Messages: []Message{
			NewMultimodalMessage("user", parts),
		},
		Tools: wrapTools(toolDefs),
	}
	return c.doChat(req)
}

func (c *Client) ChatWithAudio(text string, audioPaths []string, toolDefs []json.RawMessage) (string, error) {
	parts := []ContentPart{
		{Type: "text", Text: text},
	}
	for _, path := range audioPaths {
		fullPath := c.resolvePath(path, c.config.WorkPath.Source.Sound)
		dataURL, format, err := c.audioToBase64(fullPath)
		if err != nil {
			return "", fmt.Errorf("process audio %s: %w", path, err)
		}
		parts = append(parts, ContentPart{
			Type: "input_audio",
			InputAudio: &InputAudio{
				Data:   dataURL,
				Format: format,
			},
		})
	}
	req := &ChatCompletionRequest{
		Model: c.modelID(),
		Messages: []Message{
			NewMultimodalMessage("user", parts),
		},
		Tools: wrapTools(toolDefs),
	}
	return c.doChat(req)
}

type MediaFile struct {
	Path string
	Type string // "image", "audio", "video"
}

func (c *Client) ChatMultimodal(text string, mediaFiles []MediaFile, toolDefs []json.RawMessage) (string, error) {
	parts := []ContentPart{
		{Type: "text", Text: text},
	}
	for _, mf := range mediaFiles {
		switch mf.Type {
		case "image":
			fullPath := c.resolvePath(mf.Path, c.config.WorkPath.Source.Pic)
			dataURL, err := c.fileToDataURL(fullPath)
			if err != nil {
				return "", fmt.Errorf("process image %s: %w", mf.Path, err)
			}
			parts = append(parts, ContentPart{
				Type:     "image_url",
				ImageURL: &ImageURL{URL: dataURL},
			})
		case "audio":
			fullPath := c.resolvePath(mf.Path, c.config.WorkPath.Source.Sound)
			dataURL, format, err := c.audioToBase64(fullPath)
			if err != nil {
				return "", fmt.Errorf("process audio %s: %w", mf.Path, err)
			}
			parts = append(parts, ContentPart{
				Type: "input_audio",
				InputAudio: &InputAudio{
					Data:   dataURL,
					Format: format,
				},
			})
		case "video":
			return "", fmt.Errorf("video files are not supported directly; please extract frames as images")
		default:
			return "", fmt.Errorf("unsupported media type: %s", mf.Type)
		}
	}
	req := &ChatCompletionRequest{
		Model: c.modelID(),
		Messages: []Message{
			NewMultimodalMessage("user", parts),
		},
		Tools: wrapTools(toolDefs),
	}
	return c.doChat(req)
}

func (c *Client) ChatStream(text string, toolDefs []json.RawMessage) (<-chan string, <-chan error) {
	return c.streamChat([]Message{NewTextMessage("user", text)}, wrapTools(toolDefs))
}

func (c *Client) ChatStreamWithImages(text string, imagePaths []string, toolDefs []json.RawMessage) (<-chan string, <-chan error) {
	parts := []ContentPart{{Type: "text", Text: text}}
	for _, path := range imagePaths {
		fullPath := c.resolvePath(path, c.config.WorkPath.Source.Pic)
		dataURL, err := c.fileToDataURL(fullPath)
		if err != nil {
			errCh := make(chan error, 1)
			errCh <- fmt.Errorf("process image %s: %w", path, err)
			close(errCh)
			return nil, errCh
		}
		parts = append(parts, ContentPart{
			Type:     "image_url",
			ImageURL: &ImageURL{URL: dataURL},
		})
	}
	return c.streamChat([]Message{NewMultimodalMessage("user", parts)}, wrapTools(toolDefs))
}

func (c *Client) ChatStreamWithAudio(text string, audioPaths []string, toolDefs []json.RawMessage) (<-chan string, <-chan error) {
	parts := []ContentPart{{Type: "text", Text: text}}
	for _, path := range audioPaths {
		fullPath := c.resolvePath(path, c.config.WorkPath.Source.Sound)
		dataURL, format, err := c.audioToBase64(fullPath)
		if err != nil {
			errCh := make(chan error, 1)
			errCh <- fmt.Errorf("process audio %s: %w", path, err)
			close(errCh)
			return nil, errCh
		}
		parts = append(parts, ContentPart{
			Type: "input_audio",
			InputAudio: &InputAudio{
				Data:   dataURL,
				Format: format,
			},
		})
	}
	return c.streamChat([]Message{NewMultimodalMessage("user", parts)}, wrapTools(toolDefs))
}

func (c *Client) ChatStreamV2(messages []Message, toolDefs []json.RawMessage) (
	<-chan string, <-chan AssistantMessage, <-chan error,
) {
	return c.streamChatV2(messages, wrapTools(toolDefs))
}

func wrapTools(toolDefs []json.RawMessage) []Tool {
	if len(toolDefs) == 0 {
		return nil
	}
	tools := make([]Tool, 0, len(toolDefs))
	for _, def := range toolDefs {
		var cfg struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Parameters  json.RawMessage `json:"parameters"`
			Strict      bool            `json:"strict"`
		}
		if err := json.Unmarshal(def, &cfg); err != nil {
			continue
		}
		fn := map[string]any{
			"name":        cfg.Name,
			"description": cfg.Description,
			"parameters":  cfg.Parameters,
		}
		if cfg.Strict {
			fn["strict"] = true
		}
		fnRaw, _ := json.Marshal(fn)
		tools = append(tools, Tool{Type: "function", Function: fnRaw})
	}
	return tools
}

func wrapResponsesTools(toolDefs []json.RawMessage) []Tool {
	if len(toolDefs) == 0 {
		return nil
	}
	tools := make([]Tool, 0, len(toolDefs))
	for _, def := range toolDefs {
		tools = append(tools, Tool{Type: "function", Function: def})
	}
	return tools
}

func (c *Client) resolvePath(path, workPath string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workPath, path)
}

func (c *Client) fileToDataURL(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	mimeType := mime.TypeByExtension(filepath.Ext(path))
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, b64), nil
}

func (c *Client) audioToBase64(path string) (string, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	if ext == "" {
		ext = "mp3"
	}
	return base64.StdEncoding.EncodeToString(data), ext, nil
}

func (c *Client) doChat(req *ChatCompletionRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequest("POST", c.modelConfig.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.modelConfig.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.modelConfig.APIKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatCompletionResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", err
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("API error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return c.extractText(chatResp.Choices[0].Message.Content)
}

func (c *Client) extractText(raw json.RawMessage) (string, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}

	var parts []ContentPart
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", fmt.Errorf("unrecognized content format: %w", err)
	}

	var texts []string
	for _, part := range parts {
		if part.Type == "text" || part.Type == "" {
			texts = append(texts, part.Text)
		}
	}
	return strings.Join(texts, ""), nil
}

func (c *Client) streamChat(messages []Message, tools []Tool) (<-chan string, <-chan error) {
	textCh := make(chan string)
	errCh := make(chan error, 1)

	go func() {
		defer close(textCh)
		defer close(errCh)

		req := &ChatCompletionRequest{
			Model:    c.modelID(),
			Messages: messages,
			Tools:    tools,
			Stream:   true,
		}
		body, err := json.Marshal(req)
		if err != nil {
			errCh <- err
			return
		}

		httpReq, err := http.NewRequest("POST", c.modelConfig.BaseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			errCh <- err
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")
		if c.modelConfig.APIKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.modelConfig.APIKey)
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
			if line == "" || line == "data: [DONE]" {
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content          string `json:"content"`
						ReasoningContent string `json:"reasoning_content"`
					} `json:"delta"`
				} `json:"choices"`
				Error *APIError `json:"error"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if chunk.Error != nil {
				errCh <- fmt.Errorf("stream error: %s", chunk.Error.Message)
				return
			}

			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta
				if delta.Content != "" {
					textCh <- delta.Content
				}
				if delta.ReasoningContent != "" {
					textCh <- delta.ReasoningContent
				}
			}
		}
	}()

	return textCh, errCh
}

func (c *Client) streamChatV2(messages []Message, tools []Tool) (
	<-chan string, <-chan AssistantMessage, <-chan error,
) {
	textCh := make(chan string)
	resultCh := make(chan AssistantMessage, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(textCh)
		defer close(resultCh)
		defer close(errCh)

		req := &ChatCompletionRequest{
			Model:    c.modelID(),
			Messages: messages,
			Tools:    tools,
			Stream:   true,
		}
		if len(tools) > 0 {
			req.ToolChoice = "auto"
		}
		body, err := json.Marshal(req)
		if err != nil {
			errCh <- err
			return
		}

		httpReq, err := http.NewRequest("POST", c.modelConfig.BaseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			errCh <- err
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")
		if c.modelConfig.APIKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.modelConfig.APIKey)
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
		var reasoningContent strings.Builder
		inReasoning := false
		pending := make(map[int]*ToolCall)

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					if inReasoning {
						inReasoning = false
						textCh <- "</reasoning>"
					}
					break
				}
				errCh <- err
				return
			}

			line = strings.TrimSpace(line)
			if line == "" || line == "data: [DONE]" {
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content          string `json:"content"`
						ReasoningContent string `json:"reasoning_content"`
						ToolCalls        []struct {
							Index    int    `json:"index"`
							ID       string `json:"id"`
							Type     string `json:"type"`
							Function struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							} `json:"function"`
						} `json:"tool_calls"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
				Error *APIError `json:"error"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			if chunk.Error != nil {
				errCh <- fmt.Errorf("stream error: %s", chunk.Error.Message)
				return
			}

			if len(chunk.Choices) == 0 {
				continue
			}

			choice := chunk.Choices[0]

			if choice.Delta.ReasoningContent != "" {
				if !inReasoning {
					inReasoning = true
					textCh <- "<reasoning>"
				}
				reasoningContent.WriteString(choice.Delta.ReasoningContent)
				textCh <- choice.Delta.ReasoningContent
			}
			if choice.Delta.Content != "" {
				if inReasoning {
					inReasoning = false
					textCh <- "</reasoning>"
				}
				content.WriteString(choice.Delta.Content)
				textCh <- choice.Delta.Content
			}

			for _, tc := range choice.Delta.ToolCalls {
				p, ok := pending[tc.Index]
				if !ok {
					p = &ToolCall{}
					pending[tc.Index] = p
				}
				if tc.ID != "" {
					p.ID = tc.ID
				}
				if tc.Type != "" {
					p.Type = tc.Type
				}
				if tc.Function.Name != "" {
					p.Function.Name = tc.Function.Name
				}
				if tc.Function.Arguments != "" {
					p.Function.Arguments += tc.Function.Arguments
				}
			}

			if choice.FinishReason != "" {
				if inReasoning {
					inReasoning = false
					textCh <- "</reasoning>"
				}
				break
			}
		}

		var msg AssistantMessage
		msg.Content = content.String()
		msg.ReasoningContent = reasoningContent.String()
		for i := 0; i < len(pending); i++ {
			if p, ok := pending[i]; ok {
				ensureToolCallID(p, i)
				msg.ToolCalls = append(msg.ToolCalls, *p)
			}
		}
		resultCh <- msg
	}()

	return textCh, resultCh, errCh
}
