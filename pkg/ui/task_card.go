package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/altenwald/backlog/pkg/model"
)

// Tier colors - calibrated for high contrast in both dark & light themes
func TierColor(t model.Tier) color.Color {
	switch t {
	case model.Tier1:
		return color.NRGBA{R: 225, G: 65, B: 55, A: 255} // Blocker Red
	case model.Tier2:
		return color.NRGBA{R: 225, G: 140, B: 25, A: 255} // Important Orange
	case model.Tier3:
		return color.NRGBA{R: 20, G: 145, B: 155, A: 255} // Visual Teal
	case model.Tier4:
		return color.NRGBA{R: 120, G: 115, B: 165, A: 255} // Internal Slate
	case model.Tier5:
		return color.NRGBA{R: 125, G: 135, B: 145, A: 255} // Future Gray
	default:
		return color.NRGBA{R: 110, G: 110, B: 110, A: 255}
	}
}

// Size colors - calibrated for readability
func SizeColor(s model.Size) color.Color {
	switch s {
	case model.SizeXS:
		return color.NRGBA{R: 60, G: 130, B: 215, A: 255}
	case model.SizeS:
		return color.NRGBA{R: 45, G: 110, B: 195, A: 255}
	case model.SizeM:
		return color.NRGBA{R: 35, G: 95, B: 180, A: 255}
	case model.SizeL:
		return color.NRGBA{R: 25, G: 75, B: 160, A: 255}
	case model.SizeXL:
		return color.NRGBA{R: 15, G: 55, B: 140, A: 255}
	default:
		return color.NRGBA{R: 80, G: 80, B: 80, A: 255}
	}
}

func MakeBadge(text string, bg color.Color, fg color.Color) fyne.CanvasObject {
	lbl := canvas.NewText(text, fg)
	lbl.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	lbl.TextSize = 10.5
	lbl.Alignment = fyne.TextAlignCenter

	box := canvas.NewRectangle(bg)
	box.CornerRadius = 4

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

	// Badges column
	sizeBadge := MakeBadge(string(task.Size), SizeColor(task.Size), color.White)
	tierBadge := MakeBadge(task.Tier.ShortLabel(), TierColor(task.Tier), color.White)
	badgesCol := container.NewVBox(sizeBadge, tierBadge)

	// Title & Description
	titleText := task.Title
	if task.Done {
		titleText = "✓ " + titleText
	}
	title := widget.NewLabelWithStyle(titleText, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.Wrapping = fyne.TextWrapWord

	desc := widget.NewLabel(task.Description)
	desc.Wrapping = fyne.TextWrapWord
	desc.TextStyle = fyne.TextStyle{Italic: task.Done}

	tagText := ""
	if task.Group != "" {
		tagText = "[" + task.Group + "]"
	}
	if task.Tag != "" {
		tagText = tagText + " " + task.Tag
	}
	tagLabel := widget.NewLabelWithStyle(tagText, fyne.TextAlignTrailing, fyne.TextStyle{Monospace: true})

	textCol := container.NewVBox(title, desc)

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

	rightCol := container.NewVBox(tagLabel, container.NewHBox(editBtn, deleteBtn))

	rowContent := container.NewBorder(nil, nil, container.NewHBox(check, badgesCol), rightCol, textCol)

	// Use theme-aware background color for clean light and dark mode rendering
	cardBg := canvas.NewRectangle(theme.InputBackgroundColor())
	cardBg.CornerRadius = 8

	return container.NewStack(cardBg, container.NewPadded(rowContent))
}
