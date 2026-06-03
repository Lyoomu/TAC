package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/Lyoomu/TAC/Agent/internal/server"
	"github.com/Lyoomu/TAC/Agent/internal/tui"
	"github.com/google/shlex"
)

func main() {
	if len(os.Args) == 1 {
		if err := initContext(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer closeContext()
		runTUI()
		return
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer closeContext()
}

func runTUI() {
	srv := server.New(appCtx.Config, appCtx.RoleEngine, appCtx.ToolEngine, appCtx.AgentManager)
	ctx := &tui.AppContext{
		Config:          appCtx.Config,
		ComponentEngine: appCtx.ComponentEngine,
		ModelsEngine:    appCtx.ModelsEngine,
		RoleEngine:      appCtx.RoleEngine,
		ToolEngine:      appCtx.ToolEngine,
		AgentManager:    appCtx.AgentManager,
		Server:          srv,
		CommandFunc:     runCommand,
	}
	if err := tui.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)

		fmt.Println("Falling back to REPL mode...")
		runREPL()
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
