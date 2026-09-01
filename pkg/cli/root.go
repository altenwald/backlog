package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	flagProject string
	flagAPIURL  string
	flagDataDir string
	flagPort    int
	flagDaemon  bool
)

var RootCmd = &cobra.Command{
	Use:   "backlog",
	Short: "Backlog — Visual task & priority manager with System Tray and MCP",
	Long: `Backlog is a developer-centric task & priority manager featuring a Fyne desktop GUI,
a resident System Tray menu bar icon, a rich CLI for terminal workflows,
and a native MCP server for real-time integrations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStartApp(cmd, args)
	},
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().StringVarP(&flagProject, "project", "p", "", "Target project slug (defaults to BACKLOG_PROJECT env or active project)")
	RootCmd.PersistentFlags().StringVar(&flagAPIURL, "api-url", "http://127.0.0.1:8484", "Backlog daemon REST API URL")
	RootCmd.PersistentFlags().StringVar(&flagDataDir, "data-dir", "", "Data directory path (defaults to ~/.config/backlog)")
	RootCmd.PersistentFlags().IntVar(&flagPort, "port", 8484, "Local port for REST API and MCP SSE")
	RootCmd.PersistentFlags().BoolVarP(&flagDaemon, "daemon", "d", false, "Run Backlog in background detached from terminal")
}

func resolveProject(cliProject string) string {
	if cliProject != "" {
		return strings.ToLower(cliProject)
	}
	if envProject := os.Getenv("BACKLOG_PROJECT"); envProject != "" {
		return strings.ToLower(envProject)
	}
	return ""
}
