package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/google/shlex"
)

func runREPL() {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("TAC CLI (REPL) — type 'help' for commands, 'exit' to quit")
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" {
			break
		}
		if line == "help" {
			rootCmd.SetArgs([]string{"--help"})
			_ = rootCmd.Execute()
			continue
		}

		args, err := shlex.Split(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
			continue
		}
		rootCmd.SetArgs(args)
		if err := rootCmd.Execute(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
	}
}
