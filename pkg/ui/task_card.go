package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/altenwald/backlog/pkg/model"
)

// Tier colors
func TierColor(t model.Tier) color.Color {
	switch t {
	case model.Tier1:
		return color.NRGBA{R: 218, G: 68, B: 54, A: 255} // Red
	case model.Tier2:
		return color.NRGBA{R: 217, G: 140, B: 30, A: 255} // Orange/Gold
	case model.Tier3:
		return color.NRGBA{R: 31, G: 122, B: 140, A: 255} // Cyan/Teal
	case model.Tier4:
		return color.NRGBA{R: 110, G: 110, B: 150, A: 255} // Purple/Slate
	case model.Tier5:
		return color.NRGBA{R: 139, G: 150, B: 168, A: 255} // Gray
	default:
		return color.NRGBA{R: 100, G: 100, B: 100, A: 255}
	}
}

// Size colors
func SizeColor(s model.Size) color.Color {
	switch s {
	case model.SizeXS:
		return color.NRGBA{R: 70, G: 130, B: 200, A: 255}
	case model.SizeS:
		return color.NRGBA{R: 50, G: 110, B: 180, A: 255}
	case model.SizeM:
		return color.NRGBA{R: 59, G: 125, B: 212, A: 255}
	case model.SizeL:
		return color.NRGBA{R: 35, G: 80, B: 150, A: 255}
	case model.SizeXL:
		return color.NRGBA{R: 20, G: 40, B: 80, A: 255}
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

	// Title & Desc
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
	editBtn := widget.NewButtonWithIcon("", nil, func() {
		if callbacks.OnEdit != nil {
			callbacks.OnEdit(task)
		}
	})
	editBtn.Importance = widget.LowImportance

	deleteBtn := widget.NewButton("✕", func() {
		if callbacks.OnDelete != nil {
			callbacks.OnDelete(task.ID)
		}
	})
	deleteBtn.Importance = widget.DangerImportance

	rightCol := container.NewVBox(tagLabel, container.NewHBox(editBtn, deleteBtn))

	rowContent := container.NewBorder(nil, nil, container.NewHBox(check, badgesCol), rightCol, textCol)

	cardBg := canvas.NewRectangle(color.NRGBA{R: 245, G: 248, B: 252, A: 255})
	if task.Done {
		cardBg.FillColor = color.NRGBA{R: 235, G: 238, B: 242, A: 160}
	}
	cardBg.CornerRadius = 8

	return container.NewStack(cardBg, container.NewPadded(rowContent))
}
