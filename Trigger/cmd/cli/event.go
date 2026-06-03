package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	pb "github.com/Lyoomu/TAC/proto"
)

var eventCmd = &cobra.Command{
	Use:   "event",
	Short: "Manage events",
}

var eventCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new event",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := ensureDaemonOrInit()
		if err != nil {
			return err
		}

		id, _ := cmd.Flags().GetString("id")
		name, _ := cmd.Flags().GetString("name")
		description, _ := cmd.Flags().GetString("description")
		roleKey, _ := cmd.Flags().GetString("role-key")
		initialMsg, _ := cmd.Flags().GetString("initial-msg")
		sessionMode, _ := cmd.Flags().GetString("session-mode")
		messageMode, _ := cmd.Flags().GetString("message-mode")
		envPreset, _ := cmd.Flags().GetString("env-preset")
		env, _ := cmd.Flags().GetStringToString("env")

		if id == "" || name == "" || roleKey == "" {
			return fmt.Errorf("--id, --name and --role-key are required")
		}

		resp, err := client.CreateEvent(&pb.CreateEventRequest{
			Id:          id,
			Name:        name,
			Description: description,
			RoleKey:     roleKey,
			InitialMsg:  initialMsg,
			SessionMode: sessionMode,
			MessageMode: messageMode,
			EnvPreset:   envPreset,
			Env:         env,
		})
		if err != nil {
			return err
		}
		if resp.Success {
			fmt.Printf("event '%s' created\n", id)
		} else {
			fmt.Printf("failed: %s\n", resp.Message)
		}
		return nil
	},
}

var eventUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update an existing event",
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
		roleKey, _ := cmd.Flags().GetString("role-key")
		initialMsg, _ := cmd.Flags().GetString("initial-msg")
		sessionMode, _ := cmd.Flags().GetString("session-mode")
		messageMode, _ := cmd.Flags().GetString("message-mode")
		envPreset, _ := cmd.Flags().GetString("env-preset")
		env, _ := cmd.Flags().GetStringToString("env")

		setMessageMode := cmd.Flags().Changed("message-mode")
		setEnvPreset := cmd.Flags().Changed("env-preset")
		setEnv := cmd.Flags().Changed("env")

		resp, err := client.UpdateEvent(&pb.UpdateEventRequest{
			Id:          id,
			Name:        name,
			Description: description,
			RoleKey:     roleKey,
			InitialMsg:  initialMsg,
			SessionMode: sessionMode,
			MessageMode: messageMode,
			EnvPreset:   envPreset,
			Env:         env,
			SetMessageMode: setMessageMode,
			SetEnvPreset:   setEnvPreset,
			SetEnv:         setEnv,
		})
		if err != nil {
			return err
		}
		if resp.Success {
			fmt.Printf("event '%s' updated\n", id)
		} else {
			fmt.Printf("failed: %s\n", resp.Message)
		}
		return nil
	},
}

var eventDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an event",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := ensureDaemonOrInit()
		if err != nil {
			return err
		}

		id, _ := cmd.Flags().GetString("id")
		if id == "" {
			return fmt.Errorf("--id is required")
		}

		resp, err := client.DeleteEvent(id)
		if err != nil {
			return err
		}
		if resp.Success {
			fmt.Printf("event '%s' deleted\n", id)
		} else {
			fmt.Printf("failed: %s\n", resp.Message)
		}
		return nil
	},
}

var eventListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all events",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := ensureDaemonOrInit()
		if err != nil {
			return err
		}

		resp, err := client.ListEvents()
		if err != nil {
			return err
		}

		if len(resp.Events) == 0 {
			fmt.Println("no events")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tROLE\tSESSION\tENV")
		for _, ev := range resp.Events {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d vars\n", ev.Id, ev.Name, ev.RoleKey, ev.SessionMode, ev.EnvCount)
		}
		w.Flush()
		return nil
	},
}

func init() {
	eventCreateCmd.Flags().StringP("id", "i", "", "event unique id")
	eventCreateCmd.Flags().StringP("name", "n", "", "event name")
	eventCreateCmd.Flags().StringP("description", "d", "", "description")
	eventCreateCmd.Flags().String("role-key", "", "role key (ServerName-RoleName)")
	eventCreateCmd.Flags().String("initial-msg", "", "initial message template")
	eventCreateCmd.Flags().String("session-mode", "shared", "session mode: shared, new")
	eventCreateCmd.Flags().String("message-mode", "", "message mode: queue, reject, interrupt")
	eventCreateCmd.Flags().String("env-preset", "", "env preset name")
	eventCreateCmd.Flags().StringToString("env", nil, "environment variables (key=value)")

	eventUpdateCmd.Flags().StringP("id", "i", "", "event ID")
	eventUpdateCmd.MarkFlagRequired("id")
	eventUpdateCmd.Flags().StringP("name", "n", "", "event name")
	eventUpdateCmd.Flags().StringP("description", "d", "", "description")
	eventUpdateCmd.Flags().String("role-key", "", "role key")
	eventUpdateCmd.Flags().String("initial-msg", "", "initial message template")
	eventUpdateCmd.Flags().String("session-mode", "", "session mode")
	eventUpdateCmd.Flags().String("message-mode", "", "message mode")
	eventUpdateCmd.Flags().String("env-preset", "", "env preset name")
	eventUpdateCmd.Flags().StringToString("env", nil, "environment variables")

	eventDeleteCmd.Flags().StringP("id", "i", "", "event ID")
	eventDeleteCmd.MarkFlagRequired("id")

	eventCmd.AddCommand(eventCreateCmd, eventUpdateCmd, eventDeleteCmd, eventListCmd)
	rootCmd.AddCommand(eventCmd)
}
