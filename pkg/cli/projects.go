package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/altenwald/backlog/pkg/client"
	"github.com/spf13/cobra"
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List all projects and their status",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.NewClient(flagAPIURL)
		if !c.IsServerRunning() {
			fmt.Fprintln(os.Stderr, "⚠️  Backlog server does not appear to be running on "+flagAPIURL)
			return nil
		}

		projects, err := c.ListProjects()
		if err != nil {
			return err
		}

		fmt.Println("📂 Registered Projects in Backlog:")
		fmt.Println(strings.Repeat("─", 60))
		for _, p := range projects {
			activeIndicator := "  "
			if p.Active {
				activeIndicator = "● "
			}
			open := 0
			total := 0
			pts := 0
			if p.Summary != nil {
				open = p.Summary.OpenTasks
				total = p.Summary.TotalTasks
				pts = p.Summary.OpenPoints
			}
			fmt.Printf("%s%-12s (%-15s) %3d/%-3d open  (%3d open pts)\n",
				activeIndicator, p.Slug, p.Name, open, total, pts)
		}
		return nil
	},
}

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects (use, create)",
}

var projectUseCmd = &cobra.Command{
	Use:   "use <slug>",
	Short: "Set active project in the GUI and System Tray",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := strings.ToLower(args[0])
		c := client.NewClient(flagAPIURL)
		if !c.IsServerRunning() {
			fmt.Fprintln(os.Stderr, "⚠️  Backlog server does not appear to be running on "+flagAPIURL)
			return nil
		}

		if err := c.SetActiveProject(slug); err != nil {
			return err
		}
		fmt.Printf("✔ Active project switched to: %s\n", slug)
		return nil
	},
}

var (
	newProjName string
	newProjDesc string
)

var projectNewCmd = &cobra.Command{
	Use:   "new <slug>",
	Short: "Create a new project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := strings.ToLower(args[0])
		c := client.NewClient(flagAPIURL)
		if !c.IsServerRunning() {
			fmt.Fprintln(os.Stderr, "⚠️  Backlog server does not appear to be running on "+flagAPIURL)
			return nil
		}

		if newProjName == "" {
			newProjName = strings.Title(slug)
		}

		p, err := c.CreateProject(slug, newProjName, newProjDesc)
		if err != nil {
			return err
		}
		fmt.Printf("✔ Project '%s' (%s) created successfully.\n", p.Slug, p.Name)
		return nil
	},
}

func init() {
	projectNewCmd.Flags().StringVarP(&newProjName, "name", "n", "", "Public display name of the project")
	projectNewCmd.Flags().StringVarP(&newProjDesc, "desc", "d", "", "Project description")

	projectCmd.AddCommand(projectUseCmd)
	projectCmd.AddCommand(projectNewCmd)

	RootCmd.AddCommand(projectsCmd)
	RootCmd.AddCommand(projectCmd)
}
