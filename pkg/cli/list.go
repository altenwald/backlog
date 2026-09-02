package cli

import (
	"fmt"
	"strings"

	"github.com/altenwald/backlog/pkg/client"
	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
	"github.com/spf13/cobra"
)

var (
	flagTier     int
	flagParentID string
	flagBlocked  string // "true", "false", ""
	flagDone     string // "true", "false", "all"
	flagAssignee string
	flagSearch   string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks in the active project",
	RunE: func(cmd *cobra.Command, args []string) error {
		filter := model.TaskFilter{
			Search: flagSearch,
		}
		if flagTier >= 1 && flagTier <= 5 {
			t := model.Tier(flagTier)
			filter.Tier = &t
		}
		if flagParentID != "" {
			filter.ParentID = &flagParentID
		}
		if flagBlocked == "true" {
			b := true
			filter.Blocked = &b
		} else if flagBlocked == "false" {
			b := false
			filter.Blocked = &b
		}
		if flagDone == "true" {
			d := true
			filter.Done = &d
		} else if flagDone == "false" || flagDone == "" {
			d := false
			filter.Done = &d
		}
		if flagAssignee != "" {
			filter.Assignee = &flagAssignee
		}

		c := client.NewClient(flagAPIURL)
		var tasks []model.Task
		var sum *model.Summary
		var proj string
		var err error

		if c.IsServerRunning() {
			proj = resolveProject(flagProject)
			if proj == "" {
				active, err := c.GetActiveProject()
				if err != nil {
					return err
				}
				proj = active.Slug
			}
			tasks, err = c.ListTasks(proj, filter)
			if err != nil {
				return err
			}
			sum, _ = c.GetSummary(proj)
		} else {
			st, err := store.NewStore(flagDataDir)
			if err != nil {
				return err
			}
			proj = resolveProject(flagProject)
			if proj == "" {
				proj = st.GetActiveProjectSlug()
			}
			tasks, err = st.ListTasks(proj, filter)
			if err != nil {
				return err
			}
			sum, _ = st.GetSummary(proj)
		}

		fmt.Printf("📂 Project: %s (%d/%d open)\n", strings.ToUpper(proj), sum.OpenTasks, sum.TotalTasks)
		fmt.Println(strings.Repeat("─", 72))

		if len(tasks) == 0 {
			fmt.Println("  (No tasks match the filter criteria)")
			return nil
		}

		printTasksHierarchically(tasks)
		return nil
	},
}

func printTasksHierarchically(tasks []model.Task) {
	taskMap := make(map[string]model.Task)
	childrenMap := make(map[string][]model.Task)
	isChild := make(map[string]bool)

	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	for _, t := range tasks {
		if t.ParentID != "" {
			if _, exists := taskMap[t.ParentID]; exists {
				childrenMap[t.ParentID] = append(childrenMap[t.ParentID], t)
				isChild[t.ID] = true
			}
		}
	}

	var printNode func(t model.Task, depth int)
	printNode = func(t model.Task, depth int) {
		statusIcon := "[ ]"
		if t.Done {
			statusIcon = "[✓]"
		}
		indent := strings.Repeat("  ", depth)
		prefix := ""
		if depth > 0 {
			prefix = "↳ "
		}

		blockTag := ""
		if t.IsBlocked(taskMap) {
			blockingIDs := t.BlockingTaskIDs(taskMap)
			blockTag = fmt.Sprintf(" ⛔[blocked by #%s]", strings.Join(blockingIDs, ", #"))
		}

		tag := ""
		if t.Tag != "" {
			tag += fmt.Sprintf(" (%s)", t.Tag)
		}
		if t.Assignee != "" {
			tag += fmt.Sprintf(" @%s", strings.TrimPrefix(t.Assignee, "@"))
		}

		fmt.Printf("%s%s%s #%-3s [%-2s] [%-2s] %s%s%s\n",
			indent, prefix, statusIcon, t.ID, t.Size, t.Tier.ShortLabel(), t.Title, blockTag, tag)

		for _, child := range childrenMap[t.ID] {
			printNode(child, depth+1)
		}
	}

	for _, t := range tasks {
		if !isChild[t.ID] {
			printNode(t, 0)
		}
	}
}

func init() {
	listCmd.Flags().IntVarP(&flagTier, "tier", "t", 0, "Filter by priority Tier (1..5)")
	listCmd.Flags().StringVarP(&flagParentID, "parent", "P", "", "Filter by parent task ID")
	listCmd.Flags().StringVar(&flagBlocked, "blocked", "", "Filter by blocked state: true (blocked), false (actionable)")
	listCmd.Flags().StringVarP(&flagDone, "status", "s", "false", "Filter by status: true (done), false (open), all")
	listCmd.Flags().StringVarP(&flagAssignee, "assignee", "a", "", "Filter by assignee handle (e.g. 'claude', 'unassigned')")
	listCmd.Flags().StringVarP(&flagSearch, "search", "q", "", "Search by keyword in title/desc")
	RootCmd.AddCommand(listCmd)
}
