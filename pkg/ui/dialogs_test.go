package ui

import (
	"os"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
)

func TestSettingsWindowLifecycle(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")
	tmpDir, err := os.MkdirTemp("", "backlog-ui-settings-lifecycle-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	st, err := store.NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Open settings window
	ShowSettingsDialog(w, st)

	activeSettingsMu.Lock()
	win := activeSettingsWindow
	activeSettingsMu.Unlock()

	if win == nil {
		t.Fatal("expected activeSettingsWindow to be non-nil")
	}

	// 2. Calling again while open focuses existing window
	ShowSettingsDialog(w, st)

	activeSettingsMu.Lock()
	win2 := activeSettingsWindow
	activeSettingsMu.Unlock()

	if win2 != win {
		t.Fatal("expected activeSettingsWindow to be the same instance")
	}

	// 3. Find content objects and test interactions
	content := win.Content()
	if content == nil {
		t.Fatal("expected window content to be non-nil")
	}

	// 4. Close the window and verify activeSettingsWindow is reset
	win.Close()

	activeSettingsMu.Lock()
	winClosed := activeSettingsWindow
	activeSettingsMu.Unlock()

	if winClosed != nil {
		t.Fatal("expected activeSettingsWindow to be nil after close")
	}

	// 5. Open again and test saving custom instructions
	ShowSettingsDialog(w, st)
	activeSettingsMu.Lock()
	win3 := activeSettingsWindow
	activeSettingsMu.Unlock()
	if win3 == nil {
		t.Fatal("expected fresh activeSettingsWindow to be created")
	}

	// Test store saving via st
	customText := "Follow strict TDD and review rules."
	if err := st.SaveMCPUserInstructions(customText); err != nil {
		t.Fatalf("failed to save instructions: %v", err)
	}
	if got := st.GetMCPUserInstructions(); got != customText {
		t.Fatalf("expected %q, got %q", customText, got)
	}

	win3.Close()
}

func TestSettingsWindowSizeAndLayout(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")
	tmpDir, err := os.MkdirTemp("", "backlog-ui-settings-size-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	st, err := store.NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	ShowSettingsDialog(w, st)

	activeSettingsMu.Lock()
	win := activeSettingsWindow
	activeSettingsMu.Unlock()
	if win == nil {
		t.Fatal("expected window")
	}

	c := win.Content()
	if c.Size().Width < 600 || c.Size().Height < 500 {
		t.Fatalf("expected content size >= 600x500, got %+v", c.Size())
	}

	win.Close()
}

func TestAboutWindowLifecycle(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")

	// 1. Open About window
	ShowAboutDialog(w)

	activeAboutMu.Lock()
	win := activeAboutWindow
	activeAboutMu.Unlock()

	if win == nil {
		t.Fatal("expected activeAboutWindow to be non-nil")
	}
	if win.Title() != "About Backlog" {
		t.Fatalf("expected title 'About Backlog', got %q", win.Title())
	}

	// 2. Calling again while open focuses existing window
	ShowAboutDialog(w)

	activeAboutMu.Lock()
	win2 := activeAboutWindow
	activeAboutMu.Unlock()

	if win2 != win {
		t.Fatal("expected activeAboutWindow to be the same instance")
	}

	// 3. Verify content
	content := win.Content()
	if content == nil {
		t.Fatal("expected window content to be non-nil")
	}

	// 4. Close the window and verify activeAboutWindow is reset
	win.Close()

	activeAboutMu.Lock()
	winClosed := activeAboutWindow
	activeAboutMu.Unlock()

	if winClosed != nil {
		t.Fatal("expected activeAboutWindow to be nil after close")
	}

	// 5. Open again with nil parent (e.g. when main window is hidden or called independently)
	ShowAboutDialog(nil)
	activeAboutMu.Lock()
	win3 := activeAboutWindow
	activeAboutMu.Unlock()
	if win3 == nil {
		t.Fatal("expected fresh activeAboutWindow to be created with nil parent")
	}
	win3.Close()
}

func TestOtherDialogs(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")

	// Test ShowAddTaskDialog
	savedTask := false
	ShowAddTaskDialog(w, "my-project", func(task model.Task) {
		savedTask = true
	})

	// Test ShowEditTaskDialog
	editedTask := false
	task := model.Task{
		ID:    "1",
		Title: "Test Task",
		Size:  model.SizeM,
		Tier:  model.Tier2,
	}
	ShowEditTaskDialog(w, task, func(updated model.Task) {
		editedTask = true
	})

	// Test ShowDeleteProjectDialog
	deletedProject := false
	ShowDeleteProjectDialog(w, "My Project", "my-project", func() {
		deletedProject = true
	})

	_ = savedTask
	_ = editedTask
	_ = deletedProject
}

func TestAppSpecRefreshLive(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "backlog-ui-spec-refresh-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	st, err := store.NewStore(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	_, err = st.CreateProject("test-proj", "Test Project", "Description")
	if err != nil {
		t.Fatal(err)
	}
	_ = st.SetActiveProject("test-proj")
	_ = st.UpdateProjectSpecification("test-proj", "Initial Spec v1")

	a := test.NewApp()
	w := a.NewWindow("Test")
	bApp := &BacklogApp{
		fyneApp: a,
		window:  w,
		store:   st,
	}

	// Initialize specEntry, status, and save button
	bApp.specEntry = widget.NewMultiLineEntry()
	bApp.specEntry.OnChanged = func(s string) {
		if s != bApp.lastLoadedSpecText {
			bApp.specModified = true
			bApp.specStatus.SetText("● Unsaved")
		} else {
			bApp.specModified = false
			bApp.specStatus.SetText("")
		}
	}
	bApp.specStatus = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	bApp.specSaveBtn = widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		activeSlug := bApp.store.GetActiveProjectSlug()
		if activeSlug != "" {
			if err := bApp.store.UpdateProjectSpecification(activeSlug, bApp.specEntry.Text); err == nil {
				bApp.lastLoadedSpecText = bApp.specEntry.Text
				bApp.specModified = false
				bApp.specStatus.SetText("✔ Saved")
			} else {
				bApp.specStatus.SetText("⚠️ Error saving")
			}
		}
	})

	// Initial load
	bApp.refreshSpec()

	if bApp.specEntry.Text != "Initial Spec v1" {
		t.Fatalf("expected 'Initial Spec v1', got %q", bApp.specEntry.Text)
	}

	// 1. Simulate external update via MCP / CLI / API
	newExternalSpec := "# Updated Spec by MCP\n- Ticket 1"
	if err := st.UpdateProjectSpecification("test-proj", newExternalSpec); err != nil {
		t.Fatal(err)
	}

	// Trigger refreshSpec as listenEvents does
	bApp.refreshSpec()

	if bApp.specEntry.Text != newExternalSpec {
		t.Fatalf("expected specEntry to reflect external update %q, got %q", newExternalSpec, bApp.specEntry.Text)
	}
	if bApp.specStatus.Text != "✔ Updated" {
		t.Fatalf("expected status '✔ Updated', got %q", bApp.specStatus.Text)
	}

	// 2. Simulate user typing in GUI
	bApp.specEntry.SetText("# User Unsaved Draft")
	if !bApp.specModified {
		t.Fatal("expected specModified to be true after typing")
	}
	if bApp.specStatus.Text != "● Unsaved" {
		t.Fatalf("expected status '● Unsaved', got %q", bApp.specStatus.Text)
	}

	// 3. Simulate clicking Save in GUI
	bApp.specSaveBtn.OnTapped()
	if bApp.specModified {
		t.Fatal("expected specModified to be false after Save")
	}
	if bApp.specStatus.Text != "✔ Saved" {
		t.Fatalf("expected status '✔ Saved', got %q", bApp.specStatus.Text)
	}

	savedProj, _ := st.GetProject("test-proj")
	if savedProj.Specification != "# User Unsaved Draft" {
		t.Fatalf("expected store to have saved draft, got %q", savedProj.Specification)
	}
}

