package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/altenwald/backlog/pkg/model"
)

type SummaryBar struct {
	container    *fyne.Container
	statusLabel  *widget.Label
	chipsRow     *fyne.Container
	currentSize  *model.Size
	lastSummary  *model.Summary
	onSizeChange func(size *model.Size)
}

func NewSummaryBar(onSizeChange func(size *model.Size)) *SummaryBar {
	statusLabel := widget.NewLabelWithStyle("0 open / 0 tasks", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	chipsRow := container.NewHBox()

	row := container.NewHBox(statusLabel, layout.NewSpacer(), chipsRow)

	return &SummaryBar{
		container:    row,
		statusLabel:  statusLabel,
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

	sb.statusLabel.SetText(fmt.Sprintf("%d/%d open", sum.OpenTasks, sum.TotalTasks))

	sb.chipsRow.Objects = nil
	for _, sz := range []model.Size{model.SizeXL, model.SizeL, model.SizeM, model.SizeS, model.SizeXS} {
		total := sum.SizeCounts[sz]
		open := sum.OpenSizeCounts[sz]
		if total > 0 {
			sizeVal := sz
			isSelected := sb.currentSize != nil && *sb.currentSize == sizeVal

			btnText := fmt.Sprintf("%s (%d)", sizeVal, open)
			btn := widget.NewButton(btnText, func() {
				if sb.currentSize != nil && *sb.currentSize == sizeVal {
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
