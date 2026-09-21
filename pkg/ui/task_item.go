package ui

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/altenwald/backlog/pkg/model"
)

type TaskRowItem struct {
	widget.BaseWidget
	container     *fyne.Container
	check         *widget.Check
	title         *widget.Label
	metadata      *widget.Label
	dependency    *widget.Icon
	tier          *fyne.Container
	indent        *canvas.Rectangle
	branch        *widget.Icon
	currentTaskID string
	onToggleDone  func(taskID string, done bool)
}

func NewTaskRowItem(onToggleDone func(taskID string, done bool)) *TaskRowItem {
	item := &TaskRowItem{onToggleDone: onToggleDone}
	item.ExtendBaseWidget(item)
	item.check = widget.NewCheck("", func(checked bool) {
		if item.onToggleDone != nil && item.currentTaskID != "" {
			item.onToggleDone(item.currentTaskID, checked)
		}
	})
	item.title = widget.NewLabel("Task title")
	item.title.Truncation = fyne.TextTruncateEllipsis
	item.metadata = widget.NewLabel("Task metadata")
	item.metadata.Truncation = fyne.TextTruncateEllipsis
	item.metadata.Importance = widget.LowImportance
	item.dependency = widget.NewIcon(DependencyIcon())
	item.tier = container.NewStack(MakeBadge("T3", TierColor(model.Tier3), theme.Color(theme.ColorNameForeground)))
	secondary := container.NewBorder(nil, nil, item.dependency, nil, item.metadata)
	text := container.NewVBox(item.title, secondary)
	item.indent = canvas.NewRectangle(color.Transparent)
	item.branch = widget.NewIcon(branchIcon())
	item.indent.Hide()
	item.branch.Hide()
	leading := container.NewHBox(item.indent, container.NewCenter(item.branch), container.NewCenter(item.check))
	item.container = container.NewPadded(container.NewBorder(nil, nil, leading, container.NewCenter(item.tier), text))
	return item
}

func (item *TaskRowItem) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewThemeOverride(item.container, typographyTheme{Theme: fyne.CurrentApp().Settings().Theme(), inset: 4}))
}

func (item *TaskRowItem) Bind(task model.Task) {
	item.bindPlacement(task, taskPlacement{})
}

func (item *TaskRowItem) bindPlacement(task model.Task, placement taskPlacement) {
	item.indent.Hide()
	item.branch.Hide()
	if placement.Depth > 0 {
		item.branch.Show()
		// Align each branch icon with the preceding level's checkbox.
		// Keep a small initial inset, then one icon-and-checkbox step per level.
		// Cap very deep chains so titles remain readable in a narrow pane.
		item.indent.SetMinSize(fyne.NewSize(4+float32(min(placement.Depth-1, 4))*30, 1))
		item.indent.Show()
	}
	item.currentTaskID = task.ID
	item.check.Checked = task.Done
	item.check.Refresh()
	item.title.SetText(task.Title)
	item.title.TextStyle = fyne.TextStyle{Bold: !task.Done}
	item.title.Importance = widget.MediumImportance
	if task.Done {
		item.title.Importance = widget.LowImportance
	}
	item.title.Refresh()
	parts := []string{"#" + task.ID}
	if len(task.DependsOn) > 0 {
		parts = append(parts, "Depends on #"+strings.Join(task.DependsOn, ", #"))
		item.dependency.Show()
	} else {
		item.dependency.Hide()
	}
	if task.ParentID != "" {
		parts = append(parts, "Parent #"+task.ParentID)
	}
	parts = append(parts, string(task.Size))
	if task.Assignee != "" {
		parts = append(parts, FormatAssignee(task.Assignee))
	}

	if task.Deprecated {
		parts = append(parts, "Deprecated")
	} else if task.Done {
		parts = append(parts, "Completed")
	}
	item.metadata.SetText(strings.Join(parts, "  ·  "))
	item.tier.Objects = []fyne.CanvasObject{MakeBadge(task.Tier.ShortLabel(), TierColor(task.Tier), theme.Color(theme.ColorNameForeground))}
	item.tier.Refresh()
	item.container.Refresh()
}
