package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/record"
	sshclient "github.com/swalha1999/lazycron/ssh"
	"github.com/swalha1999/lazycron/ui"
)

// Version is set at build time via ldflags.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "lazycron",
	Short: "A terminal UI for managing cron jobs",
	Long:  "lazycron is a TUI for viewing, creating, editing, and running crontab entries — like lazygit for cron.",
	RunE:  runTUI,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var cronFile string

func init() {
	rootCmd.Version = Version
	rootCmd.Flags().StringVar(&cronFile, "cron-file", "", "use a plain crontab file instead of the system crontab (useful for testing/dry-run)")
}

// initBackendManager creates a backend.Manager with local + configured remote servers.
func initBackendManager() *backend.Manager {
	mgr := backend.NewManager()

	cfg, err := config.Load()
	if err == nil && len(cfg.Servers) > 0 {
		for _, srv := range cfg.Servers {
			port := srv.Port
			if port == 0 {
				port = 22
			}
			info := backend.ServerInfo{
				Name:   srv.Name,
				Host:   srv.Host,
				Port:   port,
				User:   srv.User,
				Status: backend.ConnDisconnected,
			}
			client := sshclient.NewClient(srv.Host, port, srv.User, "", config.ExpandHome(srv.KeyPath), srv.UseAgent)
			remote := backend.NewRemoteBackend(srv.Name, client)
			mgr.AddServer(info, remote)
		}
	}

	return mgr
}

func runTUI(cmd *cobra.Command, args []string) error {
	var mgr *backend.Manager

	if cronFile != "" {
		histDir := cronFile + ".history"
		if err := os.MkdirAll(histDir, 0o755); err != nil {
			return fmt.Errorf("create history dir: %w", err)
		}
		fb := backend.NewFileBackend(cronFile, histDir)
		mgr = backend.NewManagerWithBackend(fb)
	} else {
		if err := cron.CheckCrontabAvailable(); err != nil {
			return err
		}

		if err := record.InstallRecord(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not install record script: %v\n", err)
		}

		mgr = initBackendManager()
	}

	p := tea.NewProgram(
		ui.NewModel(mgr, Version),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		mgr.CloseAll()
		return fmt.Errorf("TUI error: %w", err)
	}

	mgr.CloseAll()
	return nil
}
