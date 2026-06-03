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

type ResponsesRequest struct {
	Model              string          `json:"model"`
	Input              []ResponseInput `json:"input"`
	Tools              []Tool          `json:"tools,omitempty"`
	Stream             bool            `json:"stream,omitempty"`
	PreviousResponseID string          `json:"previous_response_id,omitempty"`
}

type ResponseInput struct {
	Role    string            `json:"role"`
	Content []ResponseContent `json:"content"`
}

type ResponseContent struct {
	Type       string      `json:"type"`
	Text       string      `json:"text,omitempty"`
	ImageURL   string      `json:"image_url,omitempty"`
	InputAudio *InputAudio `json:"input_audio,omitempty"`
	CallID     string      `json:"call_id,omitempty"`
	Name       string      `json:"name,omitempty"`
	Arguments  string      `json:"arguments,omitempty"`
	Output     string      `json:"output,omitempty"`
}

type ResponsesResponse struct {
	ID     string         `json:"id"`
	Output []ResponseItem `json:"output"`
	Error  *APIError      `json:"error"`
}

type ResponseItem struct {
	Type      string            `json:"type"`
	Role      string            `json:"role,omitempty"`
	Status    string            `json:"status,omitempty"`
	CallID    string            `json:"call_id,omitempty"`
	Name      string            `json:"name,omitempty"`
	Arguments string            `json:"arguments,omitempty"`
	Content   []ResponseContent `json:"content,omitempty"`
}

type ResponseStreamEvent struct {
	Type         string             `json:"type"`
	OutputIndex  int                `json:"output_index,omitempty"`
	ContentIndex int                `json:"content_index,omitempty"`
	ItemIndex    int                `json:"item_index,omitempty"`
	Delta        string             `json:"delta,omitempty"`
	Item         *ResponseItem      `json:"item,omitempty"`
	Part         *ResponseContent   `json:"part,omitempty"`
	Response     *ResponsesResponse `json:"response,omitempty"`
}

type ResponsesClient struct {
	modelConfig *model.Model
	config      *config.Config
	httpClient  *http.Client
}

func NewResponsesClient(m *model.Model, cfg *config.Config) *ResponsesClient {
	return &ResponsesClient{
		modelConfig: m,
		config:      cfg,
		httpClient:  &http.Client{Timeout: 120 * time.Second},
	}
}

func (c *ResponsesClient) modelID() string {
	if c.modelConfig.Model != "" {
		return c.modelConfig.Model
	}
	return c.modelConfig.Name
}

func (c *ResponsesClient) SetHTTPClient(client *http.Client) {
	c.httpClient = client
}

func (c *ResponsesClient) ChatStreamV2(messages []Message, toolDefs []json.RawMessage) (
	<-chan string, <-chan AssistantMessage, <-chan error,
) {
	textCh := make(chan string)
	resultCh := make(chan AssistantMessage, 1)
	errCh := make(chan error, 1)

	go func() {
		defer close(textCh)
		defer close(resultCh)
		defer close(errCh)

		input := messagesToInput(messages)
		req := &ResponsesRequest{
			Model:  c.modelID(),
			Input:  input,
			Tools:  wrapResponsesTools(toolDefs),
			Stream: true,
		}

		body, err := json.Marshal(req)
		if err != nil {
			errCh <- err
			return
		}

		httpReq, err := http.NewRequest("POST", c.modelConfig.BaseURL+"/responses", bytes.NewReader(body))
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
		pending := make(map[int]*ToolCall)

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

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				continue
			}

			var event ResponseStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			switch event.Type {
			case "response.output_text.delta":
				if event.Delta != "" {
					content.WriteString(event.Delta)
					textCh <- event.Delta
				}
			case "response.reasoning_content.delta":
				if event.Delta != "" {
					content.WriteString(event.Delta)
					textCh <- event.Delta
				}

			case "response.function_call_arguments.delta":
				if tc, ok := pending[event.OutputIndex]; ok {
					tc.Function.Arguments += event.Delta
				}

			case "response.output_item.added":
				if event.Item != nil && event.Item.Type == "function_call" {
					pending[event.OutputIndex] = &ToolCall{
						ID:   event.Item.CallID,
						Type: "function",
						Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{
							Name:      event.Item.Name,
							Arguments: event.Item.Arguments,
						},
					}
				}

			case "response.output_item.done":
				if event.Item != nil && event.Item.Type == "function_call" {
					if tc, ok := pending[event.OutputIndex]; ok {
						tc.Function.Arguments = event.Item.Arguments
					}
				}

			case "response.completed":
				if event.Response != nil {

					var msg AssistantMessage
					msg.Content = content.String()
					for i := 0; i < len(pending); i++ {
						if tc, ok := pending[i]; ok {
							ensureToolCallID(tc, i)
							msg.ToolCalls = append(msg.ToolCalls, *tc)
						}
					}
					resultCh <- msg
					return
				}
			}
		}

		var msg AssistantMessage
		msg.Content = content.String()
		idx := 0
		for _, tc := range pending {
			ensureToolCallID(tc, idx)
			msg.ToolCalls = append(msg.ToolCalls, *tc)
			idx++
		}
		resultCh <- msg
	}()

	return textCh, resultCh, errCh
}

func messagesToInput(messages []Message) []ResponseInput {
	var inputs []ResponseInput
	for _, msg := range messages {
		input := ResponseInput{Role: msg.Role}

		if msg.ToolCallID != "" {
			var text string
			json.Unmarshal(msg.Content, &text)
			input.Content = []ResponseContent{{
				Type:   "function_call_output",
				CallID: msg.ToolCallID,
				Output: text,
			}}
			inputs = append(inputs, input)
			continue
		}

		for _, tc := range msg.ToolCalls {
			input.Content = append(input.Content, ResponseContent{
				Type:      "function_call",
				CallID:    tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			})
		}

		if len(input.Content) > 0 {

			inputs = append(inputs, input)
			continue
		}

		var text string
		if err := json.Unmarshal(msg.Content, &text); err == nil {
			input.Content = append(input.Content, ResponseContent{
				Type: "input_text",
				Text: text,
			})
		} else {
			var parts []ContentPart
			if err := json.Unmarshal(msg.Content, &parts); err == nil {
				for _, part := range parts {
					switch part.Type {
					case "text":
						input.Content = append(input.Content, ResponseContent{
							Type: "input_text",
							Text: part.Text,
						})
					case "image_url":
						input.Content = append(input.Content, ResponseContent{
							Type:     "input_image",
							ImageURL: part.ImageURL.URL,
						})
					case "input_audio":
						input.Content = append(input.Content, ResponseContent{
							Type:       "input_audio",
							InputAudio: part.InputAudio,
						})
					}
				}
			}
		}

		inputs = append(inputs, input)
	}
	return inputs
}
