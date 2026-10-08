package cli

import (
	"context"
	"fmt"
	"github.com/altenwald/backlog/pkg/network"
	"log"

	"github.com/altenwald/backlog/pkg/server"
	"github.com/altenwald/backlog/pkg/store"
	"github.com/altenwald/backlog/pkg/ui"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start desktop GUI with System Tray and MCP/REST background server",
	RunE:  runStartApp,
}

func init() {
	RootCmd.AddCommand(startCmd)
}

func runStartApp(cmd *cobra.Command, args []string) error {
	ensureCLISymlink()

	st, err := store.NewStore(flagDataDir)
	if err != nil {
		return fmt.Errorf("error initializing storage: %w", err)
	}

	if flagDaemon {
		return spawnDaemon(st.GetDataDir())
	}

	if err := acquireInstanceLock(st.GetDataDir()); err != nil {
		return err
	}

	if path, err := st.AutoBackup(14); err != nil {
		log.Printf("[Backlog] Daily backup failed: %v", err)
	} else if path != "" {
		log.Printf("[Backlog] Daily backup written to %s", path)
	}

	if proj := resolveProject(flagProject); proj != "" {
		_ = st.SetActiveProject(proj)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer func() { cancel(); st.Close() }()
	lan, err := network.NewService(st)
	if err != nil {
		return err
	}
	hub := network.NewHub(ctx, st, lan)
	app := ui.NewBacklogApp(hub)
	if err = lan.Start(ctx, flagPort+2); err != nil {
		return fmt.Errorf("LAN service: %w", err)
	}
	srv := server.NewServer(network.NewCommands(hub), flagPort)
	defer srv.Stop(context.Background())
	go func() {
		log.Printf("[Backlog Server] Listening REST API on http://127.0.0.1:%d", flagPort)
		log.Printf("[Backlog Server] Listening MCP SSE on http://127.0.0.1:%d/sse", flagPort+1)
		if err := srv.Start(); err != nil {
			log.Printf("[Backlog Server] HTTP Server error: %v", err)
		}
	}()

	app.Run()

	return nil
}
