package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/altenwald/backlog/pkg/store"
	"github.com/spf13/cobra"
)

var flagExportFormat string

var backupCmd = &cobra.Command{
	Use:   "backup [file]",
	Short: "Write a copy of the database (works while Backlog is running)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := store.NewStore(flagDataDir)
		if err != nil {
			return err
		}
		defer st.Close()

		dest := filepath.Join(st.GetDataDir(), "backups", "backlog-"+time.Now().Format("20060102-150405")+".db")
		if len(args) == 1 {
			dest = args[0]
		}
		if err := st.Backup(dest); err != nil {
			return err
		}
		fmt.Printf("✔ Backup written to %s\n", dest)
		return nil
	},
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Print a project as JSON or Markdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		proj := resolveProject(flagProject)
		if proj == "" {
			return fmt.Errorf("must specify a project via --project (-p) or BACKLOG_PROJECT environment variable")
		}

		st, err := store.NewStore(flagDataDir)
		if err != nil {
			return err
		}
		defer st.Close()

		switch flagExportFormat {
		case "json":
			data, err := st.ExportProjectJSON(proj)
			if err != nil {
				return err
			}
			fmt.Println(string(data))
		case "md", "markdown":
			md, err := st.ExportProjectMarkdown(proj)
			if err != nil {
				return err
			}
			fmt.Print(md)
		default:
			return fmt.Errorf("unknown format %q (use json or md)", flagExportFormat)
		}
		return nil
	},
}

var importCmd = &cobra.Command{
	Use:   "import <project.json>",
	Short: "Add a project from a JSON export or an old JSON project file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}

		st, err := store.NewStore(flagDataDir)
		if err != nil {
			return err
		}
		defer st.Close()

		p, err := st.ImportProjectJSON(data)
		if err != nil {
			return err
		}
		fmt.Printf("✔ Imported project '%s' with %d tasks\n", p.Slug, len(p.Tasks))
		return nil
	},
}

func init() {
	exportCmd.Flags().StringVarP(&flagExportFormat, "format", "f", "json", "Output format: json or md")
	RootCmd.AddCommand(backupCmd, exportCmd, importCmd)
}
