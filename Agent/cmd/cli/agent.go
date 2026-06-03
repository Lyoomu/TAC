package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Lyoomu/TAC/Agent/internal/agent"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Run an agent chat session",
}

var agentChatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat with an agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		roleName, _ := cmd.Flags().GetString("role")
		modeStr, _ := cmd.Flags().GetString("mode")
		initMsg, _ := cmd.Flags().GetString("message")

		mode := agent.MessageMode(modeStr)
		if modeStr == "" {
			role, err := appCtx.RoleEngine.Get(roleName)
			if err != nil {
				return err
			}
			mode = agent.MessageMode(role.MessageMode)
		}

		output, err := appCtx.AgentManager.RunAgent(roleName, nil, initMsg, nil, mode)
		if err != nil {
			return err
		}
		defer output.Stop()

		go func() {
			for chunk := range output.StreamCh {
				fmt.Print(chunk)
			}
		}()

		go func() {
			for err := range output.ErrCh {
				fmt.Fprintf(os.Stderr, "\n[error] %v\n", err)
			}
		}()

		toolDone := make(chan struct{})
		go func() {
			defer close(toolDone)
			reader := bufio.NewReader(os.Stdin)
			for tc := range output.ToolCallCh {
				fmt.Printf("\n[tool call] id=%s name=%s args=%s\n", tc.ID, tc.Name, tc.Args)
				fmt.Println("paste tool result, end with single line '.':")
				var lines []string
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						break
					}
					if strings.TrimSpace(line) == "." {
						break
					}
					lines = append(lines, line)
				}
				output.SubmitToolResult(tc.ID, strings.Join(lines, ""))
			}
		}()

		input := bufio.NewReader(os.Stdin)
		fmt.Printf("agent chat (id=%s, role=%s, mode=%s) — type 'exit' to quit\n", output.ID, roleName, mode)
		for {
			fmt.Print("\nyou> ")
			line, err := input.ReadString('\n')
			if err != nil {
				break
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if line == "exit" || line == "quit" {
				break
			}
			output.SendMessage(line, mode)
		}

		return nil
	},
}

func init() {
	agentChatCmd.Flags().StringP("role", "r", "", "role name (required)")
	agentChatCmd.Flags().StringP("mode", "m", "", "message mode override: queue, reject, or interrupt (default: from role)")
	agentChatCmd.Flags().String("message", "", "initial user message")
	agentChatCmd.MarkFlagRequired("role")

	agentCmd.AddCommand(agentChatCmd)
	rootCmd.AddCommand(agentCmd)
}
