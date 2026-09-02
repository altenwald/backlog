package ui_test

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/ui"
)

func TestFilterBar(t *testing.T) {
	_ = test.NewApp()

	var lastFilter model.TaskFilter
	fb := ui.NewFilterBar(func(f model.TaskFilter) {
		lastFilter = f
	})

	if fb.CanvasObject() == nil {
		t.Fatal("expected CanvasObject to not be nil")
	}

	// 1. Update counts with summary
	szM := model.SizeM
	tier2 := model.Tier2
	sum := &model.Summary{
		TotalTasks:      10,
		OpenTasks:       6,
		TierCounts:      map[model.Tier]int{tier2: 3},
		TotalTierCounts: map[model.Tier]int{tier2: 4},
	}
	fb.UpdateCounts(sum)

	// 2. Set size filter
	fb.SetSizeFilter(&szM)
	if lastFilter.Size == nil || *lastFilter.Size != szM {
		t.Fatalf("expected Size M filter, got %+v", lastFilter.Size)
	}

	fb.SetSizeFilter(nil)
	if lastFilter.Size != nil {
		t.Fatalf("expected nil Size filter, got %+v", lastFilter.Size)
	}
}

func TestSummaryBar(t *testing.T) {
	_ = test.NewApp()

	var selectedSize *model.Size
	sb := ui.NewSummaryBar(func(s *model.Size) {
		selectedSize = s
	})

	if sb.CanvasObject() == nil {
		t.Fatal("expected CanvasObject to not be nil")
	}

	szS := model.SizeS
	sum := &model.Summary{
		TotalTasks:     5,
		OpenTasks:      3,
		CompletedTasks: 2,
		OpenSizeCounts: map[model.Size]int{szS: 2},
		SizeCounts:     map[model.Size]int{szS: 3},
	}
	sb.Update(sum)

	sb.SetSelectedSize(&szS)
	sb.SetSelectedSize(nil)
	_ = selectedSize
}

func TestTaskDetailView(t *testing.T) {
	_ = test.NewApp()

	toggled := false
	edited := false
	deleted := false

	dv := ui.NewTaskDetailView(ui.TaskDetailCallbacks{
		OnToggleDone: func(taskID string, done bool) { toggled = true },
		OnEdit:       func(task model.Task) { edited = true },
		OnDelete:     func(taskID string) { deleted = true },
	})

	task := model.Task{
		ID:          "10",
		ParentID:    "2",
		DependsOn:   []string{"1", "3"},
		Title:       "Detailed Task",
		Description: "## Header\nSome details\n- bullet 1",
		Resolution:  "Fixed with PR #42",
		Size:        model.SizeL,
		Tier:        model.Tier1,
		Tag:         "v1.0",
		Assignee:    "manuel",
	}

	dv.ShowTask(task)
	_ = dv.CurrentTask()

	dv.Clear()
	if dv.CurrentTask() != nil {
		t.Fatal("expected CurrentTask to be nil after Clear")
	}

	_ = toggled
	_ = edited
	_ = deleted
}

func TestTaskItemAndRow(t *testing.T) {
	_ = test.NewApp()

	item := ui.NewTaskRowItem(func(taskID string, done bool) {})
	task := model.Task{
		ID:        "5",
		ParentID:  "1",
		DependsOn: []string{"2"},
		Title:     "Subtask blocked",
		Done:      false,
		Size:      model.SizeM,
		Tier:      model.Tier2,
		Assignee:  "claude",
	}

	item.Bind(task)

	// Test NewTaskRow card
	row := ui.NewTaskRow(task, ui.TaskCardCallbacks{})
	if row == nil {
		t.Fatal("expected NewTaskRow to not be nil")
	}
}

func TestMarkdownView(t *testing.T) {
	_ = test.NewApp()

	md := "# Header 1\n## Header 2\n### Header 3\n* list item\n`inline code`\n```\nblock code\n```\nNormal text with **bold** and *italic*."
	rendered := ui.RenderMarkdown(md)
	if rendered == nil {
		t.Fatal("expected RenderMarkdown to return canvas object")
	}
}

func TestBadgesAndColors(t *testing.T) {
	_ = test.NewApp()

	c1 := ui.TierColor(model.Tier1)
	c5 := ui.TierColor(model.Tier5)
	if c1 == c5 {
		t.Fatal("Tier1 and Tier5 colors should differ")
	}

	sS := ui.SizeColor(model.SizeS)
	sXL := ui.SizeColor(model.SizeXL)
	if sS == sXL {
		t.Fatal("SizeS and SizeXL colors should differ")
	}

	assigneeColor := ui.AssigneeBadgeColor()
	if assigneeColor == nil {
		t.Fatal("expected non-nil assignee badge color")
	}

	formatted := ui.FormatAssignee("@manuel")
	if formatted != "@manuel" {
		t.Fatalf("expected @manuel, got %s", formatted)
	}

	badge := ui.MakeBadge("TEST", color.White, color.Black)
	if badge == nil {
		t.Fatal("expected badge to not be nil")
	}
}
