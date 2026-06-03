package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/Lyoomu/TAC/Trigger/internal/daemon"
	pb "github.com/Lyoomu/TAC/proto"
)

var triggerCmd = &cobra.Command{
	Use:   "trigger",
	Short: "Manage triggers",
}

func ensureDaemonOrInit() (*daemon.Client, error) {
	if daemon.IsDaemonRunning() {
		return daemon.NewClient()
	}
	return nil, fmt.Errorf("daemon is not running, use 'trigger daemon start' first")
}

var triggerCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new trigger",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := ensureDaemonOrInit()
		if err != nil {
			return err
		}

		id, _ := cmd.Flags().GetString("id")
		name, _ := cmd.Flags().GetString("name")
		triggerType, _ := cmd.Flags().GetString("type")
		description, _ := cmd.Flags().GetString("description")
		interval, _ := cmd.Flags().GetString("interval")
		cronExpr, _ := cmd.Flags().GetString("cron")
		watchPath, _ := cmd.Flags().GetString("watch-path")
		recursive, _ := cmd.Flags().GetBool("recursive")
		eventIDs, _ := cmd.Flags().GetStringSlice("event-ids")

		if name == "" || triggerType == "" {
			return fmt.Errorf("--name and --type are required")
		}

		resp, err := client.CreateTrigger(&pb.CreateTriggerRequest{
			Id:          id,
			Name:        name,
			TriggerType: triggerType,
			Description: description,
			Interval:    interval,
			CronExpr:    cronExpr,
			WatchPath:   watchPath,
			Recursive:   recursive,
			EventIds:    eventIDs,
		})
		if err != nil {
			return err
		}
		if resp.Success {
			fmt.Println(resp.Message)
		} else {
			fmt.Printf("failed: %s\n", resp.Message)
		}
		return nil
	},
}

var triggerUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update an existing trigger",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := ensureDaemonOrInit()
		if err != nil {
			return err
		}

		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		name, _ := cmd.Flags().GetString("name")
		description, _ := cmd.Flags().GetString("description")
		interval, _ := cmd.Flags().GetString("interval")
		cronExpr, _ := cmd.Flags().GetString("cron")
		watchPath, _ := cmd.Flags().GetString("watch-path")
		recursive, _ := cmd.Flags().GetBool("recursive")
		eventIDs, _ := cmd.Flags().GetStringSlice("event-ids")

		resp, err := client.UpdateTrigger(&pb.UpdateTriggerRequest{
			Id:          id,
			Name:        name,
			Description: description,
			Interval:    interval,
			CronExpr:    cronExpr,
			WatchPath:   watchPath,
			Recursive:   recursive,
			EventIds:    eventIDs,
		})
		if err != nil {
			return err
		}
		if resp.Success {
			fmt.Printf("trigger '%s' updated\n", id)
		} else {
			fmt.Printf("failed: %s\n", resp.Message)
		}
		return nil
	},
}

var triggerDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a trigger",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := ensureDaemonOrInit()
		if err != nil {
			return err
		}

		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		resp, err := client.DeleteTrigger(id)
		if err != nil {
			return err
		}
		if resp.Success {
			fmt.Printf("trigger '%s' deleted\n", id)
		} else {
			fmt.Printf("failed: %s\n", resp.Message)
		}
		return nil
	},
}

var triggerListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all triggers",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := ensureDaemonOrInit()
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

var triggerStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a trigger",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := ensureDaemonOrInit()
		if err != nil {
			return err
		}
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		resp, err := client.StartTrigger(id)
		if err != nil {
			return err
		}
		if resp.Success {
			fmt.Printf("trigger '%s' started\n", id)
		} else {
			fmt.Printf("failed: %s\n", resp.Message)
		}
		return nil
	},
}

var triggerStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a trigger",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := ensureDaemonOrInit()
		if err != nil {
			return err
		}
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		resp, err := client.StopTrigger(id)
		if err != nil {
			return err
		}
		if resp.Success {
			fmt.Printf("trigger '%s' stopped\n", id)
		} else {
			fmt.Printf("failed: %s\n", resp.Message)
		}
		return nil
	},
}

var triggerRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Manually run a direct trigger",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := ensureDaemonOrInit()
		if err != nil {
			return err
		}
		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		resp, err := client.RunTrigger(id)
		if err != nil {
			return err
		}
		if resp.Success {
			fmt.Printf("trigger '%s' executed\n", id)
		} else {
			fmt.Printf("failed: %s\n", resp.Message)
		}
		return nil
	},
}

func init() {
	triggerCreateCmd.Flags().StringP("id", "i", "", "trigger unique id (optional, generated if empty)")
	triggerCreateCmd.Flags().StringP("name", "n", "", "trigger name")
	triggerCreateCmd.Flags().StringP("type", "t", "", "trigger type: direct, periodic, edit")
	triggerCreateCmd.Flags().StringP("description", "d", "", "description")
	triggerCreateCmd.Flags().String("interval", "", "interval for periodic trigger (e.g. 5m)")
	triggerCreateCmd.Flags().String("cron", "", "cron expression for periodic trigger")
	triggerCreateCmd.Flags().String("watch-path", "", "watch path for edit trigger")
	triggerCreateCmd.Flags().Bool("recursive", false, "recursive watch for edit trigger")
	triggerCreateCmd.Flags().StringSlice("event-ids", nil, "bound event IDs")

	triggerUpdateCmd.Flags().StringP("id", "i", "", "trigger ID")
	triggerUpdateCmd.MarkFlagRequired("id")
	triggerUpdateCmd.Flags().StringP("name", "n", "", "trigger name")
	triggerUpdateCmd.Flags().StringP("description", "d", "", "description")
	triggerUpdateCmd.Flags().String("interval", "", "interval for periodic trigger")
	triggerUpdateCmd.Flags().String("cron", "", "cron expression")
	triggerUpdateCmd.Flags().String("watch-path", "", "watch path for edit trigger")
	triggerUpdateCmd.Flags().Bool("recursive", false, "recursive watch")
	triggerUpdateCmd.Flags().StringSlice("event-ids", nil, "bound event IDs")

	triggerDeleteCmd.Flags().StringP("id", "i", "", "trigger ID")
	triggerDeleteCmd.MarkFlagRequired("id")

	triggerStartCmd.Flags().StringP("id", "i", "", "trigger ID")
	triggerStartCmd.MarkFlagRequired("id")

	triggerStopCmd.Flags().StringP("id", "i", "", "trigger ID")
	triggerStopCmd.MarkFlagRequired("id")

	triggerRunCmd.Flags().StringP("id", "i", "", "trigger ID")
	triggerRunCmd.MarkFlagRequired("id")

	triggerCmd.AddCommand(triggerCreateCmd, triggerUpdateCmd, triggerDeleteCmd, triggerListCmd, triggerStartCmd, triggerStopCmd, triggerRunCmd)
	rootCmd.AddCommand(triggerCmd)
}
