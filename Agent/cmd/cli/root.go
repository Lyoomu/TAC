package main

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "TAC",
	Short: "TAC Agent admin CLI",
	Long:  "Command-line interface for managing TAC Agent: components, models, roles, tools, and chat sessions.",
}

func init() {
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file path (default: exe_dir/properties.yaml)")
}
