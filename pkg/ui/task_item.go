package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/altenwald/backlog/pkg/model"
)

type TaskRowItem struct {
	widget.BaseWidget

	container  *fyne.Container
	check      *widget.Check
	tierBox    *canvas.Rectangle
	tierText   *canvas.Text
	sizeBox    *canvas.Rectangle
	sizeText   *canvas.Text
	title      *widget.Label
	assignWrap *fyne.Container
	assignBox  *canvas.Rectangle
	assignText *canvas.Text

	currentTaskID string
	onToggleDone  func(taskID string, done bool)
}

func NewTaskRowItem(onToggleDone func(taskID string, done bool)) *TaskRowItem {
	item := &TaskRowItem{
		onToggleDone: onToggleDone,
	}
	item.ExtendBaseWidget(item)

	item.check = widget.NewCheck("", func(checked bool) {
		if item.onToggleDone != nil && item.currentTaskID != "" {
			item.onToggleDone(item.currentTaskID, checked)
		}
	})

	// Tier badge
	item.tierText = canvas.NewText("T3", color.White)
	item.tierText.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	item.tierText.TextSize = 10
	item.tierText.Alignment = fyne.TextAlignCenter
	item.tierBox = canvas.NewRectangle(TierColor(model.Tier3))
	item.tierBox.CornerRadius = 4
	tierBadge := container.NewStack(item.tierBox, container.NewPadded(item.tierText))

	// Size badge
	item.sizeText = canvas.NewText("M", color.White)
	item.sizeText.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	item.sizeText.TextSize = 10
	item.sizeText.Alignment = fyne.TextAlignCenter
	item.sizeBox = canvas.NewRectangle(SizeColor(model.SizeM))
	item.sizeBox.CornerRadius = 4
	sizeBadge := container.NewStack(item.sizeBox, container.NewPadded(item.sizeText))

	badges := container.NewHBox(tierBadge, sizeBadge)

	item.title = widget.NewLabel("Sample Task Title")
	item.title.Truncation = fyne.TextTruncateEllipsis
	item.title.TextStyle = fyne.TextStyle{Bold: true}

	// Assignee badge
	item.assignText = canvas.NewText("@user", color.White)
	item.assignText.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	item.assignText.TextSize = 9.5
	item.assignText.Alignment = fyne.TextAlignCenter
	item.assignBox = canvas.NewRectangle(AssigneeBadgeColor())
	item.assignBox.CornerRadius = 4
	item.assignWrap = container.NewStack(item.assignBox, container.NewPadded(item.assignText))

	rightSide := container.NewHBox(layout.NewSpacer(), item.assignWrap)
	leftSide := container.NewHBox(item.check, badges)

	content := container.NewBorder(nil, nil, leftSide, rightSide, item.title)
	item.container = container.NewPadded(content)

	return item
}

func (item *TaskRowItem) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(item.container)
}

func (item *TaskRowItem) Bind(task model.Task) {
	item.currentTaskID = task.ID

	item.check.Checked = task.Done
	item.check.Refresh()

	prefix := ""
	if task.ParentID != "" {
		prefix = "↳ "
	}
	displayTitle := prefix + "#" + task.ID + "  " + task.Title
	if task.Done {
		item.title.TextStyle = fyne.TextStyle{Italic: true}
	} else {
		item.title.TextStyle = fyne.TextStyle{Bold: true}
	}
	item.title.SetText(displayTitle)

	// Tier badge
	item.tierText.Text = task.Tier.ShortLabel()
	item.tierBox.FillColor = TierColor(task.Tier)
	item.tierText.Refresh()
	item.tierBox.Refresh()

	// Size badge
	item.sizeText.Text = string(task.Size)
	item.sizeBox.FillColor = SizeColor(task.Size)
	item.sizeText.Refresh()
	item.sizeBox.Refresh()

	// Assignee badge
	if task.Assignee != "" {
		item.assignText.Text = FormatAssignee(task.Assignee)
		item.assignWrap.Show()
		item.assignText.Refresh()
		item.assignBox.Refresh()
	} else {
		item.assignWrap.Hide()
	}
}
