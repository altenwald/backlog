package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/altenwald/backlog/pkg/model"
)

type FilterBar struct {
	container      *fyne.Container
	searchEntry    *widget.Entry
	buttons        []*widget.Button
	currentTier    *model.Tier
	currentSearch  string
	onFilterChange func(filter model.TaskFilter)
}

func NewFilterBar(onFilterChange func(filter model.TaskFilter)) *FilterBar {
	fb := &FilterBar{
		onFilterChange: onFilterChange,
	}

	fb.searchEntry = widget.NewEntry()
	fb.searchEntry.SetPlaceHolder("Search tasks...")
	fb.searchEntry.OnChanged = func(s string) {
		fb.currentSearch = s
		fb.emit()
	}

	btnAll := widget.NewButton("All", func() {
		fb.currentTier = nil
		fb.updateActiveButton(0)
		fb.emit()
	})
	btnAll.Importance = widget.HighImportance

	btnT1 := widget.NewButton("T1 · Blocker", func() {
		t := model.Tier1
		fb.currentTier = &t
		fb.updateActiveButton(1)
		fb.emit()
	})

	btnT2 := widget.NewButton("T2 · Important", func() {
		t := model.Tier2
		fb.currentTier = &t
		fb.updateActiveButton(2)
		fb.emit()
	})

	btnT3 := widget.NewButton("T3 · Visual debt", func() {
		t := model.Tier3
		fb.currentTier = &t
		fb.updateActiveButton(3)
		fb.emit()
	})

	btnT4 := widget.NewButton("T4 · Internal", func() {
		t := model.Tier4
		fb.currentTier = &t
		fb.updateActiveButton(4)
		fb.emit()
	})

	btnT5 := widget.NewButton("T5 · Future", func() {
		t := model.Tier5
		fb.currentTier = &t
		fb.updateActiveButton(5)
		fb.emit()
	})

	fb.buttons = []*widget.Button{btnAll, btnT1, btnT2, btnT3, btnT4, btnT5}

	tierRow := container.NewHBox(
		btnAll, btnT1, btnT2, btnT3, btnT4, btnT5,
	)

	topBar := container.NewBorder(nil, nil, nil, fb.searchEntry, tierRow)
	fb.container = container.NewVBox(topBar)

	return fb
}

func (fb *FilterBar) updateActiveButton(activeIndex int) {
	for i, btn := range fb.buttons {
		if i == activeIndex {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.MediumImportance
		}
		btn.Refresh()
	}
}

func (fb *FilterBar) UpdateCounts(sum *model.Summary) {
	if sum == nil {
		return
	}
	fb.buttons[0].SetText(fmt.Sprintf("All (%d)", sum.TotalTasks))
	fb.buttons[1].SetText(fmt.Sprintf("T1 (%d)", sum.TierCounts[model.Tier1]))
	fb.buttons[2].SetText(fmt.Sprintf("T2 (%d)", sum.TierCounts[model.Tier2]))
	fb.buttons[3].SetText(fmt.Sprintf("T3 (%d)", sum.TierCounts[model.Tier3]))
	fb.buttons[4].SetText(fmt.Sprintf("T4 (%d)", sum.TierCounts[model.Tier4]))
	fb.buttons[5].SetText(fmt.Sprintf("T5 (%d)", sum.TierCounts[model.Tier5]))
}

func (fb *FilterBar) emit() {
	if fb.onFilterChange != nil {
		fb.onFilterChange(model.TaskFilter{
			Tier:   fb.currentTier,
			Search: fb.currentSearch,
		})
	}
}

func (fb *FilterBar) CanvasObject() fyne.CanvasObject {
	return fb.container
}
