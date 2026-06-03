package main

import (
	"fmt"
	"os"
	"os/exec"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/Lyoomu/TAC/Trigger/internal/daemon"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the Trigger daemon",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Trigger daemon in background",
	RunE: func(cmd *cobra.Command, args []string) error {
		if daemon.IsDaemonRunning() {
			fmt.Println("daemon is already running")
			return nil
		}

		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("get executable path: %w", err)
		}

		child := exec.Command(exe, "--daemon-internal")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			return fmt.Errorf("start daemon: %w", err)
		}

		fmt.Printf("daemon started (PID: %d)\n", child.Process.Pid)
		return nil
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the Trigger daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := daemon.NewClient()
		if err != nil {
			return err
		}

		if _, err := client.Shutdown(); err != nil {
			return fmt.Errorf("shutdown daemon: %w", err)
		}
		fmt.Println("daemon stopped")
		return nil
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !daemon.IsDaemonRunning() {
			fmt.Println("daemon is not running")
			return nil
		}

		client, err := daemon.NewClient()
		if err != nil {
			return err
		}

		resp, err := client.GetDaemonStatus()
		if err != nil {
			return err
		}

		fmt.Printf("Status:     running\n")
		fmt.Printf("Workspace:  %s\n", resp.Workspace)
		fmt.Printf("Triggers:   %d (%d running)\n", resp.TriggerCount, resp.RunningTriggerCount)
		fmt.Printf("Events:     %d\n", resp.EventCount)
		fmt.Printf("Uptime:     %s\n", resp.Uptime)
		return nil
	},
}

var daemonListCmd = &cobra.Command{
	Use:   "list",
	Short: "List triggers from daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !daemon.IsDaemonRunning() {
			fmt.Println("daemon is not running")
			return nil
		}

		client, err := daemon.NewClient()
		if err != nil {
			return err
		}

		resp, err := client.ListTriggers()
		if err != nil {
			return err
		}

		if len(resp.Triggers) == 0 {
			fmt.Println("no triggers")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tTYPE\tEVENTS\tSTATUS")
		for _, t := range resp.Triggers {
			status := "stopped"
			if t.Running {
				status = "running"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", t.Id, t.Name, t.TriggerType, t.EventCount, status)
		}
		w.Flush()
		return nil
	},
}

func runDaemonInternal() error {
	srv, err := daemon.NewServer()
	if err != nil {
		return fmt.Errorf("create daemon server: %w", err)
	}

	addr, err := srv.Start()
	if err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	fmt.Printf("Daemon listening on %s\n", addr)

	select {}
}

func init() {
	daemonCmd.AddCommand(daemonStartCmd, daemonStopCmd, daemonStatusCmd, daemonListCmd)
	rootCmd.AddCommand(daemonCmd)
}
