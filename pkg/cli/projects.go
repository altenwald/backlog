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
				open := 0
				total := 0
				if p.Summary != nil {
					open = p.Summary.OpenTasks
					total = p.Summary.TotalTasks
				}
				fmt.Printf("  %-12s (%-15s) %3d/%-3d open\n",
					p.Slug, p.Name, open, total)
			}
		} else {
			st, err := store.NewStore(flagDataDir)
			if err != nil {
				return err
			}
			for _, p := range st.ListProjects() {
				sum, _ := st.GetSummary(p.Slug)
				open := 0
				total := 0
				if sum != nil {
					open = sum.OpenTasks
					total = sum.TotalTasks
				}
				fmt.Printf("  %-12s (%-15s) %3d/%-3d open\n",
					p.Slug, p.Name, open, total)
			}
		}

		return nil
	},
}

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects (new, delete)",
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

	projectCmd.AddCommand(projectNewCmd)
	projectCmd.AddCommand(projectDeleteCmd)

	RootCmd.AddCommand(projectsCmd)
	RootCmd.AddCommand(projectCmd)
}
