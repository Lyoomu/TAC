package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/Lyoomu/TAC/Agent/internal/model"
)

var componentCmd = &cobra.Command{
	Use:   "component",
	Short: "Manage components",
}

var componentCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a component",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		cType, _ := cmd.Flags().GetString("type")
		content, _ := cmd.Flags().GetString("content")
		desc, _ := cmd.Flags().GetString("description")

		c := &model.Component{
			Name:        name,
			Type:        model.ComponentType(cType),
			Content:     content,
			Description: desc,
		}
		if err := appCtx.ComponentEngine.Create(c); err != nil {
			return err
		}
		fmt.Printf("created: %s\n", name)
		return nil
	},
}

var componentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all components",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		list, err := appCtx.ComponentEngine.List()
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tTYPE\tDESCRIPTION")
		for _, c := range list {
			fmt.Fprintf(w, "%s\t%s\t%s\n", c.Name, c.Type, c.Description)
		}
		w.Flush()
		return nil
	},
}

var componentGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a component by name",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		c, err := appCtx.ComponentEngine.Get(name)
		if err != nil {
			return err
		}
		out, _ := json.MarshalIndent(c, "", "  ")
		fmt.Println(string(out))
		return nil
	},
}

var componentUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a component",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		cType, _ := cmd.Flags().GetString("type")
		content, _ := cmd.Flags().GetString("content")
		desc, _ := cmd.Flags().GetString("description")

		updates := &model.Component{
			Type:        model.ComponentType(cType),
			Content:     content,
			Description: desc,
		}
		if err := appCtx.ComponentEngine.Update(name, updates); err != nil {
			return err
		}
		fmt.Printf("updated: %s\n", name)
		return nil
	},
}

var componentDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a component",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		if err := appCtx.ComponentEngine.Delete(name); err != nil {
			return err
		}
		fmt.Printf("deleted: %s\n", name)
		return nil
	},
}

func init() {
	componentCreateCmd.Flags().StringP("name", "n", "", "component name (required)")
	componentCreateCmd.Flags().StringP("type", "t", "", "component type: static or embedded (required)")
	componentCreateCmd.Flags().StringP("content", "o", "", "component content (required)")
	componentCreateCmd.Flags().StringP("description", "d", "", "component description")
	componentCreateCmd.MarkFlagRequired("name")
	componentCreateCmd.MarkFlagRequired("type")
	componentCreateCmd.MarkFlagRequired("content")

	componentGetCmd.Flags().StringP("name", "n", "", "component name (required)")
	componentGetCmd.MarkFlagRequired("name")

	componentUpdateCmd.Flags().StringP("name", "n", "", "component name (required)")
	componentUpdateCmd.Flags().StringP("type", "t", "", "new type")
	componentUpdateCmd.Flags().StringP("content", "o", "", "new content")
	componentUpdateCmd.Flags().StringP("description", "d", "", "new description")
	componentUpdateCmd.MarkFlagRequired("name")

	componentDeleteCmd.Flags().StringP("name", "n", "", "component name (required)")
	componentDeleteCmd.MarkFlagRequired("name")

	componentCmd.AddCommand(componentCreateCmd, componentListCmd, componentGetCmd, componentUpdateCmd, componentDeleteCmd)
	rootCmd.AddCommand(componentCmd)
}
