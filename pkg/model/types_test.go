package model_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/altenwald/backlog/pkg/model"
)

func TestSizeWeight(t *testing.T) {
	if model.SizeXL.Weight() <= model.SizeL.Weight() ||
		model.SizeL.Weight() <= model.SizeM.Weight() ||
		model.SizeM.Weight() <= model.SizeS.Weight() ||
		model.SizeS.Weight() <= model.SizeXS.Weight() {
		t.Fatalf("weights not ordered strictly: XL=%d, L=%d, M=%d, S=%d, XS=%d",
			model.SizeXL.Weight(), model.SizeL.Weight(), model.SizeM.Weight(),
			model.SizeS.Weight(), model.SizeXS.Weight())
	}
}

func TestTierLabels(t *testing.T) {
	if model.Tier1.ShortLabel() != "T1" || model.Tier5.ShortLabel() != "T5" {
		t.Fatalf("unexpected short label: T1=%s, T5=%s", model.Tier1.ShortLabel(), model.Tier5.ShortLabel())
	}
	if model.Tier1.Label() != "Blocker (T1)" {
		t.Fatalf("unexpected label: %s", model.Tier1.Label())
	}
}

func TestTaskBlockedHelpers(t *testing.T) {
	tasks := map[string]model.Task{
		"1": {ID: "1", Done: false},
		"2": {ID: "2", Done: true},
		"3": {ID: "3", DependsOn: []string{"1", "2"}},
		"4": {ID: "4", DependsOn: []string{"2"}},
		"5": {ID: "5"},
	}

	// Task 3 depends on 1 (open) and 2 (done) -> blocked!
	if !tasks["3"].IsBlocked(tasks) {
		t.Fatal("expected task 3 to be blocked")
	}
	blocking := tasks["3"].BlockingTaskIDs(tasks)
	if len(blocking) != 1 || blocking[0] != "1" {
		t.Fatalf("expected blocking task [1], got %+v", blocking)
	}

	// Task 4 depends only on 2 (done) -> not blocked!
	if tasks["4"].IsBlocked(tasks) {
		t.Fatal("expected task 4 not to be blocked")
	}

	// Task 5 has no dependencies -> not blocked
	if tasks["5"].IsBlocked(tasks) {
		t.Fatal("expected task 5 not to be blocked")
	}
}

func TestLegacyTaskUnmarshalJSON(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	jsonStr := `{"id":"99","title":"Legacy Task","created_at":"` + now.Format(time.RFC3339) + `","done_at":"` + now.Format(time.RFC3339) + `"}`

	var task model.Task
	err := json.Unmarshal([]byte(jsonStr), &task)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if task.InsertedAt.Unix() != now.Unix() {
		t.Fatalf("expected InsertedAt to map from created_at: got %v, expected %v", task.InsertedAt, now)
	}
	if task.TerminatedAt == nil || task.TerminatedAt.Unix() != now.Unix() {
		t.Fatalf("expected TerminatedAt to map from done_at: got %v, expected %v", task.TerminatedAt, now)
	}
}

func TestLegacyProjectUnmarshalJSON(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	jsonStr := `{"slug":"legacy-proj","name":"Legacy Project","description":"Old format","created_at":"` + now.Format(time.RFC3339) + `"}`

	var proj model.Project
	err := json.Unmarshal([]byte(jsonStr), &proj)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if proj.InsertedAt.Unix() != now.Unix() {
		t.Fatalf("expected InsertedAt to map from created_at: got %v, expected %v", proj.InsertedAt, now)
	}
}
