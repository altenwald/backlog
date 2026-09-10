package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/altenwald/backlog/pkg/client"
	"github.com/altenwald/backlog/pkg/server"
	"github.com/altenwald/backlog/pkg/store"
	"github.com/spf13/cobra"
)

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Manage Backlog settings",
}

var settingsGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show the current user-defined MCP instructions",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.NewClient(flagAPIURL)
		if c.IsServerRunning() {
			s, err := c.GetSettings()
			if err != nil {
				return err
			}
			printInstructions(s.MCPUserInstructions)
		} else {
			st, err := store.NewStore(flagDataDir)
			if err != nil {
				return err
			}
			printInstructions(st.GetMCPUserInstructions())
		}
		return nil
	},
}

var settingsSetCmd = &cobra.Command{
	Use:     "set-instructions <text>",
	Aliases: []string{"set"},
	Short:   "Replace the user-defined MCP instructions (pass text or - to read from stdin)",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		text, err := resolveInstructionsArg(args[0])
		if err != nil {
			return err
		}

		c := client.NewClient(flagAPIURL)
		if c.IsServerRunning() {
			_, err := c.UpdateSettings(client.SettingsInfo{MCPUserInstructions: text})
			if err != nil {
				return err
			}
		} else {
			st, err := store.NewStore(flagDataDir)
			if err != nil {
				return err
			}
			if err := st.SaveMCPUserInstructions(text); err != nil {
				return err
			}
		}

		fmt.Println("✔ MCP instructions updated successfully.")
		fmt.Println("  Changes take effect the next time the MCP server (re)connects.")
		return nil
	},
}

var settingsResetCmd = &cobra.Command{
	Use:     "reset-instructions",
	Aliases: []string{"reset"},
	Short:   "Reset user-defined MCP instructions to the built-in default",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.NewClient(flagAPIURL)
		if c.IsServerRunning() {
			// Storing empty string tells the server to use the default
			_, err := c.UpdateSettings(client.SettingsInfo{MCPUserInstructions: ""})
			if err != nil {
				return err
			}
		} else {
			st, err := store.NewStore(flagDataDir)
			if err != nil {
				return err
			}
			if err := st.SaveMCPUserInstructions(""); err != nil {
				return err
			}
		}

		fmt.Println("✔ MCP instructions reset to default.")
		return nil
	},
}

// printInstructions prints the effective instructions, noting when the default is active.
func printInstructions(saved string) {
	fmt.Println("── Core instructions (read-only) ─────────────────────────────────")
	fmt.Println(server.BacklogCoreInstructions)
	fmt.Println()
	fmt.Println("── Custom instructions ────────────────────────────────────────────")
	if strings.TrimSpace(saved) == "" {
		fmt.Println("[using built-in default]")
		fmt.Println(strings.TrimSpace(server.BacklogDefaultUserInstructions))
	} else {
		fmt.Println(saved)
	}
}

// resolveInstructionsArg returns the instruction text.
// If arg is "-", it reads from stdin.
func resolveInstructionsArg(arg string) (string, error) {
	if arg == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("failed to read from stdin: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}
	return strings.TrimSpace(arg), nil
}

func init() {
	settingsCmd.AddCommand(settingsGetCmd)
	settingsCmd.AddCommand(settingsSetCmd)
	settingsCmd.AddCommand(settingsResetCmd)

	RootCmd.AddCommand(settingsCmd)
}
