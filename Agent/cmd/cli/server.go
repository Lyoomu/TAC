package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/Lyoomu/TAC/Agent/internal/daemon"
	"github.com/Lyoomu/TAC/Agent/internal/server"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run the Agent as a gRPC server",
}

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the gRPC server",
	RunE: func(cmd *cobra.Command, args []string) error {
		isDaemon, _ := cmd.Flags().GetBool("daemon")
		addr, _ := cmd.Flags().GetString("addr")
		token, _ := cmd.Flags().GetString("token")
		certFile, _ := cmd.Flags().GetString("cert")
		keyFile, _ := cmd.Flags().GetString("key")

		if isDaemon {
			childArgs := []string{"server", "start"}
			if addr != ":50051" {
				childArgs = append(childArgs, "--addr", addr)
			}
			if token != "" {
				childArgs = append(childArgs, "--token", token)
			}
			if certFile != "" {
				childArgs = append(childArgs, "--cert", certFile)
			}
			if keyFile != "" {
				childArgs = append(childArgs, "--key", keyFile)
			}

			_, err := daemon.StartBackground(childArgs)
			if err != nil {
				return err
			}

			for i := 0; i < 30; i++ {
				if daemon.IsRunning() {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if !daemon.IsRunning() {
				return fmt.Errorf("background server failed to start (check logs)")
			}
			port, _ := daemon.ReadPort()
			if port == "" {
				port = addr
			}
			fmt.Printf("TAC Agent server started in background on %s\n", port)
			return nil
		}

		if err := initContext(); err != nil {
			return err
		}

		if token == "" {
			token = os.Getenv("TAC_SERVER_TOKEN")
		}
		if token == "" {
			fmt.Println("warning: no auth token set. Use --token or TAC_SERVER_TOKEN env var for security.")
		}

		srv := server.New(appCtx.Config, appCtx.RoleEngine, appCtx.ToolEngine, appCtx.AgentManager)
		srv.SetAuthToken(token)

		if err := srv.SetupTLS(certFile, keyFile); err != nil {
			return fmt.Errorf("setup tls: %w", err)
		}

		fingerprint := srv.CertificateFingerprint()
		if fingerprint != "" {
			fmt.Printf("TLS Certificate Fingerprint (SHA-256): %s\n", fingerprint)
			fmt.Println("Share this fingerprint with Trigger clients for verification.")
		}

		if err := srv.Start(addr); err != nil {
			return fmt.Errorf("start server: %w", err)
		}

		fmt.Printf("TAC Agent gRPC server started on %s\n", srv.Addr())
		if token != "" {
			fmt.Println("Auth token: enabled")
		}

		if os.Getenv("TAC_AGENT_DAEMON") == "1" {
			if err := daemon.SaveState(os.Getpid(), srv.Addr()); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to save daemon state: %v\n", err)
			}

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
			fmt.Println("\nShutting down agent server...")
			srv.Stop()
			daemon.ClearState()
			return nil
		}

		select {}
	},
}

var serverStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the background agent server",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := daemon.Stop(); err != nil {
			return err
		}
		fmt.Println("TAC Agent server stopped")
		return nil
	},
}

var serverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show agent server status",
	RunE: func(cmd *cobra.Command, args []string) error {
		if daemon.IsRunning() {
			port, _ := daemon.ReadPort()
			fmt.Printf("TAC Agent server is running on %s\n", port)
		} else {
			fmt.Println("TAC Agent server is not running")
		}
		return nil
	},
}

func init() {
	serverStartCmd.Flags().StringP("addr", "a", ":50051", "server listen address")
	serverStartCmd.Flags().StringP("token", "t", "", "access token for client authentication")
	serverStartCmd.Flags().StringP("cert", "", "", "TLS certificate file path")
	serverStartCmd.Flags().StringP("key", "", "", "TLS private key file path")
	serverStartCmd.Flags().BoolP("daemon", "d", false, "run server in background")

	serverCmd.AddCommand(serverStartCmd, serverStopCmd, serverStatusCmd)
	rootCmd.AddCommand(serverCmd)
}
