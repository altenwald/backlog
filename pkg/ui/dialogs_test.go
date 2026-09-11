package ui

import (
	"os"
	"testing"

	"fyne.io/fyne/v2/test"
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
