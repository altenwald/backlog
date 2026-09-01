package cli

import (
	"fmt"

	"github.com/altenwald/backlog/pkg/client"
	"github.com/altenwald/backlog/pkg/server"
	mcpServer "github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the MCP server over standard I/O (stdio) for MCP-compatible clients",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.NewClient(flagAPIURL)
		if !c.IsServerRunning() {
			return fmt.Errorf("⚠️  Backlog server daemon is not running on %s. Please start Backlog first", flagAPIURL)
		}

		if proj := resolveProject(flagProject); proj != "" {
			_ = c.SetActiveProject(proj)
		}

		s := server.NewMCPServerWithBackend(server.NewClientBackend(c))
		return mcpServer.ServeStdio(s)
	},
}

func init() {
	RootCmd.AddCommand(mcpCmd)
}
