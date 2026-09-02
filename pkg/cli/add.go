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
	addTitle      string
	addDesc       string
	addGroup      string
	addSize       string
	addTier       int
	addTag        string
	addResolution string
	addAssignee   string
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new task to the project",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(addTitle) == "" {
			if len(args) > 0 {
				addTitle = strings.Join(args, " ")
			} else {
				return fmt.Errorf("must specify a title via --title or as an argument")
			}
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

		tier := model.Tier3
		if addTier >= 1 && addTier <= 5 {
			tier = model.Tier(addTier)
		}

		size := model.SizeM
		if addSize != "" {
			size = model.Size(strings.ToUpper(addSize))
		}

		task, err := c.AddTask(proj, model.Task{
			Title:       addTitle,
			Description: addDesc,
			Group:       addGroup,
			Size:        size,
			Tier:        tier,
			Tag:         addTag,
			Resolution:  addResolution,
			Assignee:    addAssignee,
		})
		if err != nil {
			return err
		}

		sum, _ := c.GetSummary(proj)
		fmt.Printf("✔ Task #%s created in '%s' [%s] [%s]: %s\n", task.ID, proj, task.Size, task.Tier.ShortLabel(), task.Title)
		fmt.Printf("  Current status: %d/%d open\n", sum.OpenTasks, sum.TotalTasks)
		return nil
	},
}

func init() {
	addCmd.Flags().StringVarP(&addTitle, "title", "T", "", "Task title")
	addCmd.Flags().StringVarP(&addDesc, "desc", "D", "", "Detailed description or context")
	addCmd.Flags().StringVarP(&addGroup, "group", "g", "General", "Group or category")
	addCmd.Flags().StringVarP(&addSize, "size", "S", "M", "Effort size (XS, S, M, L, XL)")
	addCmd.Flags().IntVarP(&addTier, "tier", "t", 3, "Priority Tier (1=Blocker, 2=Important, 3=Visual debt, 4=Internal, 5=Future)")
	addCmd.Flags().StringVar(&addTag, "tag", "", "Tag or reference label")
	addCmd.Flags().StringVarP(&addResolution, "resolution", "r", "", "Summary of implementation details or resolution")
	addCmd.Flags().StringVarP(&addAssignee, "assignee", "a", "", "Assignee handle (e.g. 'claude', 'manuel')")
	RootCmd.AddCommand(addCmd)
}
