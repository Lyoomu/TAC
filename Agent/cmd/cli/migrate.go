package main

import (
	"github.com/spf13/cobra"

	"github.com/Lyoomu/TAC/Agent/internal/db"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Database migrations",
}

var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Apply all pending migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		return db.MigrateUp(appCtx.DB)
	},
}

var migrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Roll back all migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		return db.MigrateDown(appCtx.DB)
	},
}

func init() {
	migrateCmd.AddCommand(migrateUpCmd, migrateDownCmd)
	rootCmd.AddCommand(migrateCmd)
}
