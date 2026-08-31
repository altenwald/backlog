package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/altenwald/backlog/pkg/client"
	"github.com/altenwald/backlog/pkg/model"
	"github.com/spf13/cobra"
)

var (
	flagTier   int
	flagGroup  string
	flagDone   string // "true", "false", "all"
	flagSearch string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks in the active project",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.NewClient(flagAPIURL)
		if !c.IsServerRunning() {
			fmt.Fprintln(os.Stderr, "⚠️  Backlog server does not appear to be running on "+flagAPIURL)
			fmt.Fprintln(os.Stderr, "   Start it with 'backlog' or 'backlog start'")
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

		filter := model.TaskFilter{
			Search: flagSearch,
		}
		if flagTier >= 1 && flagTier <= 5 {
			t := model.Tier(flagTier)
			filter.Tier = &t
		}
		if flagGroup != "" {
			filter.Group = &flagGroup
		}
		if flagDone == "true" {
			d := true
			filter.Done = &d
		} else if flagDone == "false" || flagDone == "" {
			d := false
			filter.Done = &d
		}

		tasks, err := c.ListTasks(proj, filter)
		if err != nil {
			return err
		}

		sum, _ := c.GetSummary(proj)
		fmt.Printf("📂 Project: %s (%d/%d open · %d points)\n", strings.ToUpper(proj), sum.OpenTasks, sum.TotalTasks, sum.OpenPoints)
		fmt.Println(strings.Repeat("─", 72))

		if len(tasks) == 0 {
			fmt.Println("  (No tasks match the filter criteria)")
			return nil
		}

		for _, t := range tasks {
			statusIcon := "[ ]"
			if t.Done {
				statusIcon = "[✓]"
			}
			tag := ""
			if t.Group != "" {
				tag = fmt.Sprintf(" [%s]", t.Group)
			}
			if t.Tag != "" {
				tag += fmt.Sprintf(" (%s)", t.Tag)
			}

			fmt.Printf("%s #%-3s [%-2s] [%-2s] %-40s%s\n",
				statusIcon, t.ID, t.Size, t.Tier.ShortLabel(), t.Title, tag)
		}

		return nil
	},
}

func init() {
	listCmd.Flags().IntVarP(&flagTier, "tier", "t", 0, "Filter by priority Tier (1..5)")
	listCmd.Flags().StringVarP(&flagGroup, "group", "g", "", "Filter by group/category")
	listCmd.Flags().StringVarP(&flagDone, "done", "d", "false", "Filter by status: 'false' (open), 'true' (completed), 'all'")
	listCmd.Flags().StringVarP(&flagSearch, "search", "s", "", "Search query term")
	RootCmd.AddCommand(listCmd)
}
