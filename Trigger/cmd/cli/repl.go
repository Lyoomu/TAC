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
	for {
		fmt.Print("trigger> ")
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
}

func confirmInteractive(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [y/n]: ", prompt)
		line, err := reader.ReadString('\n')
		if err != nil {
			return false
		}
		line = strings.TrimSpace(strings.ToLower(line))
		switch line {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		default:
			fmt.Println("please enter 'y' or 'n'")
		}
	}
}
