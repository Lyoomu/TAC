package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/Lyoomu/TAC/Trigger/internal/model"
	srv "github.com/Lyoomu/TAC/Trigger/internal/server"
	"github.com/Lyoomu/TAC/Trigger/internal/tool"
)

var srvEngine *srv.Engine

func initServerEngine() error {
	if srvEngine != nil {
		return nil
	}
	srvEngine = srv.NewEngine()
	if err := srvEngine.Load(); err != nil {
		return fmt.Errorf("load servers: %w", err)
	}
	return nil
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage Agent Server connections",
}

var serverConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to an Agent Server",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initServerEngine(); err != nil {
			return err
		}

		address, _ := cmd.Flags().GetString("address")
		token, _ := cmd.Flags().GetString("token")
		name, _ := cmd.Flags().GetString("name")
		fingerprint, _ := cmd.Flags().GetString("fingerprint")

		if address == "" {
			return fmt.Errorf("--address is required")
		}

		if srvEngine.Exists(address) {
			existing, _ := srvEngine.Get(address)
			fmt.Printf("server '%s' (%s) already connected\n", existing.DisplayName, address)
			return nil
		}

		connFingerprint := fingerprint
		if connFingerprint == "" {
			connFingerprint = "insecure"
		}

		client, err := srv.Connect(address, token, connFingerprint)
		if err != nil {
			return fmt.Errorf("connect to server: %w", err)
		}
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		pingResp, err := client.Ping(ctx)
		if err != nil {
			return fmt.Errorf("ping failed: %w", err)
		}

		fmt.Printf("Connected to %s (version: %s)\n", pingResp.ServerName, pingResp.ServerVersion)
		fmt.Printf("Certificate fingerprint: %s\n", pingResp.CertificateFingerprint)

		if fingerprint == "" {
			if confirmInteractive("Trust and save this certificate fingerprint for future connections?") {
				fingerprint = pingResp.CertificateFingerprint
			} else {
				fmt.Println("warning: connection accepted without saving fingerprint. You will be prompted again next time.")
			}
		}
		defer client.Close()

		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()

		info, err := client.GetServerInfo(ctx2)
		if err != nil {
			fmt.Printf("warning: cannot get server info: %v\n", err)
		} else {
			fmt.Printf("Server: %s | Roles: %d | Tools: %d\n", info.ServerName, info.RoleCount, info.ToolCount)
		}

		if name == "" {
			name = address

			fmt.Printf("Use name '%s' for this server? [y/n]: ", name)
			reader := newLineReader()
			if resp := reader.readLine(); resp == "n" || resp == "no" {
				fmt.Print("Enter custom name: ")
				name = reader.readLine()
				if name == "" {
					name = address
				}
			}
		}

		if srvEngine.ExistsByDisplayName(name) {
			fmt.Printf("warning: display name '%s' is already used by another server\n", name)
		}

		if fingerprint == "" {
			fingerprint = pingResp.CertificateFingerprint
		}

		if err := srvEngine.Add(address, name, token, fingerprint); err != nil {
			return fmt.Errorf("save server: %w", err)
		}

		fmt.Printf("server connected: %s -> %s\n", name, address)
		if fingerprint != "" {
			fmt.Println("certificate fingerprint saved for future verification")
		}
		return nil
	},
}

var serverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List connected Agent Servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initServerEngine(); err != nil {
			return err
		}
		list := srvEngine.List()
		if len(list) == 0 {
			fmt.Println("no servers connected")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tADDRESS\tROLES\tFINGERPRINT")
		for _, s := range list {
			fp := ""
			if s.TrustedFingerprint != "" {
				fp = s.TrustedFingerprint[:16] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", s.DisplayName, s.Address, len(s.Roles), fp)
		}
		w.Flush()
		return nil
	},
}

var serverRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a server connection",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initServerEngine(); err != nil {
			return err
		}
		address, _ := cmd.Flags().GetString("address")
		if address == "" {
			return fmt.Errorf("--address is required")
		}

		s, err := srvEngine.Get(address)
		if err != nil {
			return err
		}

		if confirmInteractive(fmt.Sprintf("remove server '%s' (%s)?", s.DisplayName, address)) {
			if err := srvEngine.Remove(address); err != nil {
				return err
			}
			fmt.Printf("removed: %s\n", address)
		} else {
			fmt.Println("cancelled")
		}
		return nil
	},
}

