package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/altenwald/backlog/pkg/client"
	"github.com/spf13/cobra"
)

var assignCmd = &cobra.Command{
	Use:   "assign <task_id> [assignee]",
	Short: "Assign a task to an agent or user handle (or empty to unassign)",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		assignee := ""
		if len(args) > 1 {
			assignee = args[1]
		}

		c := client.NewClient(flagAPIURL)
		if !c.IsServerRunning() {
			fmt.Fprintln(os.Stderr, "⚠️  Backlog server does not appear to be running on "+flagAPIURL)
			return nil
		}

		proj := resolveProject(flagProject)
		if proj == "" {
			active, err := c.GetActiveProject()
			if err != nil {
				return err
			}
			proj = active.Slug
		}

		task, err := c.AssignTask(proj, taskID, assignee)
		if err != nil {
			return err
		}

		if task.Assignee != "" {
			fmt.Printf("✔ Task #%s in '%s' assigned to @%s: %s\n", task.ID, proj, strings.TrimPrefix(task.Assignee, "@"), task.Title)
		} else {
			fmt.Printf("✔ Task #%s in '%s' unassigned: %s\n", task.ID, proj, task.Title)
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(assignCmd)
}
