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
	hideDoneCheck  *widget.Check
	buttons        []*widget.Button
	currentTier    *model.Tier
	currentSize    *model.Size
	currentDone    *bool
	currentSearch  string
	lastSummary    *model.Summary
	onFilterChange func(filter model.TaskFilter)
}

func NewFilterBar(onFilterChange func(filter model.TaskFilter)) *FilterBar {
	doneFalse := false
	fb := &FilterBar{
		currentDone:    &doneFalse,
		onFilterChange: onFilterChange,
	}

	fb.searchEntry = widget.NewEntry()
	fb.searchEntry.SetPlaceHolder("Search tasks…")
	fb.searchEntry.OnChanged = func(s string) {
		fb.currentSearch = s
		fb.emit()
	}

	fb.hideDoneCheck = widget.NewCheck("Hide completed", func(checked bool) {
		if checked {
			doneFalse := false
			fb.currentDone = &doneFalse
		} else {
			fb.currentDone = nil
		}
		fb.UpdateCounts(fb.lastSummary)
		fb.emit()
	})
	fb.hideDoneCheck.Checked = true

	btnAll := widget.NewButton("All", func() {
		fb.currentTier = nil
		fb.updateActiveButton(0)
		fb.emit()
	})
	btnAll.Importance = widget.HighImportance

	btnT1 := widget.NewButton("T1", func() {
		t := model.Tier1
		fb.currentTier = &t
		fb.updateActiveButton(1)
		fb.emit()
	})

	btnT2 := widget.NewButton("T2", func() {
		t := model.Tier2
		fb.currentTier = &t
		fb.updateActiveButton(2)
		fb.emit()
	})

	btnT3 := widget.NewButton("T3", func() {
		t := model.Tier3
		fb.currentTier = &t
		fb.updateActiveButton(3)
		fb.emit()
	})

	btnT4 := widget.NewButton("T4", func() {
		t := model.Tier4
		fb.currentTier = &t
		fb.updateActiveButton(4)
		fb.emit()
	})

	btnT5 := widget.NewButton("T5", func() {
		t := model.Tier5
		fb.currentTier = &t
		fb.updateActiveButton(5)
		fb.emit()
	})

	fb.buttons = []*widget.Button{btnAll, btnT1, btnT2, btnT3, btnT4, btnT5}
	fb.updateActiveButton(0)

	tierRow := container.NewGridWithColumns(6,
		btnAll, btnT1, btnT2, btnT3, btnT4, btnT5,
	)

	searchRow := container.NewBorder(nil, nil, nil, fb.hideDoneCheck, fb.searchEntry)
	fb.container = container.NewVBox(searchRow, tierRow)

	return fb
}

func (fb *FilterBar) SetSizeFilter(size *model.Size) {
	fb.currentSize = size
	fb.emit()
}

func (fb *FilterBar) updateActiveButton(activeIndex int) {
	for i, btn := range fb.buttons {
		if i == activeIndex {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.LowImportance
		}
		btn.Refresh()
	}
}

func (fb *FilterBar) UpdateCounts(sum *model.Summary) {
	fb.lastSummary = sum
	if sum == nil {
		fb.buttons[0].SetText("All (0)")
		fb.buttons[1].SetText("T1 (0)")
		fb.buttons[2].SetText("T2 (0)")
		fb.buttons[3].SetText("T3 (0)")
		fb.buttons[4].SetText("T4 (0)")
		fb.buttons[5].SetText("T5 (0)")
		return
	}
	total, counts := sum.OpenTasks, sum.TierCounts
	if fb.currentDone == nil {
		total, counts = sum.TotalTasks, sum.TotalTierCounts
	}
	fb.buttons[0].SetText(fmt.Sprintf("All (%d)", total))
	for i := 1; i < len(fb.buttons); i++ {
		fb.buttons[i].SetText(fmt.Sprintf("T%d (%d)", i, counts[model.Tier(i)]))
	}

}

func (fb *FilterBar) emit() {
	if fb.onFilterChange != nil {
		fb.onFilterChange(model.TaskFilter{
			Tier:   fb.currentTier,
			Size:   fb.currentSize,
			Done:   fb.currentDone,
			Search: fb.currentSearch,
		})
	}
}

func (fb *FilterBar) CurrentFilter() model.TaskFilter {
	return model.TaskFilter{
		Tier:   fb.currentTier,
		Size:   fb.currentSize,
		Done:   fb.currentDone,
		Search: fb.currentSearch,
	}
}

func (fb *FilterBar) CanvasObject() fyne.CanvasObject {
	return fb.container
}
