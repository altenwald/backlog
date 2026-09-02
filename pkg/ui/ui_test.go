package ui_test

import (
	"image/color"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
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

	md := "# Header 1\n## Header 2\n### Header 3\n* list item\n`inline code`\n```\nblock code\n```\nNormal text with **bold** and *italic*.\nThis is a long line of description that ends with an inline code token `client.ListTasks(param1, param2)` without breaking vertically."
	rendered := ui.RenderMarkdown(md)
	if rendered == nil {
		t.Fatal("expected RenderMarkdown to return canvas object")
	}

	w := test.NewWindow(rendered)
	w.Resize(fyne.NewSize(300, 400))
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

func TestBurnUpChart(t *testing.T) {
	_ = test.NewApp()

	chart := ui.NewBurnUpChart()
	if chart.Container == nil {
		t.Fatal("expected chart container to not be nil")
	}

	// 1. Empty tasks update
	chart.Update([]model.Task{})

	// 2. Tasks with history
	now := time.Now()
	twoDaysAgo := now.Add(-48 * time.Hour)
	oneDayAgo := now.Add(-24 * time.Hour)

	tasks := []model.Task{
		{
			ID:           "1",
			Title:        "Old task completed",
			InsertedAt:   twoDaysAgo,
			TerminatedAt: &oneDayAgo,
			Done:         true,
		},
		{
			ID:           "2",
			Title:        "Recent task completed",
			InsertedAt:   oneDayAgo,
			TerminatedAt: &now,
			Done:         true,
		},
		{
			ID:         "3",
			Title:      "Pending open task",
			InsertedAt: oneDayAgo,
			Done:       false,
		},
	}

	chart.Update(tasks)

	// 3. Test CalculateBurnUpPoints
	ptsEmpty := ui.CalculateBurnUpPoints([]model.Task{})
	if len(ptsEmpty) == 0 {
		t.Fatal("expected at least 1 point for empty tasks")
	}

	pts := ui.CalculateBurnUpPoints(tasks)
	if len(pts) < 2 {
		t.Fatalf("expected multiple points across timeline, got %d", len(pts))
	}

	firstPt := pts[0]
	lastPt := pts[len(pts)-1]

	if lastPt.Total != 3 {
		t.Fatalf("expected final total 3, got %d", lastPt.Total)
	}
	if lastPt.Completed != 2 {
		t.Fatalf("expected final completed 2, got %d", lastPt.Completed)
	}
	if firstPt.Total > lastPt.Total {
		t.Fatal("burn up total scope cannot decrease")
	}

	// 4. Test inside window with layout resize
	w := test.NewWindow(chart.Container)
	w.Resize(fyne.NewSize(400, 300))

	// 5. Test hover and tap interaction
	if h, ok := chart.ChartWidget().(desktop.Hoverable); ok {
		h.MouseIn(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(100, 50)}})
		h.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(200, 50)}})
		h.MouseOut()
	}
	if tObj, ok := chart.ChartWidget().(fyne.Tappable); ok {
		tObj.Tapped(&fyne.PointEvent{Position: fyne.NewPos(150, 50)})
	}
}

