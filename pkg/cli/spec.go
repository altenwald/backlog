package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/altenwald/backlog/pkg/client"
	"github.com/altenwald/backlog/pkg/store"
	"github.com/spf13/cobra"
)

var flagSpecFile string

var specCmd = &cobra.Command{
	Use:     "spec",
	Aliases: []string{"specification"},
	Short:   "View or update composite project specification",
	RunE: func(cmd *cobra.Command, args []string) error {
		return specGetCmd.RunE(cmd, args)
	},
}

var specGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show the composite project specification",
	RunE: func(cmd *cobra.Command, args []string) error {
		proj := resolveProject(flagProject)
		if proj == "" {
			return fmt.Errorf("must specify a project via --project (-p) or BACKLOG_PROJECT environment variable")
		}

		c := client.NewClient(flagAPIURL)
		var spec string
		var err error
		if c.IsServerRunning() {
			spec, err = c.GetProjectSpecification(proj)
			if err != nil {
				return err
			}
		} else {
			st, err := store.NewStore(flagDataDir)
			if err != nil {
				return err
			}
			p, err := st.GetProject(proj)
			if err != nil {
				return err
			}
			spec = p.Specification
		}

		if strings.TrimSpace(spec) == "" {
			fmt.Printf("ℹ Project '%s' has no specification defined yet.\n", proj)
			return nil
		}

		fmt.Println(spec)
		return nil
	},
}

var specSetCmd = &cobra.Command{
	Use:   "set [text]",
	Short: "Update the composite project specification (provide text argument, --file/-f flag, or '-' to read from stdin)",
	RunE: func(cmd *cobra.Command, args []string) error {
		proj := resolveProject(flagProject)
		if proj == "" {
			return fmt.Errorf("must specify a project via --project (-p) or BACKLOG_PROJECT environment variable")
		}

		var specText string
		if flagSpecFile != "" {
			data, err := os.ReadFile(flagSpecFile)
			if err != nil {
				return fmt.Errorf("failed to read specification file: %w", err)
			}
			specText = string(data)
		} else if len(args) > 0 {
			arg := args[0]
			if arg == "-" {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read from stdin: %w", err)
				}
				specText = string(data)
			} else if fi, err := os.Stat(arg); err == nil && !fi.IsDir() {
				// If arg happens to be a valid file path, read it as file content
				data, err := os.ReadFile(arg)
				if err != nil {
					return fmt.Errorf("failed to read specification file: %w", err)
				}
				specText = string(data)
			} else {
				specText = arg
			}
		} else {
			return fmt.Errorf("must provide specification content as argument, --file/-f flag, or '-' to read from stdin")
		}

		c := client.NewClient(flagAPIURL)
		if c.IsServerRunning() {
			if err := c.UpdateProjectSpecification(proj, specText); err != nil {
				return err
			}
		} else {
			st, err := store.NewStore(flagDataDir)
			if err != nil {
				return err
			}
			if err := st.UpdateProjectSpecification(proj, specText); err != nil {
				return err
			}
		}

		fmt.Printf("✔ Project specification for '%s' updated successfully.\n", proj)
		return nil
	},
}

func init() {
	specSetCmd.Flags().StringVarP(&flagSpecFile, "file", "f", "", "Read specification from markdown/text file")
	specCmd.AddCommand(specGetCmd)
	specCmd.AddCommand(specSetCmd)
	RootCmd.AddCommand(specCmd)
}
