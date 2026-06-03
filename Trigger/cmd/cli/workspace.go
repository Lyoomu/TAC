package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/Lyoomu/TAC/Trigger/internal/workspace"
)

var wsEngine *workspace.Engine

func initWorkspaceEngine() error {
	if wsEngine != nil {
		return nil
	}
	wsEngine = workspace.NewEngine()
	if err := wsEngine.Load(); err != nil {
		return fmt.Errorf("load workspaces: %w", err)
	}
	return nil
}

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage workspace bindings",
}

var workspaceBindCmd = &cobra.Command{
	Use:   "bind",
	Short: "Bind a logical name to a disk directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initWorkspaceEngine(); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		path, _ := cmd.Flags().GetString("path")

		err := wsEngine.Bind(name, path)
		if err == nil {
			fmt.Printf("bound: %s -> %s\n", name, path)
			return nil
		}

		if err == workspace.ErrWorkspaceExists {
			return fmt.Errorf("workspace already exists")
		}

		if err == workspace.ErrPathAlreadyBound {
			return fmt.Errorf("path already bound to another workspace")
		}

		return err
	},
}

var workspaceUnbindCmd = &cobra.Command{
	Use:   "unbind",
	Short: "Unbind a workspace (does not delete .tac directory)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initWorkspaceEngine(); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")

		ws, err := wsEngine.Get(name)
		if err != nil {
			return err
		}

		if confirmInteractive(fmt.Sprintf("unbind workspace '%s' (%s)?", name, ws.Path)) {
			if err := wsEngine.Unbind(name); err != nil {
				return err
			}
			fmt.Printf("unbound: %s\n", name)
		} else {
			fmt.Println("unbind cancelled")
		}
		return nil
	},
}

var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workspaces",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initWorkspaceEngine(); err != nil {
			return err
		}
		list := wsEngine.List()
		if len(list) == 0 {
			fmt.Println("no workspaces bound")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tPATH\tACTIVE")
		for _, ws := range list {
			active := ""
			if ws.IsActive {
				active = "*"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", ws.Name, ws.Path, active)
		}
		w.Flush()
		return nil
	},
}

var workspaceActivateCmd = &cobra.Command{
	Use:   "activate",
	Short: "Activate a workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initWorkspaceEngine(); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		if err := wsEngine.Activate(name); err != nil {
			return err
		}
		fmt.Printf("activated: %s\n", name)
		return nil
	},
}

var workspaceGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get workspace details",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initWorkspaceEngine(); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		ws, err := wsEngine.Get(name)
		if err != nil {
			return err
		}
		fmt.Printf("Name:      %s\n", ws.Name)
		fmt.Printf("Path:      %s\n", ws.Path)
		fmt.Printf("Active:    %v\n", ws.IsActive)
		fmt.Printf("CreatedAt: %s\n", ws.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("UpdatedAt: %s\n", ws.UpdatedAt.Format("2006-01-02 15:04:05"))
		return nil
	},
}

var workspaceActiveCmd = &cobra.Command{
	Use:   "active",
	Short: "Show current active workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initWorkspaceEngine(); err != nil {
			return err
		}
		ws, err := wsEngine.GetActive()
		if err != nil {
			return err
		}
		fmt.Printf("%s -> %s\n", ws.Name, ws.Path)
		return nil
	},
}

func init() {
	workspaceBindCmd.Flags().StringP("name", "n", "", "logical workspace name (required)")
	workspaceBindCmd.Flags().StringP("path", "p", ".", "disk directory path")
	workspaceBindCmd.MarkFlagRequired("name")

	workspaceUnbindCmd.Flags().StringP("name", "n", "", "workspace name (required)")
	workspaceUnbindCmd.MarkFlagRequired("name")

	workspaceActivateCmd.Flags().StringP("name", "n", "", "workspace name (required)")
	workspaceActivateCmd.MarkFlagRequired("name")

	workspaceGetCmd.Flags().StringP("name", "n", "", "workspace name (required)")
	workspaceGetCmd.MarkFlagRequired("name")

	workspaceCmd.AddCommand(workspaceBindCmd, workspaceUnbindCmd, workspaceListCmd,
		workspaceActivateCmd, workspaceGetCmd, workspaceActiveCmd)
	rootCmd.AddCommand(workspaceCmd)
}
