package ui

import (
	"errors"
	"fmt"
	"net"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
)

type interruptedBackend struct {
	store.Backend
	offline bool
}

func (b *interruptedBackend) ListSpecSections(slug string) ([]model.SpecSectionInfo, error) {
	if b.offline {
		return nil, errors.New("disconnected")
	}
	return b.Backend.ListSpecSections(slug)
}
func TestPageDraftSurvivesDisconnectAndRemoteDelete(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("Draft")
	st, e := store.NewStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	st.CreateProject("p", "P", "")
	st.AddSpecSection("p", "draft", "Draft", "original", -1)
	b := &interruptedBackend{Backend: st}
	v := NewSpecView(b, w)
	v.Refresh()
	v.Open("draft")
	v.setEditMode(true)
	v.bodyEntry.SetText("my unsaved work")
	b.offline = true
	v.Refresh()
	if v.bodyEntry.Text != "my unsaved work" || !v.modified || v.selectedID != "draft" {
		t.Fatal("disconnect discarded draft")
	}
	b.offline = false
	st.DeleteSpecSection("p", "draft")
	v.Refresh()
	if v.bodyEntry.Text != "my unsaved work" || !v.modified || v.selectedID != "draft" {
		t.Fatal("remote deletion discarded draft")
	}
}

func TestReapplyTaskDraftKeepsConcurrentUneditedFields(t *testing.T) {
	base := model.Task{ID: "1", Title: "Old", Assignee: "A", Size: model.SizeM}
	draft := base
	draft.Title = "My title"
	current := base
	current.Assignee = "B"
	current.Size = model.SizeXL
	merged := reapplyTaskDraft(base, draft, current)
	if merged.Title != "My title" || merged.Assignee != "B" || merged.Size != model.SizeXL {
		t.Fatalf("reapplying draft overwrote other edits: %+v", merged)
	}
}

func TestConnectionMessageHidesTransportStack(t *testing.T) {
	err := fmt.Errorf("failed to WebSocket dial: %w", &net.OpError{Op: "dial", Net: "tcp", Err: syscall.EHOSTUNREACH})
	message := connectionMessage(err)
	if strings.Contains(message, "WebSocket") || strings.Contains(message, "https://") || len(message) > 120 {
		t.Fatalf("transport details leaked into dialog: %s", message)
	}
	if runtime.GOOS == "darwin" && !strings.Contains(message, "Local Network") {
		t.Fatalf("missing permission guidance: %s", message)
	}
}
