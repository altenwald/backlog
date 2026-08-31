package cli

import (
	"fmt"
	"strings"

	"github.com/altenwald/backlog/pkg/client"
	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
	"github.com/spf13/cobra"
)

var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Display metric summary and point breakdown for the active project",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := client.NewClient(flagAPIURL)
		var sum *model.Summary
		var err error

		if c.IsServerRunning() {
			proj := resolveProject(flagProject)
			if proj == "" {
				active, err := c.GetActiveProject()
				if err != nil {
					return err
				}
				proj = active.Slug
			}
			sum, err = c.GetSummary(proj)
			if err != nil {
				return err
			}
		} else {
			st, err := store.NewStore(flagDataDir)
			if err != nil {
				return err
			}
			proj := resolveProject(flagProject)
			if proj == "" {
				proj = st.GetActiveProjectSlug()
			}
			sum, err = st.GetSummary(proj)
			if err != nil {
				return err
			}
		}

		fmt.Printf("📊 Backlog Summary: %s\n", strings.ToUpper(sum.ProjectName))
		fmt.Println(strings.Repeat("─", 50))
		fmt.Printf("  • Open Tasks:           %d / %d (%.1f%% completed)\n",
			sum.OpenTasks, sum.TotalTasks, float64(sum.CompletedTasks)/float64(sum.TotalTasks)*100)
		fmt.Printf("  • Pending Points:       %d pts / %d pts total\n",
			sum.OpenPoints, sum.TotalPoints)
		fmt.Println()
		fmt.Println("  Priority (Open by Tier):")
		fmt.Printf("    T1 · Blocker:      %d\n", sum.TierCounts[model.Tier1])
		fmt.Printf("    T2 · Important:    %d\n", sum.TierCounts[model.Tier2])
		fmt.Printf("    T3 · Visual debt:  %d\n", sum.TierCounts[model.Tier3])
		fmt.Printf("    T4 · Internal:     %d\n", sum.TierCounts[model.Tier4])
		fmt.Printf("    T5 · Future:       %d\n", sum.TierCounts[model.Tier5])
		fmt.Println()
		fmt.Println("  Effort Breakdown by Size:")
		for _, s := range []model.Size{model.SizeXL, model.SizeL, model.SizeM, model.SizeS, model.SizeXS} {
			fmt.Printf("    %-2s (%d pts): %d tasks\n", s, s.Points(), sum.SizeCounts[s])
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(summaryCmd)
}
