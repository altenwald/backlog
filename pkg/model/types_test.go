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
	// Default branch: unknown size returns 3 (same as M)
	if got := model.Size("UNKNOWN").Weight(); got != 3 {
		t.Fatalf("expected default Weight=3, got %d", got)
	}
}

func TestTierLabels(t *testing.T) {
	// ShortLabel for all defined tiers
	shortLabels := map[model.Tier]string{
		model.Tier1: "T1",
		model.Tier2: "T2",
		model.Tier3: "T3",
		model.Tier4: "T4",
		model.Tier5: "T5",
	}
	for tier, expected := range shortLabels {
		if got := tier.ShortLabel(); got != expected {
			t.Fatalf("ShortLabel(%d): expected %s, got %s", tier, expected, got)
		}
	}
	// Default branch for ShortLabel (unknown tier)
	if got := model.Tier(99).ShortLabel(); got != "T3" {
		t.Fatalf("ShortLabel default: expected T3, got %s", got)
	}

	// Label for all defined tiers
	labels := map[model.Tier]string{
		model.Tier1: "Blocker (T1)",
		model.Tier2: "Important (T2)",
		model.Tier3: "Visual debt (T3)",
		model.Tier4: "Internal (T4)",
		model.Tier5: "Future (T5)",
	}
	for tier, expected := range labels {
		if got := tier.Label(); got != expected {
			t.Fatalf("Label(%d): expected %s, got %s", tier, expected, got)
		}
	}
	// Default branch for Label (unknown tier)
	if got := model.Tier(99).Label(); got != "Visual debt (T3)" {
		t.Fatalf("Label default: expected 'Visual debt (T3)', got %s", got)
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
