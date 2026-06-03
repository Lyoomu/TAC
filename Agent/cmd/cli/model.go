package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/Lyoomu/TAC/Agent/internal/model"
)

var modelCmd = &cobra.Command{
	Use:   "model",
	Short: "Manage LLM models",
}

var modelCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a model",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		modelID, _ := cmd.Flags().GetString("model")
		baseURL, _ := cmd.Flags().GetString("base-url")
		apiKey, _ := cmd.Flags().GetString("api-key")

		m := &model.Model{
			Name:    name,
			Model:   modelID,
			BaseURL: baseURL,
			APIKey:  apiKey,
		}
		if err := appCtx.ModelsEngine.Create(m); err != nil {
			return err
		}
		fmt.Printf("created: %s\n", name)
		return nil
	},
}

var modelListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all models (api_key hidden)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		list, err := appCtx.ModelsEngine.List()
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tMODEL\tBASE_URL")
		for _, m := range list {
			fmt.Fprintf(w, "%s\t%s\t%s\n", m.Name, m.Model, m.BaseURL)
		}
		w.Flush()
		return nil
	},
}

var modelGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a model (api_key hidden)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		m, err := appCtx.ModelsEngine.Get(name)
		if err != nil {
			return err
		}
		out, _ := json.MarshalIndent(m, "", "  ")
		fmt.Println(string(out))
		return nil
	},
}

var modelUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a model",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		modelID, _ := cmd.Flags().GetString("model")
		baseURL, _ := cmd.Flags().GetString("base-url")
		apiKey, _ := cmd.Flags().GetString("api-key")

		updates := &model.Model{
			Model:   modelID,
			BaseURL: baseURL,
			APIKey:  apiKey,
		}
		if err := appCtx.ModelsEngine.Update(name, updates); err != nil {
			return err
		}
		fmt.Printf("updated: %s\n", name)
		return nil
	},
}

var modelDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a model",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initContext(); err != nil {
			return err
		}
		name, _ := cmd.Flags().GetString("name")
		if err := appCtx.ModelsEngine.Delete(name); err != nil {
			return err
		}
		fmt.Printf("deleted: %s\n", name)
		return nil
	},
}

func init() {
	modelCreateCmd.Flags().StringP("name", "n", "", "model config name (required)")
	modelCreateCmd.Flags().StringP("model", "m", "", "LLM model ID, e.g. gpt-4o (required)")
	modelCreateCmd.Flags().StringP("base-url", "u", "", "API base URL (required)")
	modelCreateCmd.Flags().StringP("api-key", "k", "", "API key (required)")
	modelCreateCmd.MarkFlagRequired("name")
	modelCreateCmd.MarkFlagRequired("model")
	modelCreateCmd.MarkFlagRequired("base-url")
	modelCreateCmd.MarkFlagRequired("api-key")

	modelGetCmd.Flags().StringP("name", "n", "", "model config name (required)")
	modelGetCmd.MarkFlagRequired("name")

	modelUpdateCmd.Flags().StringP("name", "n", "", "model config name (required)")
	modelUpdateCmd.Flags().StringP("model", "m", "", "new LLM model ID")
	modelUpdateCmd.Flags().StringP("base-url", "u", "", "new base URL")
	modelUpdateCmd.Flags().StringP("api-key", "k", "", "new API key")
	modelUpdateCmd.MarkFlagRequired("name")

	modelDeleteCmd.Flags().StringP("name", "n", "", "model config name (required)")
	modelDeleteCmd.MarkFlagRequired("name")

	modelCmd.AddCommand(modelCreateCmd, modelListCmd, modelGetCmd, modelUpdateCmd, modelDeleteCmd)
	rootCmd.AddCommand(modelCmd)
}
