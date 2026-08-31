package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/altenwald/backlog/pkg/model"
)

type SummaryBar struct {
	container *fyne.Container
	ptsLabel  *widget.Label
	statLabel *widget.Label
	chipsRow  *fyne.Container
}

func NewSummaryBar() *SummaryBar {
	ptsLabel := widget.NewLabelWithStyle("0 pts", fyne.TextAlignLeading, fyne.TextStyle{Bold: true, Monospace: true})
	statLabel := widget.NewLabelWithStyle("0 open · 0 completed", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
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
		container: c,
		ptsLabel:  ptsLabel,
		statLabel: statLabel,
		chipsRow:  chipsRow,
	}
}

func (sb *SummaryBar) Update(sum *model.Summary) {
	if sum == nil {
		return
	}
	sb.ptsLabel.SetText(fmt.Sprintf("%d points (%d open)", sum.TotalPoints, sum.OpenPoints))
	sb.statLabel.SetText(fmt.Sprintf("%d open · %d completed", sum.OpenTasks, sum.CompletedTasks))

	sb.chipsRow.Objects = nil
	for _, size := range []model.Size{model.SizeXL, model.SizeL, model.SizeM, model.SizeS, model.SizeXS} {
		count := sum.SizeCounts[size]
		if count > 0 {
			badge := MakeBadge(fmt.Sprintf("%d %s", count, size), SizeColor(size), color.White)
			sb.chipsRow.Add(badge)
		}
	}
	sb.chipsRow.Refresh()
}

func (sb *SummaryBar) CanvasObject() fyne.CanvasObject {
	return sb.container
}
