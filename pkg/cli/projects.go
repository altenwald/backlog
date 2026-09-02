package cli

import (
	"fmt"
	"strings"

	"github.com/altenwald/backlog/pkg/client"
	"github.com/altenwald/backlog/pkg/store"
	"github.com/spf13/cobra"
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List all projects and their status",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.NewClient(flagAPIURL)
		fmt.Println("📂 Registered Projects in Backlog:")
		fmt.Println(strings.Repeat("─", 60))

		if c.IsServerRunning() {
			projects, err := c.ListProjects()
			if err != nil {
				return err
			}
			for _, p := range projects {
				activeIndicator := "  "
				if p.Active {
					activeIndicator = "● "
				}
				open := 0
				total := 0
				if p.Summary != nil {
					open = p.Summary.OpenTasks
					total = p.Summary.TotalTasks
				}
				fmt.Printf("%s%-12s (%-15s) %3d/%-3d open\n",
					activeIndicator, p.Slug, p.Name, open, total)
			}
		} else {
			st, err := store.NewStore(flagDataDir)
			if err != nil {
				return err
			}
			activeSlug := st.GetActiveProjectSlug()
			for _, p := range st.ListProjects() {
				activeIndicator := "  "
				if p.Slug == activeSlug {
					activeIndicator = "● "
				}
				sum, _ := st.GetSummary(p.Slug)
				open := 0
				total := 0
				if sum != nil {
					open = sum.OpenTasks
					total = sum.TotalTasks
				}
				fmt.Printf("%s%-12s (%-15s) %3d/%-3d open\n",
					activeIndicator, p.Slug, p.Name, open, total)
			}
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
		if c.IsServerRunning() {
			if err := c.SetActiveProject(slug); err != nil {
				return err
			}
		} else {
			st, err := store.NewStore(flagDataDir)
			if err != nil {
				return err
			}
			if err := st.SetActiveProject(slug); err != nil {
				return err
			}
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
		if newProjName == "" {
			newProjName = strings.Title(slug)
		}

		c := client.NewClient(flagAPIURL)
		if c.IsServerRunning() {
			p, err := c.CreateProject(slug, newProjName, newProjDesc)
			if err != nil {
				return err
			}
			fmt.Printf("✔ Project '%s' (%s) created successfully.\n", p.Slug, p.Name)
		} else {
			st, err := store.NewStore(flagDataDir)
			if err != nil {
				return err
			}
			p, err := st.CreateProject(slug, newProjName, newProjDesc)
			if err != nil {
				return err
			}
			fmt.Printf("✔ Project '%s' (%s) created successfully.\n", p.Slug, p.Name)
		}
		return nil
	},
}

var projectDeleteCmd = &cobra.Command{
	Use:     "delete <slug>",
	Aliases: []string{"rm"},
	Short:   "Delete a project and all its tasks",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slug := strings.ToLower(args[0])
		c := client.NewClient(flagAPIURL)
		if c.IsServerRunning() {
			if err := c.DeleteProject(slug); err != nil {
				return err
			}
		} else {
			st, err := store.NewStore(flagDataDir)
			if err != nil {
				return err
			}
			if err := st.DeleteProject(slug); err != nil {
				return err
			}
		}
		fmt.Printf("✔ Project '%s' deleted successfully.\n", slug)
		return nil
	},
}

func init() {
	projectNewCmd.Flags().StringVarP(&newProjName, "name", "n", "", "Public display name of the project")
	projectNewCmd.Flags().StringVar(&newProjDesc, "desc", "", "Project description")

	projectCmd.AddCommand(projectUseCmd)
	projectCmd.AddCommand(projectNewCmd)
	projectCmd.AddCommand(projectDeleteCmd)

	RootCmd.AddCommand(projectsCmd)
	RootCmd.AddCommand(projectCmd)
}
