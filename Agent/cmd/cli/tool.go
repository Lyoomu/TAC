package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var toolCmd = &cobra.Command{
	Use:   "tool",
	Short: "Manage tools",
}

var toolListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered tools",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		list := appCtx.ToolEngine.List()
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tVERSION\tDESCRIPTION\tSCRIPTS")
		for _, t := range list {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", t.Name, t.Version, t.Config.Description, len(t.Scripts))
		}
		w.Flush()
		return nil
	},
}

var toolRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Ensure builtin tools and reload from database",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		if err := appCtx.ToolEngine.Register(); err != nil {
			return err
		}
		fmt.Println("tool registry refreshed")
		return nil
	},
}

var toolInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show tool config from database",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		info, err := appCtx.ToolEngine.GetInfo(name)
		if err != nil {
			return err
		}
		fmt.Println(string(info))
		return nil
	},
}

var toolDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a tool from the database",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		if err := appCtx.ToolEngine.Delete(name); err != nil {
			return err
		}
		fmt.Printf("deleted tool: %s\n", name)
		return nil
	},
}

func init() {
	toolInfoCmd.Flags().StringP("name", "n", "", "tool name (required)")
	toolInfoCmd.MarkFlagRequired("name")
	toolDeleteCmd.Flags().StringP("name", "n", "", "tool name (required)")
	toolDeleteCmd.MarkFlagRequired("name")

	toolCmd.AddCommand(toolListCmd, toolRegisterCmd, toolInfoCmd, toolDeleteCmd)
	rootCmd.AddCommand(toolCmd)
}
