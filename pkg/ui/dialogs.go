package ui

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/altenwald/backlog/pkg/model"
)

func ShowAddTaskDialog(parent fyne.Window, projectSlug string, onSave func(task model.Task)) {
	titleEntry := widget.NewEntry()
	titleEntry.SetPlaceHolder("Task title...")

	descEntry := widget.NewMultiLineEntry()
	descEntry.SetPlaceHolder("Details and context...")

	groupEntry := widget.NewEntry()
	groupEntry.SetPlaceHolder("e.g. Monetization, Domains, Bugs...")

	sizeSelect := widget.NewSelect([]string{"XS", "S", "M", "L", "XL"}, nil)
	sizeSelect.SetSelected("M")

	tierSelect := widget.NewSelect([]string{
		"1 · Blocker (T1)",
		"2 · Important (T2)",
		"3 · Visual debt (T3)",
		"4 · Internal (T4)",
		"5 · Future (T5)",
	}, nil)
	tierSelect.SetSelected("3 · Visual debt (T3)")

	tagEntry := widget.NewEntry()
	tagEntry.SetPlaceHolder("e.g. TODO, spec 08-24...")

	items := []*widget.FormItem{
		widget.NewFormItem("Title", titleEntry),
		widget.NewFormItem("Description", descEntry),
		widget.NewFormItem("Group / Category", groupEntry),
		widget.NewFormItem("Effort Size", sizeSelect),
		widget.NewFormItem("Priority / Tier", tierSelect),
		widget.NewFormItem("Tag / Reference", tagEntry),
	}

	d := dialog.NewForm("Add Task to "+projectSlug, "Save", "Cancel", items, func(confirmed bool) {
		if !confirmed || strings.TrimSpace(titleEntry.Text) == "" {
			return
		}

		tierNum := 3
		if len(tierSelect.Selected) > 0 {
			if num, err := strconv.Atoi(string(tierSelect.Selected[0])); err == nil {
				tierNum = num
			}
		}

		task := model.Task{
			Title:       strings.TrimSpace(titleEntry.Text),
			Description: strings.TrimSpace(descEntry.Text),
			Group:       strings.TrimSpace(groupEntry.Text),
			Size:        model.Size(sizeSelect.Selected),
			Tier:        model.Tier(tierNum),
			Tag:         strings.TrimSpace(tagEntry.Text),
		}

		if onSave != nil {
			onSave(task)
		}
	}, parent)

	d.Resize(fyne.NewSize(580, 480))
	d.Show()
}

func ShowEditTaskDialog(parent fyne.Window, task model.Task, onSave func(task model.Task)) {
	titleEntry := widget.NewEntry()
	titleEntry.SetText(task.Title)

	descEntry := widget.NewMultiLineEntry()
	descEntry.SetText(task.Description)

	groupEntry := widget.NewEntry()
	groupEntry.SetText(task.Group)

	sizeSelect := widget.NewSelect([]string{"XS", "S", "M", "L", "XL"}, nil)
	sizeSelect.SetSelected(string(task.Size))

	tiers := []string{
		"1 · Blocker (T1)",
		"2 · Important (T2)",
		"3 · Visual debt (T3)",
		"4 · Internal (T4)",
		"5 · Future (T5)",
	}
	tierIndex := int(task.Tier) - 1
	if tierIndex < 0 || tierIndex >= len(tiers) {
		tierIndex = 2
	}
	tierSelect := widget.NewSelect(tiers, nil)
	tierSelect.SetSelected(tiers[tierIndex])

	tagEntry := widget.NewEntry()
	tagEntry.SetText(task.Tag)

	items := []*widget.FormItem{
		widget.NewFormItem("Title", titleEntry),
		widget.NewFormItem("Description", descEntry),
		widget.NewFormItem("Group / Category", groupEntry),
		widget.NewFormItem("Effort Size", sizeSelect),
		widget.NewFormItem("Priority / Tier", tierSelect),
		widget.NewFormItem("Tag / Reference", tagEntry),
	}

	d := dialog.NewForm("Edit Task #"+task.ID, "Save", "Cancel", items, func(confirmed bool) {
		if !confirmed || strings.TrimSpace(titleEntry.Text) == "" {
			return
		}

		tierNum := int(task.Tier)
		if len(tierSelect.Selected) > 0 {
			if num, err := strconv.Atoi(string(tierSelect.Selected[0])); err == nil {
				tierNum = num
			}
		}

		updated := task
		updated.Title = strings.TrimSpace(titleEntry.Text)
		updated.Description = strings.TrimSpace(descEntry.Text)
		updated.Group = strings.TrimSpace(groupEntry.Text)
		updated.Size = model.Size(sizeSelect.Selected)
		updated.Tier = model.Tier(tierNum)
		updated.Tag = strings.TrimSpace(tagEntry.Text)

		if onSave != nil {
			onSave(updated)
		}
	}, parent)

	d.Resize(fyne.NewSize(580, 480))
	d.Show()
}

func ShowNewProjectDialog(parent fyne.Window, onSave func(slug, name, desc string)) {
	slugEntry := widget.NewEntry()
	slugEntry.SetPlaceHolder("unique slug (e.g. dymmer, conta)")

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Display Name (e.g. Dymmer, Conta)")

	descEntry := widget.NewMultiLineEntry()
	descEntry.SetPlaceHolder("Project description...")

	items := []*widget.FormItem{
		widget.NewFormItem("Project ID (slug)", slugEntry),
		widget.NewFormItem("Name", nameEntry),
		widget.NewFormItem("Description", descEntry),
	}

	d := dialog.NewForm("Create New Project", "Create", "Cancel", items, func(confirmed bool) {
		if !confirmed || strings.TrimSpace(slugEntry.Text) == "" {
			return
		}
		slug := strings.ToLower(strings.TrimSpace(slugEntry.Text))
		name := strings.TrimSpace(nameEntry.Text)
		if name == "" {
			name = strings.Title(slug)
		}
		desc := strings.TrimSpace(descEntry.Text)

		if onSave != nil {
			onSave(slug, name, desc)
		}
	}, parent)

	d.Resize(fyne.NewSize(540, 360))
	d.Show()
}
