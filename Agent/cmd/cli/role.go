package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/Lyoomu/TAC/Agent/internal/model"
)

var roleCmd = &cobra.Command{
	Use:   "role",
	Short: "Manage roles",
}

var roleCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a role",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		desc, _ := cmd.Flags().GetString("description")
		components, _ := cmd.Flags().GetStringSlice("components")
		tools, _ := cmd.Flags().GetStringSlice("tools")
		envFlag, _ := cmd.Flags().GetStringSlice("env")
		apiType, _ := cmd.Flags().GetString("api-type")
		messageMode, _ := cmd.Flags().GetString("message-mode")
		modelName, _ := cmd.Flags().GetString("model")

		envMap := parseKVList(envFlag)

		r := &model.Role{
			Name:        name,
			Description: desc,
			Components:  components,
			Tools:       tools,
			Env:         envMap,
			APIType:     model.APIType(apiType),
			MessageMode: messageMode,
			Model:       modelName,
		}
		if err := appCtx.RoleEngine.Create(r); err != nil {
			return err
		}
		fmt.Printf("created: %s\n", name)
		return nil
	},
}

var roleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all roles",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		list, err := appCtx.RoleEngine.List()
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tAPI_TYPE\tMODE\tMODEL\tCOMPONENTS\tTOOLS")
		for _, r := range list {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Name, r.APIType, r.MessageMode, r.Model, strings.Join(r.Components, ","), strings.Join(r.Tools, ","))
		}
		w.Flush()
		return nil
	},
}

var roleGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a role by name",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		r, err := appCtx.RoleEngine.Get(name)
		if err != nil {
			return err
		}
		out, _ := json.MarshalIndent(r, "", "  ")
		fmt.Println(string(out))
		return nil
	},
}

var roleUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a role",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		desc, _ := cmd.Flags().GetString("description")
		components, _ := cmd.Flags().GetStringSlice("components")
		tools, _ := cmd.Flags().GetStringSlice("tools")
		envFlag, _ := cmd.Flags().GetStringSlice("env")
		apiType, _ := cmd.Flags().GetString("api-type")
		messageMode, _ := cmd.Flags().GetString("message-mode")
		modelName, _ := cmd.Flags().GetString("model")

		updates := &model.Role{
			Description: desc,
			Components:  components,
			Tools:       tools,
			Env:         parseKVList(envFlag),
			APIType:     model.APIType(apiType),
			MessageMode: messageMode,
			Model:       modelName,
		}
		if err := appCtx.RoleEngine.Update(name, updates); err != nil {
			return err
		}
		fmt.Printf("updated: %s\n", name)
		return nil
	},
}

var roleDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a role",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		if err := appCtx.RoleEngine.Delete(name); err != nil {
			return err
		}
		fmt.Printf("deleted: %s\n", name)
		return nil
	},
}

func parseKVList(items []string) map[string]string {
	m := make(map[string]string)
	for _, item := range items {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}

func init() {
	roleCreateCmd.Flags().StringP("name", "n", "", "role name (required)")
	roleCreateCmd.Flags().StringP("description", "d", "", "description")
	roleCreateCmd.Flags().StringSliceP("components", "m", nil, "component names (comma-separated)")
	roleCreateCmd.Flags().StringSliceP("tools", "t", nil, "tool names (comma-separated)")
	roleCreateCmd.Flags().StringSlice("env", nil, "env vars in key=value form")
	roleCreateCmd.Flags().StringP("api-type", "a", "chat_completion", "API type: chat_completion, responses, or anthropic")
	roleCreateCmd.Flags().String("message-mode", "", "message mode: queue, reject, or interrupt (default: interrupt)")
	roleCreateCmd.Flags().String("model", "", "bound model name (required)")
	roleCreateCmd.MarkFlagRequired("name")
	roleCreateCmd.MarkFlagRequired("model")

	roleGetCmd.Flags().StringP("name", "n", "", "role name (required)")
	roleGetCmd.MarkFlagRequired("name")

	roleUpdateCmd.Flags().StringP("name", "n", "", "role name (required)")
	roleUpdateCmd.Flags().StringP("description", "d", "", "description")
	roleUpdateCmd.Flags().StringSliceP("components", "m", nil, "component names")
	roleUpdateCmd.Flags().StringSliceP("tools", "t", nil, "tool names")
	roleUpdateCmd.Flags().StringSlice("env", nil, "env vars in key=value form")
	roleUpdateCmd.Flags().StringP("api-type", "a", "", "API type")
	roleUpdateCmd.Flags().String("message-mode", "", "message mode: queue, reject, or interrupt")
	roleUpdateCmd.Flags().String("model", "", "bound model name")
	roleUpdateCmd.MarkFlagRequired("name")

	roleDeleteCmd.Flags().StringP("name", "n", "", "role name (required)")
	roleDeleteCmd.MarkFlagRequired("name")

	roleCmd.AddCommand(roleCreateCmd, roleListCmd, roleGetCmd, roleUpdateCmd, roleDeleteCmd)
	rootCmd.AddCommand(roleCmd)
}
