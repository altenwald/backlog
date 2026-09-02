package model

import (
	"encoding/json"
	"strings"
	"time"
)

type Size string

const (
	SizeXS Size = "XS"
	SizeS  Size = "S"
	SizeM  Size = "M"
	SizeL  Size = "L"
	SizeXL Size = "XL"
)

func (s Size) Weight() int {
	switch strings.ToUpper(string(s)) {
	case "XS":
		return 1
	case "S":
		return 2
	case "M":
		return 3
	case "L":
		return 4
	case "XL":
		return 5
	default:
		return 3
	}
}

type Tier int

const (
	Tier1 Tier = 1 // Blocker
	Tier2 Tier = 2 // Important
	Tier3 Tier = 3 // Visual debt
	Tier4 Tier = 4 // Internal
	Tier5 Tier = 5 // Future
)

func (t Tier) Label() string {
	switch t {
	case Tier1:
		return "Blocker (T1)"
	case Tier2:
		return "Important (T2)"
	case Tier3:
		return "Visual debt (T3)"
	case Tier4:
		return "Internal (T4)"
	case Tier5:
		return "Future (T5)"
	default:
		return "Visual debt (T3)"
	}
}

func (t Tier) ShortLabel() string {
	switch t {
	case Tier1:
		return "T1"
	case Tier2:
		return "T2"
	case Tier3:
		return "T3"
	case Tier4:
		return "T4"
	case Tier5:
		return "T5"
	default:
		return "T3"
	}
}

type Task struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	Description  string     `json:"description"`
	Group        string     `json:"group"` // e.g. "Monetization", "Domains", "Infrastructure", "Bugs", etc.
	Size         Size       `json:"size"`
	Tier         Tier       `json:"tier"`
	Done         bool       `json:"done"`
	Assignee     string     `json:"assignee,omitempty"`      // e.g. "claude", "antigravity", "manuel"
	Resolution   string     `json:"resolution,omitempty"`    // Summary of implementation details and resolution
	Tag          string     `json:"tag,omitempty"`
	InsertedAt   time.Time  `json:"inserted_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	TerminatedAt *time.Time `json:"terminated_at,omitempty"`
}

func (t *Task) UnmarshalJSON(data []byte) error {
	type Alias Task
	aux := &struct {
		*Alias
		LegacyCreatedAt *time.Time `json:"created_at"`
		LegacyDoneAt    *time.Time `json:"done_at"`
	}{
		Alias: (*Alias)(t),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if t.InsertedAt.IsZero() && aux.LegacyCreatedAt != nil {
		t.InsertedAt = *aux.LegacyCreatedAt
	}
	if t.TerminatedAt == nil && aux.LegacyDoneAt != nil {
		t.TerminatedAt = aux.LegacyDoneAt
	}
	return nil
}

type Project struct {
	Slug        string    `json:"slug"` // e.g. "my-project"
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tasks       []Task    `json:"tasks"`
	InsertedAt  time.Time `json:"inserted_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (p *Project) UnmarshalJSON(data []byte) error {
	type Alias Project
	aux := &struct {
		*Alias
		LegacyCreatedAt *time.Time `json:"created_at"`
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if p.InsertedAt.IsZero() && aux.LegacyCreatedAt != nil {
		p.InsertedAt = *aux.LegacyCreatedAt
	}
	return nil
}

type TaskFilter struct {
	Tier     *Tier   `json:"tier,omitempty"`
	Group    *string `json:"group,omitempty"`
	Size     *Size   `json:"size,omitempty"`
	Done     *bool   `json:"done,omitempty"`
	Assignee *string `json:"assignee,omitempty"`
	Search   string  `json:"search,omitempty"`
}

type Summary struct {
	ProjectSlug     string       `json:"project_slug"`
	ProjectName     string       `json:"project_name"`
	TotalTasks      int          `json:"total_tasks"`
	OpenTasks       int          `json:"open_tasks"`
	CompletedTasks  int          `json:"completed_tasks"`
	SizeCounts      map[Size]int `json:"size_counts"`       // Total tasks by size
	OpenSizeCounts  map[Size]int `json:"open_size_counts"`  // Open tasks by size
	TierCounts      map[Tier]int `json:"tier_counts"`       // Open tasks by tier
	TotalTierCounts map[Tier]int `json:"total_tier_counts"` // Total tasks by tier
	Groups          []string     `json:"groups"`
}
