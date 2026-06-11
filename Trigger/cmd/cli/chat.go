package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Lyoomu/TAC/Trigger/internal/model"
	"github.com/Lyoomu/TAC/Trigger/internal/session"
	"github.com/Lyoomu/TAC/Trigger/internal/tool"
	pb "github.com/Lyoomu/TAC/proto"
)

var sessionMgr *session.Manager

func initSessionManager() error {
	if sessionMgr != nil {
		return nil
	}
	sessionMgr = session.NewManager()
	return nil
}

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start a chat session with a loaded role",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initServerEngine(); err != nil {
			return err
		}
		if err := initSessionManager(); err != nil {
			return err
		}

		roleKey, _ := cmd.Flags().GetString("role")
		sessionID, _ := cmd.Flags().GetString("session")
		messageMode, _ := cmd.Flags().GetString("message-mode")

		if roleKey == "" {
			return fmt.Errorf("--role is required (format: ServerName-RoleName)")
		}

		var matchedRole *model.LoadedRole
		for _, r := range srvEngine.GetLoadedRoles() {
			key := r.ServerName + "-" + r.RoleName
			if key == roleKey {
				matchedRole = &r
				break
			}
		}
		if matchedRole == nil {
			return fmt.Errorf("role '%s' not found in loaded roles, use 'server load' first", roleKey)
		}

		serverDisplayName := matchedRole.ServerName
		roleName := matchedRole.RoleName

		serverConn, err := srvEngine.GetByDisplayName(serverDisplayName)
		if err != nil {
			return fmt.Errorf("server '%s' not found: %w", serverDisplayName, err)
		}

		client, err := srvEngine.GetClient(serverConn.Address)
		if err != nil {
			return fmt.Errorf("connect: %w", err)
		}
		defer srvEngine.CloseAll()

		sess, err := sessionMgr.GetOrCreate("chat", serverDisplayName, roleName, sessionID)
		if err != nil {
			return fmt.Errorf("session: %w", err)
		}

		fmt.Printf("\nStarting chat with \x1b[36m%s\x1b[0m-\x1b[33m%s\x1b[0m\n", serverDisplayName, roleName)
		fmt.Printf("Session: %s | Messages: %d\n", sess.ID, len(sess.Messages))
		fmt.Println("Type your message, 'exit' to quit, 'interrupt' to stop generation")
		fmt.Println()

		ctx := context.Background()
		stream, err := client.Chat.Chat(ctx)
		if err != nil {
			return fmt.Errorf("start chat stream: %w", err)
		}

		if len(sess.Messages) > 0 {
			for _, msg := range sess.Messages {
				_ = stream.Send(&pb.ChatMessage{
					Role:        msg.Role,
					Content:     msg.Content,
					MessageType: "history",
					IsHistory:   true,
					RoleName:    roleName,
					SessionId:   sess.ID,
				})
			}
		}

		recvDone := make(chan error, 1)
		go func() {
			for {
				resp, err := stream.Recv()
				if err == io.EOF {
					recvDone <- nil
					return
				}
				if err != nil {
					recvDone <- err
					return
				}

				switch resp.MessageType {
				case "text":
					if resp.Content == "" && resp.EndOfTurn {
						fmt.Println()

					} else {
						fmt.Print(resp.Content)
						if resp.EndOfTurn {
							fmt.Println()
						}
					}
				case "tool_call":
					fmt.Printf("\n\n[Tool Call] %s(%s)\n", resp.ToolName, resp.ToolArguments)

					fmt.Print("Execute this tool? [y/n]: ")
					reader := bufio.NewReader(os.Stdin)
					line, _ := reader.ReadString('\n')
					line = strings.TrimSpace(strings.ToLower(line))
					if line == "y" || line == "yes" {
						var result string
						toolInfo, ok := tool.FindLoadedTool(srvEngine.GetLoadedRoles(), serverDisplayName, roleName, resp.ToolName)
						if !ok {
							errJSON, _ := json.Marshal(map[string]string{"error": "tool not loaded: " + resp.ToolName})
							result = string(errJSON)
						} else {
							workDir := ""
						if err := initWorkspaceEngine(); err == nil {
							if ws, err := wsEngine.GetActive(); err == nil {
								workDir = ws.Path
							}
						}
						res, err := tool.Execute(toolInfo, resp.ToolArguments, workDir)
							if err != nil {
								errJSON, _ := json.Marshal(map[string]string{"error": err.Error()})
								result = string(errJSON)
							} else {
								result = res
							}
						}
						_ = stream.Send(&pb.ChatMessage{
							MessageType: "tool_result",
							ToolCallId:  resp.ToolCallId,
							ToolResult:  result,
							RoleName:    roleName,
							SessionId:   sess.ID,
						})
					} else {
						_ = stream.Send(&pb.ChatMessage{
							MessageType: "tool_result",
							ToolCallId:  resp.ToolCallId,
							ToolResult:  `{"status": "cancelled"}`,
							RoleName:    roleName,
							SessionId:   sess.ID,
						})
					}

				case "error":
					fmt.Printf("\n[Error] %s\n", resp.ErrorMessage)
				}

				if resp.EndOfTurn {
					fmt.Print("\n> ")
				}
			}
		}()

		scanner := bufio.NewScanner(os.Stdin)
		fmt.Print("> ")
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				fmt.Print("> ")
				continue
			}
			if line == "exit" || line == "quit" {
				_ = stream.Send(&pb.ChatMessage{
					MessageType: "control",
					Content:     "end_session",
					RoleName:    roleName,
					SessionId:   sess.ID,
				})
				break
			}
			if line == "interrupt" {
				_ = stream.Send(&pb.ChatMessage{
					MessageType: "control",
					Content:     "interrupt",
					RoleName:    roleName,
					SessionId:   sess.ID,
				})
				fmt.Print("> ")
				continue
			}

			_ = sessionMgr.AppendMessage(sess, "user", line)

			textMsg := &pb.ChatMessage{
				MessageType: "text",
				Content:     line,
				Role:        "user",
				RoleName:    roleName,
				SessionId:   sess.ID,
			}
			if messageMode != "" {
				textMsg.MessageMode = messageMode
			}
			err := stream.Send(textMsg)
			if err != nil {
				fmt.Printf("send error: %v\n", err)
				break
			}
		}

		select {
		case err := <-recvDone:
			if err != nil {
				fmt.Printf("\nchat ended with error: %v\n", err)
			}
		case <-time.After(2 * time.Second):
		}

		if err := sessionMgr.Save(sess); err != nil {
			fmt.Printf("warning: failed to save session: %v\n", err)
		}

		fmt.Printf("\nSession saved: %s\n", sess.ID)
		return nil
	},
}

func init() {
	chatCmd.Flags().StringP("role", "r", "", "role to chat with (format: ServerName-RoleName)")
	chatCmd.Flags().StringP("session", "s", "", "session ID to resume (auto-generated if empty)")
	chatCmd.Flags().StringP("message-mode", "m", "", "message mode override: queue, reject, or interrupt (default: from Role)")
	chatCmd.MarkFlagRequired("role")

	rootCmd.AddCommand(chatCmd)
}