var serverLoadCmd = &cobra.Command{
	Use:   "load",
	Short: "Load a Role from a connected server",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initServerEngine(); err != nil {
			return err
		}
		address, _ := cmd.Flags().GetString("address")
		roleName, _ := cmd.Flags().GetString("role")

		if address == "" {
			return fmt.Errorf("--address is required")
		}
		if roleName == "" {
			return fmt.Errorf("--role is required")
		}

		s, err := srvEngine.Get(address)
		if err != nil {
			return err
		}

		client, err := srv.Connect(address, s.AuthToken, s.TrustedFingerprint)
		if err != nil {
			return fmt.Errorf("connect: %w", err)
		}
		defer client.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		role, err := client.GetRole(ctx, roleName)
		if err != nil {
			return fmt.Errorf("get role: %w", err)
		}

		toolsResp, err := client.GetRoleTools(ctx, roleName)
		if err != nil {
			fmt.Printf("warning: cannot get role tools: %v\n", err)
		}

		var toolInfos []model.ToolInfo
		if toolsResp != nil {
			for _, t := range toolsResp.Tools {
				toolInfos = append(toolInfos, model.ToolInfo{
					Name:                t.Name,
					Description:         t.Description,
					Language:            t.Language,
					Version:             t.Version,
					Dependencies:        t.Dependencies,
					RequiresCompilation: t.RequiresCompilation,
					IsBinary:            t.IsBinary,
					SourceAvailable:     t.SourceAvailable,
					RuntimeRequirement:  t.RuntimeRequirement,
					Files:               t.Files,
				})

				if confirmInteractive(fmt.Sprintf("download tool '%s' (%s)?", t.Name, t.Language)) {
					downloadSource := true
					downloadBinary := true
					if t.IsBinary && !t.SourceAvailable {
						fmt.Printf("WARNING: Tool '%s' is binary-only (no source available). Security risk!\n", t.Name)
						if !confirmInteractive("Download binary-only tool? (HIGH RISK)") {
							downloadBinary = false
						}
					}

					files, err := client.DownloadTool(ctx, t.Name, downloadSource, downloadBinary)
					if err != nil {
						fmt.Printf("failed to download tool '%s': %v\n", t.Name, err)
						continue
					}

					toolDir, err := tool.SaveFiles(s.DisplayName, t.Name, files)
					if err != nil {
						fmt.Printf("failed to save tool '%s': %v\n", t.Name, err)
						continue
					}

					for i := range toolInfos {
						if toolInfos[i].Name == t.Name {
							toolInfos[i].LocalPath = toolDir
							toolInfos[i].DownloadedAt = time.Now()
							break
						}
					}

					fmt.Printf("saved tool '%s' to %s (%d files)\n", t.Name, toolDir, len(files))
					for fname := range files {
						fmt.Printf("  - %s\n", fname)
					}
				}
			}
		}

		loadedRole := model.LoadedRole{
			ServerName:  s.DisplayName,
			RoleName:    role.Name,
			Description: role.Description,
			APIType:     role.ApiType,
			MessageMode: role.MessageMode,
			Tools:       toolInfos,
			LoadedAt:    time.Now(),
		}

		if err := srvEngine.LoadRole(address, loadedRole); err != nil {
			return fmt.Errorf("save loaded role: %w", err)
		}

		fmt.Printf("loaded role: %s-%s\n", s.DisplayName, role.Name)
		return nil
	},
}

var serverRolesCmd = &cobra.Command{
	Use:   "roles",
	Short: "List loaded roles from all servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initServerEngine(); err != nil {
			return err
		}

		roles := srvEngine.GetLoadedRoles()
		if len(roles) == 0 {
			fmt.Println("no roles loaded")
			return nil
		}

		for _, r := range roles {

			fmt.Printf("\x1b[36m%s\x1b[0m-\x1b[33m%s\x1b[0m  %s  [%s] mode=%s\n",
				r.ServerName, r.RoleName, r.Description, r.APIType, r.MessageMode)
			if len(r.Tools) > 0 {
				fmt.Printf("  tools: ")
				for i, t := range r.Tools {
					if i > 0 {
						fmt.Print(", ")
					}
					fmt.Print(t.Name)
				}
				fmt.Println()
			}
		}
		return nil
	},
}

type lineReader struct{}

func newLineReader() *lineReader {
	return &lineReader{}
}

func (r *lineReader) readLine() string {
	var s string
	fmt.Scanln(&s)
	return s
}

func init() {
	serverConnectCmd.Flags().StringP("address", "a", "", "server address (host:port)")
	serverConnectCmd.Flags().StringP("token", "t", "", "auth token")
	serverConnectCmd.Flags().StringP("name", "n", "", "display name (defaults to address)")
	serverConnectCmd.Flags().StringP("fingerprint", "f", "", "trusted certificate fingerprint")
	serverConnectCmd.MarkFlagRequired("address")

	serverRemoveCmd.Flags().StringP("address", "a", "", "server address")
	serverRemoveCmd.MarkFlagRequired("address")

	serverLoadCmd.Flags().StringP("address", "a", "", "server address")
	serverLoadCmd.Flags().StringP("role", "r", "", "role name to load")
	serverLoadCmd.MarkFlagRequired("address")
	serverLoadCmd.MarkFlagRequired("role")

	serverCmd.AddCommand(serverConnectCmd, serverListCmd, serverRemoveCmd, serverLoadCmd, serverRolesCmd)
	rootCmd.AddCommand(serverCmd)
}
