package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/shlex"
	"github.com/spf13/cobra"

	srv "github.com/Lyoomu/TAC/Trigger/internal/server"
	"github.com/Lyoomu/TAC/Trigger/internal/session"
	"github.com/Lyoomu/TAC/Trigger/internal/tui"
	"github.com/Lyoomu/TAC/Trigger/internal/workspace"
)

var rootCmd = &cobra.Command{
	Use:   "TAC-Trigger",
	Short: "TAC Trigger - Agent service gateway",
	Long:  "TAC Trigger is the service entry point that exposes Agent capabilities via network APIs.",
}

func scanLocalAgent() {
	fmt.Println("[Scan] Scanning for local Agent Servers on ports 50051-50055...")
	ports := []int{50051, 50052, 50053, 50054, 50055}
	var wg sync.WaitGroup
	var foundPorts []int
	var mu sync.Mutex

	for _, port := range ports {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()

			address := fmt.Sprintf("127.0.0.1:%d", p)
			conn, err := net.DialTimeout("tcp", address, 300*time.Millisecond)
			if err == nil {
				conn.Close()
				mu.Lock()
				foundPorts = append(foundPorts, p)
				mu.Unlock()
				return
			}

			addressIPv6 := fmt.Sprintf("[::1]:%d", p)
			conn, err = net.DialTimeout("tcp", addressIPv6, 300*time.Millisecond)
			if err == nil {
				conn.Close()
				mu.Lock()
				foundPorts = append(foundPorts, p)
				mu.Unlock()
			}
		}(port)
	}
	wg.Wait()

	if len(foundPorts) > 0 {
		fmt.Printf("[Scan] Found active local Agent Server(s) on port(s): %v\n", foundPorts)
	} else {
		fmt.Println("[Scan] No active local Agent Server found on default ports (50051-50055).")
	}
	fmt.Println()
}

func main() {

	if len(os.Args) > 1 && os.Args[1] == "--daemon-internal" {
		if err := runDaemonInternal(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if len(os.Args) == 1 {
		if err := runTUI(); err != nil {
			fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
			fmt.Println("Falling back to REPL mode...")
			runREPL()
		}
		return
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runTUI() error {

	srvEngine := srv.NewEngine()
	if err := srvEngine.Load(); err != nil {
		return fmt.Errorf("load servers: %w", err)
	}
	sessionManager := session.NewManager()
	wsEngine := workspace.NewEngine()
	if err := wsEngine.Load(); err != nil {
		return fmt.Errorf("load workspaces: %w", err)
	}
	if err := ensureCurrentDirectoryWorkspace(wsEngine); err != nil {
		return err
	}

	ctx := &tui.AppContext{
		ServerEngine:    srvEngine,
		SessionManager:  sessionManager,
		WorkspaceEngine: wsEngine,
		CommandFunc:     runCommand,
	}
	return tui.Run(ctx)
}

func ensureCurrentDirectoryWorkspace(wsEngine *workspace.Engine) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return fmt.Errorf("resolve current directory: %w", err)
	}

	for _, ws := range wsEngine.List() {
		absPath, err := filepath.Abs(ws.Path)
		if err == nil && filepath.Clean(absPath) == filepath.Clean(absCwd) {
			return nil
		}
	}

	if !confirmInteractive(fmt.Sprintf("current directory is not bound as a TAC workspace: %s. Bind it now?", absCwd)) {
		return nil
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("workspace logical name: ")
		name, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read workspace name: %w", err)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			fmt.Println("workspace name cannot be empty")
			continue
		}
		if err := wsEngine.Bind(name, absCwd); err != nil {
			switch err {
			case workspace.ErrWorkspaceExists:
				fmt.Println("workspace name already exists")
			case workspace.ErrPathAlreadyBound:
				return nil
			default:
				return fmt.Errorf("bind current directory workspace: %w", err)
			}
			continue
		}
		fmt.Printf("bound: %s -> %s\n", name, absCwd)
		return nil
	}
}

func runCommand(cmd string) (string, error) {
	args, err := shlex.Split(cmd)
	if err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	if len(args) == 0 {
		return "", nil
	}

	oldOut := rootCmd.OutOrStdout()
	oldErr := rootCmd.ErrOrStderr()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	defer func() {
		rootCmd.SetOut(oldOut)
		rootCmd.SetErr(oldErr)
	}()

	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		output := buf.String()
		if output == "" {
			output = err.Error()
		}
		return output, nil
	}
	return buf.String(), nil
}
