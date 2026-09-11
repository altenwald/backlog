package ui

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/server"
	"github.com/altenwald/backlog/pkg/store"
	"github.com/altenwald/backlog/pkg/version"
)

func ShowAddTaskDialog(parent fyne.Window, projectSlug string, onSave func(task model.Task)) {
	titleEntry := widget.NewEntry()
	titleEntry.SetPlaceHolder("Task title...")

	descEntry := widget.NewMultiLineEntry()
	descEntry.Wrapping = fyne.TextWrapWord
	descEntry.TextStyle = fyne.TextStyle{Monospace: true}
	descEntry.SetMinRowsVisible(6)
	descEntry.SetPlaceHolder("Details and context (Markdown supported)...")

	parentEntry := widget.NewEntry()
	parentEntry.SetPlaceHolder("Parent Task ID (optional, e.g. 1)")

	dependsEntry := widget.NewEntry()
	dependsEntry.SetPlaceHolder("Task IDs this task depends on (e.g. 1, 2)")

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

	resEntry := widget.NewMultiLineEntry()
	resEntry.Wrapping = fyne.TextWrapWord
	resEntry.TextStyle = fyne.TextStyle{Monospace: true}
	resEntry.SetMinRowsVisible(3)
	resEntry.SetPlaceHolder("Implementation details / resolution summary (optional)...")

	items := []*widget.FormItem{
		widget.NewFormItem("Title", titleEntry),
		widget.NewFormItem("Description", descEntry),
		widget.NewFormItem("Parent Task ID", parentEntry),
		widget.NewFormItem("Depends on", dependsEntry),
		widget.NewFormItem("Assignee", assigneeEntry),
		widget.NewFormItem("Effort Size", sizeSelect),
		widget.NewFormItem("Priority / Tier", tierSelect),
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

		var dependsOn []string
		for _, part := range strings.Split(dependsEntry.Text, ",") {
			if s := strings.TrimSpace(part); s != "" {
				dependsOn = append(dependsOn, s)
			}
		}

		task := model.Task{
			Title:       strings.TrimSpace(titleEntry.Text),
			Description: strings.TrimSpace(descEntry.Text),
			ParentID:    strings.TrimSpace(parentEntry.Text),
			DependsOn:   dependsOn,
			Assignee:    strings.TrimSpace(assigneeEntry.Text),
			Size:        model.Size(sizeSelect.Selected),
			Tier:        model.Tier(tierNum),
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
	descEntry.TextStyle = fyne.TextStyle{Monospace: true}
	descEntry.SetMinRowsVisible(6)
	descEntry.SetText(task.Description)

	parentEntry := widget.NewEntry()
	parentEntry.SetPlaceHolder("Parent Task ID (optional, e.g. 1)")
	parentEntry.SetText(task.ParentID)

	dependsEntry := widget.NewEntry()
	dependsEntry.SetPlaceHolder("e.g. 1, 2 (IDs this task depends on)")
	dependsEntry.SetText(strings.Join(task.DependsOn, ", "))

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

	resEntry := widget.NewMultiLineEntry()
	resEntry.Wrapping = fyne.TextWrapWord
	resEntry.TextStyle = fyne.TextStyle{Monospace: true}
	resEntry.SetMinRowsVisible(4)
	resEntry.SetPlaceHolder("Summary of implementation details, architectural decisions, and resolution (Markdown supported)...")
	resEntry.SetText(task.Resolution)

	items := []*widget.FormItem{
		widget.NewFormItem("Title", titleEntry),
		widget.NewFormItem("Description", descEntry),
		widget.NewFormItem("Parent Task ID", parentEntry),
		widget.NewFormItem("Depends on", dependsEntry),
		widget.NewFormItem("Assignee", assigneeEntry),
		widget.NewFormItem("Effort Size", sizeSelect),
		widget.NewFormItem("Priority / Tier", tierSelect),
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

		var dependsOn []string
		for _, part := range strings.Split(dependsEntry.Text, ",") {
			if s := strings.TrimSpace(part); s != "" {
				dependsOn = append(dependsOn, s)
			}
		}

		updated := task
		updated.Title = strings.TrimSpace(titleEntry.Text)
		updated.Description = strings.TrimSpace(descEntry.Text)
		updated.ParentID = strings.TrimSpace(parentEntry.Text)
		updated.DependsOn = dependsOn
		updated.Assignee = strings.TrimSpace(assigneeEntry.Text)
		updated.Size = model.Size(sizeSelect.Selected)
		updated.Tier = model.Tier(tierNum)
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
	slugEntry.SetPlaceHolder("unique slug (e.g. my-project, web-app)")

	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Display Name (e.g. My Project, Web App)")

	descEntry := widget.NewMultiLineEntry()
	descEntry.Wrapping = fyne.TextWrapWord
	descEntry.TextStyle = fyne.TextStyle{Monospace: true}
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

func ShowDeleteProjectDialog(parent fyne.Window, projectName, projectSlug string, onConfirm func()) {
	displayName := projectName
	if displayName == "" {
		displayName = projectSlug
	}
	msg := fmt.Sprintf("Are you sure you want to permanently delete project '%s' (%s) and ALL its tasks?\n\nThis action cannot be undone.", displayName, projectSlug)
	dialog.ShowConfirm("Delete Project", msg, func(confirmed bool) {
		if confirmed && onConfirm != nil {
			onConfirm()
		}
	}, parent)
}

var (
	activeAboutMu     sync.Mutex
	activeAboutWindow fyne.Window
)

// ShowAboutDialog opens the About Backlog window as an independent window.
// If the window is already open, it is brought to the front and focused.
func ShowAboutDialog(parent fyne.Window) {
	activeAboutMu.Lock()
	if activeAboutWindow != nil {
		win := activeAboutWindow
		activeAboutMu.Unlock()
		win.Show()
		win.RequestFocus()
		return
	}

	appInstance := fyne.CurrentApp()
	if appInstance == nil {
		activeAboutMu.Unlock()
		return
	}

	win := appInstance.NewWindow("About Backlog")
	activeAboutWindow = win
	activeAboutMu.Unlock()

	win.SetIcon(GetAppIconResource())
	win.SetOnClosed(func() {
		activeAboutMu.Lock()
		activeAboutWindow = nil
		activeAboutMu.Unlock()
	})

	// Close on Cmd+W (or Ctrl+W)
	win.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyW,
		Modifier: fyne.KeyModifierShortcutDefault,
	}, func(shortcut fyne.Shortcut) {
		win.Close()
	})

	iconImg := canvas.NewImageFromResource(GetAppIconResource())
	iconImg.SetMinSize(fyne.NewSize(72, 72))
	iconImg.FillMode = canvas.ImageFillContain

	titleLabel := widget.NewLabelWithStyle("Backlog", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	versionStr := fmt.Sprintf("Version %s", version.Version)
	if version.GitCommit != "" && version.GitCommit != "none" {
		versionStr += fmt.Sprintf(" (%s)", version.GitCommit)
	}
	versionLabel := widget.NewLabelWithStyle(versionStr, fyne.TextAlignCenter, fyne.TextStyle{Italic: true})

	descLabel := widget.NewLabelWithStyle(
		"Developer-centric task & priority manager\nwith System Tray and MCP integration",
		fyne.TextAlignCenter,
		fyne.TextStyle{},
	)

	copyrightLabel := widget.NewLabelWithStyle(
		"Copyright © 2026 Altenwald\nAuthor: Manuel Rubio",
		fyne.TextAlignCenter,
		fyne.TextStyle{},
	)

	altenwaldURL, _ := url.Parse("https://altenwald.com")
	altenwaldLink := widget.NewHyperlink("altenwald.com", altenwaldURL)

	githubURL, _ := url.Parse("https://github.com/altenwald/backlog")
	githubLink := widget.NewHyperlink("github.com/altenwald/backlog", githubURL)

	linksBox := container.NewHBox(
		altenwaldLink,
		widget.NewLabel("·"),
		githubLink,
	)
	linksCenter := container.NewCenter(linksBox)

	licenseEntry := widget.NewMultiLineEntry()
	licenseEntry.Wrapping = fyne.TextWrapWord
	licenseEntry.TextStyle = fyne.TextStyle{Monospace: true}
	licenseEntry.SetMinRowsVisible(6)
	licenseEntry.SetText(`MIT License

Copyright (c) 2026 Altenwald / Manuel Rubio

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.`)
	licenseEntry.Disable()

	licenseAccordion := widget.NewAccordion(
		widget.NewAccordionItem("📜 View License (MIT)", licenseEntry),
	)

	content := container.NewVBox(
		container.NewCenter(iconImg),
		titleLabel,
		versionLabel,
		widget.NewSeparator(),
		descLabel,
		copyrightLabel,
		linksCenter,
		widget.NewSeparator(),
		licenseAccordion,
	)

	closeBtn := widget.NewButton("Close", func() {
		win.Close()
	})
	closeBtn.Importance = widget.MediumImportance
	closeCenter := container.NewCenter(closeBtn)

	scrollContent := container.NewVScroll(container.NewPadded(content))

	layout := container.NewBorder(
		nil,
		container.NewVBox(widget.NewSeparator(), closeCenter),
		nil,
		nil,
		scrollContent,
	)

	win.SetContent(layout)
	win.Resize(fyne.NewSize(480, 520))
	win.CenterOnScreen()
	win.Show()
	win.RequestFocus()
}

var (
	activeSettingsMu     sync.Mutex
	activeSettingsWindow fyne.Window
)

// ShowSettingsDialog opens the application settings window.
// The core Backlog instructions are shown read-only; the user can freely edit
// the custom instructions section that is appended to the core prompt.
func ShowSettingsDialog(parent fyne.Window, st *store.Store) {
	activeSettingsMu.Lock()
	if activeSettingsWindow != nil {
		win := activeSettingsWindow
		activeSettingsMu.Unlock()
		win.Show()
		win.RequestFocus()
		return
	}

	appInstance := fyne.CurrentApp()
	if appInstance == nil {
		activeSettingsMu.Unlock()
		return
	}

	win := appInstance.NewWindow("Settings")
	activeSettingsWindow = win
	activeSettingsMu.Unlock()

	win.SetIcon(GetAppIconResource())
	win.SetOnClosed(func() {
		activeSettingsMu.Lock()
		activeSettingsWindow = nil
		activeSettingsMu.Unlock()
	})

	// Close on Cmd+W (or Ctrl+W)
	win.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyW,
		Modifier: fyne.KeyModifierShortcutDefault,
	}, func(shortcut fyne.Shortcut) {
		win.Close()
	})

	// ── Core instructions (read-only, formatted in high-contrast Markdown) ──
	coreRichText := widget.NewRichTextFromMarkdown(server.BacklogCoreInstructions)
	coreRichText.Wrapping = fyne.TextWrapWord

	coreScroll := container.NewVScroll(container.NewPadded(coreRichText))

	var copyBtn *widget.Button
	copyBtn = widget.NewButtonWithIcon("Copy to clipboard", theme.ContentCopyIcon(), func() {
		win.Clipboard().SetContent(server.BacklogCoreInstructions)
		copyBtn.SetText("✔ Copied!")
		go func() {
			time.Sleep(2 * time.Second)
			copyBtn.SetText("Copy to clipboard")
		}()
	})
	copyBtn.Importance = widget.LowImportance

	coreLockLabel := widget.NewLabelWithStyle(
		"🔒 Core instructions — read-only (defines the Backlog tool protocol)",
		fyne.TextAlignLeading,
		fyne.TextStyle{Italic: true},
	)

	coreTop := container.NewBorder(nil, nil, nil, copyBtn, coreLockLabel)
	coreSection := container.NewBorder(
		container.NewVBox(
			coreTop,
			widget.NewSeparator(),
		),
		nil, nil, nil,
		coreScroll,
	)

	// ── Custom / user instructions (editable) ──────────────────────────────
	customEntry := widget.NewMultiLineEntry()
	customEntry.Wrapping = fyne.TextWrapWord
	customEntry.TextStyle = fyne.TextStyle{Monospace: true}
	customEntry.SetPlaceHolder("Add your own methodology, conventions, or workflow rules here…")

	// Load persisted instructions (fall back to default when empty)
	saved := st.GetMCPUserInstructions()
	if saved == "" {
		customEntry.SetText(strings.TrimSpace(server.BacklogDefaultUserInstructions))
	} else {
		customEntry.SetText(saved)
	}

	resetBtn := widget.NewButtonWithIcon("Reset to default", theme.ContentRedoIcon(), func() {
		customEntry.SetText(strings.TrimSpace(server.BacklogDefaultUserInstructions))
	})
	resetBtn.Importance = widget.LowImportance

	customLabel := widget.NewLabelWithStyle(
		"✏️ Custom instructions — appended after core (editable)",
		fyne.TextAlignLeading,
		fyne.TextStyle{Italic: true},
	)

	customTop := container.NewBorder(nil, nil, nil, resetBtn, customLabel)
	customSection := container.NewBorder(
		container.NewVBox(
			customTop,
			widget.NewSeparator(),
		),
		nil, nil, nil,
		customEntry,
	)

	// ── Layout ─────────────────────────────────────────────────────────────
	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Custom Instructions", theme.DocumentCreateIcon(), customSection),
		container.NewTabItemWithIcon("Core Instructions", theme.InfoIcon(), coreSection),
	)

	saveBtn := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		text := strings.TrimSpace(customEntry.Text)
		// Storing empty string means "use default" at next MCP launch
		if text == strings.TrimSpace(server.BacklogDefaultUserInstructions) {
			text = ""
		}
		if err := st.SaveMCPUserInstructions(text); err != nil {
			dialog.ShowError(err, win)
			return
		}
		win.Close()
	})
	saveBtn.Importance = widget.HighImportance

	cancelBtn := widget.NewButton("Cancel", func() {
		win.Close()
	})

	buttons := container.NewHBox(cancelBtn, saveBtn)
	buttonsRight := container.NewBorder(nil, nil, nil, buttons)

	content := container.NewBorder(
		nil,
		container.NewVBox(widget.NewSeparator(), buttonsRight),
		nil, nil,
		tabs,
	)

	win.SetContent(content)
	win.Resize(fyne.NewSize(700, 620))
	win.CenterOnScreen()
	win.Show()
	win.RequestFocus()
}
