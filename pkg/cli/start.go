package cli

import (
	"fmt"
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
	st, err := store.NewStore(flagDataDir)
	if err != nil {
		return fmt.Errorf("error initializing storage: %w", err)
	}

	if err := acquireInstanceLock(st.GetDataDir()); err != nil {
		return err
	}

	if proj := resolveProject(flagProject); proj != "" {
		_ = st.SetActiveProject(proj)
	}

	srv := server.NewServer(st, flagPort)
	go func() {
		log.Printf("[Backlog Server] Listening REST API on http://127.0.0.1:%d", flagPort)
		log.Printf("[Backlog Server] Listening MCP SSE on http://127.0.0.1:%d/sse", flagPort+1)
		if err := srv.Start(); err != nil {
			log.Printf("[Backlog Server] HTTP Server error: %v", err)
		}
	}()

	app := ui.NewBacklogApp(st)
	app.Run()

	return nil
}
