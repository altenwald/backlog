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

// Semantic colors are used as quiet tints, rather than solid blocks.
func TierColor(t model.Tier) color.Color {
	switch t {
	case model.Tier1:
		return color.NRGBA{R: 180, G: 65, B: 65, A: 255}
	case model.Tier2:
		return color.NRGBA{R: 158, G: 111, B: 40, A: 255}
	case model.Tier3:
		return color.NRGBA{R: 45, G: 128, B: 125, A: 255}
	case model.Tier4:
		return color.NRGBA{R: 108, G: 105, B: 145, A: 255}
	default:
		return color.NRGBA{R: 100, G: 115, B: 135, A: 255}
	}
}

func SizeColor(s model.Size) color.Color {
	return color.NRGBA{R: 90, G: 110, B: uint8(125 + s.Weight()), A: 255}
}

func AssigneeBadgeColor() color.Color { return color.NRGBA{R: 100, G: 115, B: 135, A: 255} }

func FormatAssignee(assignee string) string {
	name := strings.TrimSpace(assignee)
	if name == "" {
		return ""
	}
	clean := strings.TrimPrefix(name, "@")
	return "@" + clean
}

func MakeBadge(text string, bg color.Color, fg color.Color) fyne.CanvasObject {
	lbl := canvas.NewText(text, fg)
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	lbl.TextSize = 11
	lbl.Alignment = fyne.TextAlignCenter

	r, g, b, _ := bg.RGBA()
	box := canvas.NewRectangle(color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: 32})
	box.CornerRadius = 5

	return container.NewStack(box, container.NewPadded(lbl))
}

type TaskCardCallbacks struct {
	OnToggleDone func(taskID string, done bool)
	OnEdit       func(task model.Task)
	OnDelete     func(taskID string)
}

