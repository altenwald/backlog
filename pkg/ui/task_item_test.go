package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/altenwald/backlog/pkg/model"
)

// List rows are recycled: status, icons and callback targets must follow the
// newly bound task, without triggering completion while scrolling or filtering.
func TestTaskRowRebind(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	var toggledID string
	row := NewTaskRowItem(func(id string, done bool) { toggledID = id })
	row.bindPlacement(model.Task{ID: "84", Title: "Dependent task", DependsOn: []string{"80"}, Deprecated: true, Done: true, Assignee: "claude", Size: model.SizeM, Tier: model.Tier2}, taskPlacement{Depth: 2, AnchorID: "80"})
	if !row.branch.Visible() || !row.indent.Visible() || !row.dependency.Visible() || !strings.Contains(row.metadata.Text, "Depends on #80") {
		t.Fatal("dependency missing")
	}
	row.Bind(model.Task{ID: "90", Title: "Independent task", Size: model.SizeS, Tier: model.Tier3})
	if row.branch.Visible() || row.indent.Visible() || row.dependency.Visible() || row.check.Checked || strings.Contains(row.metadata.Text, "claude") || strings.Contains(row.metadata.Text, "Deprecated") {
		t.Fatal("recycled row retained old task state")
	}
	if toggledID != "" {
		t.Fatal("binding a row must not complete a task")
	}
	test.Tap(row.check)
	if toggledID != "90" {
		t.Fatalf("completion targeted %q, want 90", toggledID)
	}
}

func TestFilterCountsIncludeCompleted(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	fb := NewFilterBar(nil)
	fb.UpdateCounts(&model.Summary{OpenTasks: 2, TotalTasks: 5, TierCounts: map[model.Tier]int{model.Tier1: 1}, TotalTierCounts: map[model.Tier]int{model.Tier1: 3}})
	if fb.buttons[0].Text != "All (2)" {
		t.Fatal("open count missing")
	}
	test.Tap(fb.hideDoneCheck)
	if fb.buttons[0].Text != "All (5)" || fb.buttons[1].Text != "T1 (3)" {
		t.Fatal("counts must include completed tasks when visible")
	}
	test.Tap(fb.hideDoneCheck)
	if fb.buttons[0].Text != "All (2)" || fb.buttons[1].Text != "T1 (1)" {
		t.Fatal("counts must return to open tasks")
	}
}
