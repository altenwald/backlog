package cli

import (
	"fmt"

	"github.com/altenwald/backlog/pkg/server"
	"github.com/altenwald/backlog/pkg/store"
	mcpServer "github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the MCP server over standard I/O (stdio) for MCP-compatible clients",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := store.NewStore(flagDataDir)
		if err != nil {
			return fmt.Errorf("error initializing storage: %w", err)
		}

		if proj := resolveProject(flagProject); proj != "" {
			_ = st.SetActiveProject(proj)
		}

		s := server.NewMCPServer(st)
		return mcpServer.ServeStdio(s)
	},
}

func init() {
	RootCmd.AddCommand(mcpCmd)
}