func NewTaskRow(task model.Task, callbacks TaskCardCallbacks) fyne.CanvasObject {
	check := widget.NewCheck("", func(checked bool) {
		if callbacks.OnToggleDone != nil {
			callbacks.OnToggleDone(task.ID, checked)
		}
	})
	check.Checked = task.Done

	// Badges column (fixed width)
	sizeBadge := MakeBadge(string(task.Size), SizeColor(task.Size), theme.Color(theme.ColorNameForeground))
	tierBadge := MakeBadge(task.Tier.ShortLabel(), TierColor(task.Tier), theme.Color(theme.ColorNameForeground))
	badgesCol := container.NewVBox(sizeBadge, tierBadge)

	// Title
	titleText := task.Title
	if task.ParentID != "" {
		titleText = "Subtask: " + titleText
	}
	if task.Done {
		titleText = "Completed: " + titleText
	}
	title := widget.NewLabelWithStyle(titleText, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.Wrapping = fyne.TextWrapWord

	textCol := container.NewVBox(title)

	// Collapsible Markdown Description, Resolution, and Timestamps
	descTrimmed := strings.TrimSpace(task.Description)
	resTrimmed := strings.TrimSpace(task.Resolution)

	if descTrimmed != "" || resTrimmed != "" || !task.InsertedAt.IsZero() {
		detailBox := container.NewVBox()

		if descTrimmed != "" {
			detailBox.Add(RenderMarkdown(descTrimmed))
		}

		if resTrimmed != "" {
			resHeading := widget.NewLabelWithStyle("Resolution", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			resContent := RenderMarkdown(resTrimmed)

			resBg := canvas.NewRectangle(theme.ButtonColor())
			resBg.CornerRadius = 6

			resBox := container.NewStack(resBg, container.NewPadded(container.NewVBox(resHeading, resContent)))
			detailBox.Add(resBox)
		}

		// Timestamps metadata
		var timeParts []string
		if !task.InsertedAt.IsZero() {
			timeParts = append(timeParts, fmt.Sprintf("Created: %s", task.InsertedAt.Format("2006-01-02 15:04")))
		}
		if !task.UpdatedAt.IsZero() && !task.UpdatedAt.Equal(task.InsertedAt) {
			timeParts = append(timeParts, fmt.Sprintf("Updated: %s", task.UpdatedAt.Format("2006-01-02 15:04")))
		}
		if task.TerminatedAt != nil && !task.TerminatedAt.IsZero() {
			timeParts = append(timeParts, fmt.Sprintf("Completed: %s", task.TerminatedAt.Format("2006-01-02 15:04")))
		}

		if len(timeParts) > 0 {
			timeStr := strings.Join(timeParts, "  •  ")
			timeLbl := widget.NewLabelWithStyle(timeStr, fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
			detailBox.Add(timeLbl)
		}

		mdContainer := container.NewPadded(detailBox)
		mdContainer.Hide()

		var toggleBtn *widget.Button
		expanded := false

		btnLabel := "Details"
		if task.Done && resTrimmed != "" {
			btnLabel = "Details & Resolution"
		}

		toggleBtn = widget.NewButtonWithIcon(btnLabel, theme.MenuExpandIcon(), func() {
			expanded = !expanded
			if expanded {
				toggleBtn.SetIcon(theme.MenuDropUpIcon())
				toggleBtn.SetText("Hide details")
				mdContainer.Show()
			} else {
				toggleBtn.SetIcon(theme.MenuExpandIcon())
				toggleBtn.SetText(btnLabel)
				mdContainer.Hide()
			}
		})
		toggleBtn.Importance = widget.LowImportance

		toggleRow := container.NewHBox(toggleBtn, layout.NewSpacer())
		textCol.Add(container.NewVBox(toggleRow, mdContainer))
	}

	var parentBadge fyne.CanvasObject
	if task.ParentID != "" {
		parentBadge = MakeBadge("Parent #"+task.ParentID, color.NRGBA{R: 55, G: 75, B: 105, A: 255}, theme.Color(theme.ColorNameForeground))
	}

	var depBadge fyne.CanvasObject
	if len(task.DependsOn) > 0 {
		depBadge = MakeBadge("Depends on #"+strings.Join(task.DependsOn, ",#"), color.NRGBA{R: 155, G: 60, B: 60, A: 255}, theme.Color(theme.ColorNameForeground))
	}

	// Assignee Badge (Option A: distinctive pill)
	var assigneeBadge fyne.CanvasObject
	if task.Assignee != "" {
		assigneeBadge = MakeBadge(FormatAssignee(task.Assignee), AssigneeBadgeColor(), theme.Color(theme.ColorNameForeground))
	}

	// Action buttons
	editBtn := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() {
		if callbacks.OnEdit != nil {
			callbacks.OnEdit(task)
		}
	})
	editBtn.Importance = widget.LowImportance

	deleteBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		if callbacks.OnDelete != nil {
			callbacks.OnDelete(task.ID)
		}
	})
	deleteBtn.Importance = widget.DangerImportance

	// Ensure parent badge, dep badge, assignee and buttons are always flushed cleanly to the right edge
	tagRowItems := []fyne.CanvasObject{layout.NewSpacer()}
	if parentBadge != nil {
		tagRowItems = append(tagRowItems, parentBadge)
	}
	if depBadge != nil {
		tagRowItems = append(tagRowItems, depBadge)
	}
	if assigneeBadge != nil {
		tagRowItems = append(tagRowItems, assigneeBadge)
	}
	tagRow := container.NewHBox(tagRowItems...)

	btnRow := container.NewHBox(layout.NewSpacer(), editBtn, deleteBtn)
	rightCol := container.NewVBox(tagRow, btnRow)

	rowContent := container.NewBorder(nil, nil, container.NewHBox(check, badgesCol), rightCol, textCol)

	// Theme-aware background
	cardBg := canvas.NewRectangle(theme.InputBackgroundColor())
	cardBg.CornerRadius = 8

	return container.NewStack(cardBg, container.NewPadded(rowContent))
}
