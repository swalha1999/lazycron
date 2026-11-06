package cmd

import (
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/record"
	"github.com/swalha1999/lazycron/ui"
)

// Version is set at build time via ldflags.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:   "lazycron",
	Short: "A terminal UI for managing cron jobs",
	Long:  "lazycron is a TUI for viewing, creating, editing, and running crontab entries — like lazygit for cron.",
	RunE:  runTUI,

	SilenceErrors:     true,
	SilenceUsage:      true,
	PersistentPreRunE: loadGlobalSettings,
}

// Execute runs the CLI and exits with its exit code.
func Execute() {
	os.Exit(executeArgs(os.Args[1:], os.Stdout, os.Stderr))
}

// executeArgs runs the CLI with args and returns the process exit code.
// Errors are printed to stderr as {"error":{kind,message,hint}} in JSON mode.
func executeArgs(args []string, stdout, stderr io.Writer) int {
	rootCmd.SetArgs(args)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	cmd, err := rootCmd.ExecuteC()
	if err == nil {
		return exitOK
	}
	ce := asCLIError(err)
	printError(cmd, stderr, ce)
	return ce.code
}

var (
	cronFile     string
	globalServer string
)

func init() {
	rootCmd.Version = Version
	pf := rootCmd.PersistentFlags()
	pf.BoolVar(&jsonFlag, "json", false, "JSON output (default when stdout is not a terminal; --json=false forces text)")
	pf.StringVar(&cronFile, "cron-file", "", "use a plain crontab file instead of the system crontab (testing/dry-run)")
	pf.StringVar(&globalServer, "server", "", "target server name from config (default: this machine)")
	rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return errUsage("%s", err.Error()).withHint("run `lazycron <command> --help`")
	})
}

// initBackendManager creates a backend.Manager with local + configured remote servers.
func initBackendManager() *backend.Manager {
	mgr := backend.NewManager()
	cfg, err := config.Load()
	if err != nil {
		return mgr
	}
	for _, srv := range cfg.Servers {
		info := backend.ServerInfo{
			Name:   srv.Name,
			Host:   srv.Host,
			Port:   srv.PortOrDefault(),
			User:   srv.User,
			Status: backend.ConnDisconnected,
		}
		mgr.AddServer(info, newRemoteBackend(srv))
	}
	return mgr
}

func runTUI(cmd *cobra.Command, args []string) error {
	var mgr *backend.Manager

	if cronFile != "" {
		fb := backend.NewFileBackend(cronFile, cronFile+".history")
		mgr = backend.NewManagerWithBackend(fb)
	} else {
		if err := cron.CheckCrontabAvailable(); err != nil {
			return errUnavailable("%v", err)
		}
		if err := record.InstallRecord(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not install record script: %v\n", err)
		}
		record.InstallSelfLink()
		mgr = initBackendManager()
	}

	p := tea.NewProgram(
		ui.NewModel(mgr, Version),
		tea.WithAltScreen(),
	)

	_, err := p.Run()
	mgr.CloseAll()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	return nil
}
