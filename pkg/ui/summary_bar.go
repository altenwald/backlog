package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/altenwald/backlog/pkg/model"
)

type SummaryBar struct {
	container    *fyne.Container
	ptsLabel     *widget.Label
	statLabel    *widget.Label
	chipsRow     *fyne.Container
	currentSize  *model.Size
	lastSummary  *model.Summary
	onSizeChange func(size *model.Size)
}

func NewSummaryBar(onSizeChange func(size *model.Size)) *SummaryBar {
	ptsLabel := widget.NewLabelWithStyle("0 open / 0 tasks", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	statLabel := widget.NewLabelWithStyle("0 open pts (0 total)", fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})
	chipsRow := container.NewHBox()

	// Theme-aware card background
	bg := canvas.NewRectangle(theme.InputBackgroundColor())
	bg.CornerRadius = 8

	content := container.NewHBox(
		ptsLabel,
		widget.NewSeparator(),
		statLabel,
		widget.NewSeparator(),
		chipsRow,
	)

	c := container.NewStack(bg, container.NewPadded(content))

	return &SummaryBar{
		container:    c,
		ptsLabel:     ptsLabel,
		statLabel:    statLabel,
		chipsRow:     chipsRow,
		onSizeChange: onSizeChange,
	}
}

func (sb *SummaryBar) SetSelectedSize(size *model.Size) {
	sb.currentSize = size
	if sb.lastSummary != nil {
		sb.Update(sb.lastSummary)
	}
}

func (sb *SummaryBar) Update(sum *model.Summary) {
	if sum == nil {
		return
	}
	sb.lastSummary = sum

	sb.ptsLabel.SetText(fmt.Sprintf("%d open / %d total tasks", sum.OpenTasks, sum.TotalTasks))
	sb.statLabel.SetText(fmt.Sprintf("%d open pts (%d total pts)", sum.OpenPoints, sum.TotalPoints))

	sb.chipsRow.Objects = nil
	for _, sz := range []model.Size{model.SizeXL, model.SizeL, model.SizeM, model.SizeS, model.SizeXS} {
		total := sum.SizeCounts[sz]
		open := sum.OpenSizeCounts[sz]
		if total > 0 {
			sizeVal := sz
			isSelected := sb.currentSize != nil && *sb.currentSize == sizeVal

			btnText := fmt.Sprintf("%d/%d %s", open, total, sizeVal)
			btn := widget.NewButton(btnText, func() {
				if sb.currentSize != nil && *sb.currentSize == sizeVal {
					// Toggle off filter
					sb.currentSize = nil
				} else {
					sb.currentSize = &sizeVal
				}
				sb.Update(sb.lastSummary)
				if sb.onSizeChange != nil {
					sb.onSizeChange(sb.currentSize)
				}
			})

			if isSelected {
				btn.Importance = widget.HighImportance
			} else {
				btn.Importance = widget.LowImportance
			}

			sb.chipsRow.Add(btn)
		}
	}
	sb.chipsRow.Refresh()
}

func (sb *SummaryBar) CanvasObject() fyne.CanvasObject {
	return sb.container
}
