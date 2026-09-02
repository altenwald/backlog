package ui

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/altenwald/backlog/pkg/model"
)

type TaskDetailCallbacks struct {
	OnToggleDone func(taskID string, done bool)
	OnEdit       func(task model.Task)
	OnDelete     func(taskID string)
}

type TaskDetailView struct {
	Container   *fyne.Container
	placeholder *fyne.Container
	contentWrap *fyne.Container

	titleLabel    *widget.Label
	toggleDoneBtn *widget.Button
	editBtn       *widget.Button
	deleteBtn     *widget.Button

	badgesRow   *fyne.Container
	timeLabel   *widget.Label
	descContent *fyne.Container
	resBox      *fyne.Container
	resContent  *fyne.Container

	currentTask *model.Task
	callbacks   TaskDetailCallbacks
}

func NewTaskDetailView(callbacks TaskDetailCallbacks) *TaskDetailView {
	dv := &TaskDetailView{
		callbacks: callbacks,
	}

	// Placeholder when no task is selected
	icon := widget.NewIcon(theme.DocumentIcon())
	msg := widget.NewLabelWithStyle("Select a task to inspect details", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	hint := widget.NewLabelWithStyle("Use ↑ / ↓ arrow keys to quickly navigate between tasks", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
	dv.placeholder = container.NewCenter(container.NewVBox(icon, msg, hint))

	// Title
	dv.titleLabel = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	dv.titleLabel.Wrapping = fyne.TextWrapWord

	// Action buttons
	dv.toggleDoneBtn = widget.NewButtonWithIcon("Mark as Done", theme.ConfirmIcon(), func() {
		if dv.currentTask != nil && dv.callbacks.OnToggleDone != nil {
			dv.callbacks.OnToggleDone(dv.currentTask.ID, !dv.currentTask.Done)
		}
	})

	dv.editBtn = widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), func() {
		if dv.currentTask != nil && dv.callbacks.OnEdit != nil {
			dv.callbacks.OnEdit(*dv.currentTask)
		}
	})
	dv.editBtn.Importance = widget.LowImportance

	dv.deleteBtn = widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
		if dv.currentTask != nil && dv.callbacks.OnDelete != nil {
			dv.callbacks.OnDelete(dv.currentTask.ID)
		}
	})
	dv.deleteBtn.Importance = widget.DangerImportance

	btnBar := container.NewHBox(dv.toggleDoneBtn, dv.editBtn, dv.deleteBtn, layout.NewSpacer())

	// Badges row & dates
	dv.badgesRow = container.NewHBox()
	dv.timeLabel = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Italic: true})

	// Body content
	dv.descContent = container.NewVBox()

	dv.resContent = container.NewVBox()
	resHeading := widget.NewLabelWithStyle("✔ Resolution & Implementation Notes", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	resBg := canvas.NewRectangle(theme.ButtonColor())
	resBg.CornerRadius = 8
	dv.resBox = container.NewStack(resBg, container.NewPadded(container.NewVBox(resHeading, dv.resContent)))

	scrollableBody := container.NewVBox(
		widget.NewLabelWithStyle("Description", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		dv.descContent,
		widget.NewSeparator(),
		dv.resBox,
	)
	scroll := container.NewVScroll(container.NewPadded(scrollableBody))

	topArea := container.NewVBox(
		dv.titleLabel,
		dv.badgesRow,
		dv.timeLabel,
		btnBar,
		widget.NewSeparator(),
	)

	dv.contentWrap = container.NewBorder(topArea, nil, nil, nil, scroll)
	dv.contentWrap.Hide()

	dv.Container = container.NewStack(dv.placeholder, dv.contentWrap)

	return dv
}

func (dv *TaskDetailView) CurrentTask() *model.Task {
	return dv.currentTask
}

func (dv *TaskDetailView) Clear() {
	dv.currentTask = nil
	dv.contentWrap.Hide()
	dv.placeholder.Show()
	dv.Container.Refresh()
}

func (dv *TaskDetailView) ShowTask(task model.Task) {
	dv.currentTask = &task
	dv.placeholder.Hide()
	dv.contentWrap.Show()

	// Title
	if task.Done {
		dv.toggleDoneBtn.SetText("Reopen Task")
		dv.toggleDoneBtn.SetIcon(theme.ViewRefreshIcon())
		dv.toggleDoneBtn.Importance = widget.LowImportance
	} else {
		dv.toggleDoneBtn.SetText("Mark as Done")
		dv.toggleDoneBtn.SetIcon(theme.ConfirmIcon())
		dv.toggleDoneBtn.Importance = widget.HighImportance
	}
	dv.titleLabel.SetText(fmt.Sprintf("#%s: %s", task.ID, task.Title))

	// Badges
	dv.badgesRow.Objects = nil
	tierBadge := MakeBadge(task.Tier.Label(), TierColor(task.Tier), color.White)
	sizeBadge := MakeBadge(string(task.Size), SizeColor(task.Size), color.White)
	dv.badgesRow.Add(tierBadge)
	dv.badgesRow.Add(sizeBadge)

	if task.ParentID != "" {
		parentBadge := MakeBadge("↳ Parent #"+task.ParentID, color.NRGBA{R: 55, G: 75, B: 105, A: 255}, color.White)
		dv.badgesRow.Add(parentBadge)
	}

	if len(task.DependsOn) > 0 {
		depBadge := MakeBadge("⛔ Depends on #"+strings.Join(task.DependsOn, ", #"), color.NRGBA{R: 160, G: 65, B: 65, A: 255}, color.White)
		dv.badgesRow.Add(depBadge)
	}

	if task.Tag != "" {
		tagBadge := MakeBadge(task.Tag, color.NRGBA{R: 90, G: 90, B: 100, A: 255}, color.White)
		dv.badgesRow.Add(tagBadge)
	}

	if task.Assignee != "" {
		assignBadge := MakeBadge(FormatAssignee(task.Assignee), AssigneeBadgeColor(), color.White)
		dv.badgesRow.Add(assignBadge)
	}
	dv.badgesRow.Refresh()

	// Timestamps
	var timeParts []string
	if !task.InsertedAt.IsZero() {
		timeParts = append(timeParts, fmt.Sprintf("Inserted: %s", task.InsertedAt.Format("2006-01-02 15:04")))
	}
	if !task.UpdatedAt.IsZero() && !task.UpdatedAt.Equal(task.InsertedAt) {
		timeParts = append(timeParts, fmt.Sprintf("Updated: %s", task.UpdatedAt.Format("2006-01-02 15:04")))
	}
	if task.TerminatedAt != nil && !task.TerminatedAt.IsZero() {
		timeParts = append(timeParts, fmt.Sprintf("Completed: %s", task.TerminatedAt.Format("2006-01-02 15:04")))
	}
	dv.timeLabel.SetText(strings.Join(timeParts, "   ·   "))

	// Description
	dv.descContent.Objects = nil
	descTrimmed := strings.TrimSpace(task.Description)
	if descTrimmed != "" {
		dv.descContent.Add(RenderMarkdown(descTrimmed))
	} else {
		dv.descContent.Add(widget.NewLabelWithStyle("(No description provided)", fyne.TextAlignLeading, fyne.TextStyle{Italic: true}))
	}
	dv.descContent.Refresh()

	// Resolution
	resTrimmed := strings.TrimSpace(task.Resolution)
	if resTrimmed != "" {
		dv.resContent.Objects = nil
		dv.resContent.Add(RenderMarkdown(resTrimmed))
		dv.resBox.Show()
	} else {
		dv.resBox.Hide()
	}
	dv.resContent.Refresh()

	dv.Container.Refresh()
}
