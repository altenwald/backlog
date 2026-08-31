package cli

import (
	"fmt"
	"os"

	"github.com/altenwald/backlog/pkg/client"
	"github.com/spf13/cobra"
)

var doneCmd = &cobra.Command{
	Use:   "done <task_id>",
	Short: "Mark a task as completed",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
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

		task, err := c.CompleteTask(proj, taskID, true)
		if err != nil {
			return err
		}

		sum, _ := c.GetSummary(proj)
		fmt.Printf("✔ Task #%s completed in '%s': %s\n", task.ID, proj, task.Title)
		fmt.Printf("  Remaining open tasks: %d/%d (%d pts)\n", sum.OpenTasks, sum.TotalTasks, sum.OpenPoints)
		return nil
	},
}

var undoneCmd = &cobra.Command{
	Use:   "undone <task_id>",
	Short: "Reopen a completed task as pending",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
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

		task, err := c.CompleteTask(proj, taskID, false)
		if err != nil {
			return err
		}

		sum, _ := c.GetSummary(proj)
		fmt.Printf("↺ Task #%s reopened as pending in '%s': %s\n", task.ID, proj, task.Title)
		fmt.Printf("  Remaining open tasks: %d/%d (%d pts)\n", sum.OpenTasks, sum.TotalTasks, sum.OpenPoints)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(doneCmd)
	RootCmd.AddCommand(undoneCmd)
}
