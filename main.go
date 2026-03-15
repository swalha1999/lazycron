package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/record"
	sshclient "github.com/swalha1999/lazycron/ssh"
	"github.com/swalha1999/lazycron/ui"
)

func main() {
	if err := cron.CheckCrontabAvailable(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := record.InstallRecord(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not install record script: %v\n", err)
	}

	mgr := backend.NewManager()

	// Load config and add remote servers
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
			client := sshclient.NewClient(srv.Host, port, srv.User, srv.Password, config.ExpandHome(srv.KeyPath), srv.UseAgent)
			remote := backend.NewRemoteBackend(srv.Name, client)
			mgr.AddServer(info, remote)
		}
	}

	p := tea.NewProgram(
		ui.NewModel(mgr),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		mgr.CloseAll()
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	mgr.CloseAll()
}
