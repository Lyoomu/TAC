package main

import (
	"fmt"
	"os"
	"time"

	"github.com/Lyoomu/TAC/Agent/internal/config"
	"github.com/Lyoomu/TAC/Agent/internal/model"
	"github.com/Lyoomu/TAC/Agent/internal/models/llm"
)

func main() {
	fmt.Println("=== Anthropic API 测试 ===")

	cfg, _ := config.Load("properties.yaml")

	apiKey := os.Getenv("TAC_TEST_API_KEY")
	baseURL := os.Getenv("TAC_TEST_BASE_URL")
	modelID := os.Getenv("TAC_TEST_MODEL")
	if apiKey == "" || baseURL == "" || modelID == "" {
		fmt.Fprintln(os.Stderr, "Please set TAC_TEST_API_KEY, TAC_TEST_BASE_URL, TAC_TEST_MODEL environment variables")
		os.Exit(1)
	}
	anthropicModel := &model.Model{
		Name:    "anthropic-test",
		Model:   modelID,
		BaseURL: baseURL,
		APIKey:  apiKey,
	}
	client := llm.NewAnthropicClient(anthropicModel, cfg)

	fmt.Println("\n[TEST 1] 简单对话")
	testChat(client, []llm.Message{
		llm.NewTextMessage("user", "你好，请介绍一下自己"),
	})

	fmt.Println("\n[TEST 2] 带 system prompt 的对话")
	testChat(client, []llm.Message{
		llm.NewTextMessage("system", "你是一个专业的天气助手，只回答天气相关问题。"),
		llm.NewTextMessage("user", "北京今天天气如何？"),
	})

	fmt.Println("\n=== Anthropic API 测试完成 ===")
}

func testChat(client *llm.AnthropicClient, messages []llm.Message) {
	streamCh, resultCh, errCh := client.ChatStreamV2(messages, nil)

	go func() {
		for err := range errCh {
			fmt.Printf("\n[ERROR] %v\n", err)
		}
	}()

	go func() {
		result := <-resultCh
		fmt.Printf("\n[RESULT] contentLen=%d toolCalls=%d\n", len(result.Content), len(result.ToolCalls))
	}()

	for chunk := range streamCh {
		fmt.Print(chunk)
	}

	fmt.Println("\n[Stream ended]")
	time.Sleep(1 * time.Second)
}
