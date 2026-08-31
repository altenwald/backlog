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
	descEntry.Wrapping = fyne.TextWrapWord
	descEntry.SetMinRowsVisible(6)
	descEntry.SetPlaceHolder("Details and context (Markdown supported)...")

	groupEntry := widget.NewEntry()
	groupEntry.SetPlaceHolder("e.g. Monetization, Domains, Bugs...")

	assigneeEntry := widget.NewEntry()
	assigneeEntry.SetPlaceHolder("e.g. @claude, @manuel...")

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

	resEntry := widget.NewMultiLineEntry()
	resEntry.Wrapping = fyne.TextWrapWord
	resEntry.SetMinRowsVisible(3)
	resEntry.SetPlaceHolder("Implementation details / resolution summary (optional)...")

	items := []*widget.FormItem{
		widget.NewFormItem("Title", titleEntry),
		widget.NewFormItem("Description", descEntry),
		widget.NewFormItem("Group / Category", groupEntry),
		widget.NewFormItem("Assignee", assigneeEntry),
		widget.NewFormItem("Effort Size", sizeSelect),
		widget.NewFormItem("Priority / Tier", tierSelect),
		widget.NewFormItem("Tag / Reference", tagEntry),
		widget.NewFormItem("Resolution", resEntry),
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
			Assignee:    strings.TrimSpace(assigneeEntry.Text),
			Size:        model.Size(sizeSelect.Selected),
			Tier:        model.Tier(tierNum),
			Tag:         strings.TrimSpace(tagEntry.Text),
			Resolution:  strings.TrimSpace(resEntry.Text),
		}

		if onSave != nil {
			onSave(task)
		}
	}, parent)

	d.Resize(fyne.NewSize(680, 600))
	d.Show()
}

func ShowEditTaskDialog(parent fyne.Window, task model.Task, onSave func(task model.Task)) {
	titleEntry := widget.NewEntry()
	titleEntry.SetText(task.Title)

	descEntry := widget.NewMultiLineEntry()
	descEntry.Wrapping = fyne.TextWrapWord
	descEntry.SetMinRowsVisible(6)
	descEntry.SetText(task.Description)

	groupEntry := widget.NewEntry()
	groupEntry.SetText(task.Group)

	assigneeEntry := widget.NewEntry()
	assigneeEntry.SetPlaceHolder("e.g. @claude, @manuel...")
	assigneeEntry.SetText(task.Assignee)

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

	resEntry := widget.NewMultiLineEntry()
	resEntry.Wrapping = fyne.TextWrapWord
	resEntry.SetMinRowsVisible(4)
	resEntry.SetPlaceHolder("Summary of implementation details, architectural decisions, and resolution (Markdown supported)...")
	resEntry.SetText(task.Resolution)

	items := []*widget.FormItem{
		widget.NewFormItem("Title", titleEntry),
		widget.NewFormItem("Description", descEntry),
		widget.NewFormItem("Group / Category", groupEntry),
		widget.NewFormItem("Assignee", assigneeEntry),
		widget.NewFormItem("Effort Size", sizeSelect),
		widget.NewFormItem("Priority / Tier", tierSelect),
		widget.NewFormItem("Tag / Reference", tagEntry),
		widget.NewFormItem("Resolution / Details", resEntry),
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
		updated.Assignee = strings.TrimSpace(assigneeEntry.Text)
		updated.Size = model.Size(sizeSelect.Selected)
		updated.Tier = model.Tier(tierNum)
		updated.Tag = strings.TrimSpace(tagEntry.Text)
		updated.Resolution = strings.TrimSpace(resEntry.Text)

		if onSave != nil {
			onSave(updated)
		}
	}, parent)

	d.Resize(fyne.NewSize(680, 600))
	d.Show()
}

func ShowNewProjectDialog(parent fyne.Window, onSave func(slug, name, desc string)) {
	slugEntry := widget.NewEntry()
	slugEntry.SetPlaceHolder("unique slug (e.g. dymmer, conta)")

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Display Name (e.g. Dymmer, Conta)")

	descEntry := widget.NewMultiLineEntry()
	descEntry.Wrapping = fyne.TextWrapWord
	descEntry.SetMinRowsVisible(4)
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

	d.Resize(fyne.NewSize(580, 420))
	d.Show()
}
