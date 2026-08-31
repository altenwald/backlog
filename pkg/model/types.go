package model

import (
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

func (s Size) Points() int {
	switch strings.ToUpper(string(s)) {
	case "XS":
		return 1
	case "S":
		return 2
	case "M":
		return 3
	case "L":
		return 5
	case "XL":
		return 8
	default:
		return 1
	}
}

type Tier int

const (
	Tier1 Tier = 1 // Blocker: real bug or core function without any path
	Tier2 Tier = 2 // Important: solid business, doesn't block today
	Tier3 Tier = 3 // Visual debt: consistency, zero functional impact
	Tier4 Tier = 4 // Internal: internal quality, invisible
	Tier5 Tier = 5 // Future: out of scope for full MVP
)

func (t Tier) Label() string {
	switch t {
	case Tier1:
		return "T1 · Blocker"
	case Tier2:
		return "T2 · Important"
	case Tier3:
		return "T3 · Visual debt"
	case Tier4:
		return "T4 · Internal"
	case Tier5:
		return "T5 · Future"
	default:
		return "T3"
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
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Group       string     `json:"group"` // e.g. "Monetization", "Domains", "Infrastructure", "Bugs", etc.
	Size        Size       `json:"size"`
	Tier        Tier       `json:"tier"`
	Done        bool       `json:"done"`
	DoneAt      *time.Time `json:"done_at,omitempty"`
	Assignee    string     `json:"assignee,omitempty"`   // e.g. "claude", "antigravity", "manuel"
	Resolution  string     `json:"resolution,omitempty"` // Summary of implementation details and resolution
	Tag         string     `json:"tag,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Project struct {
	Slug        string    `json:"slug"` // "dymmer", "conta"
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Tasks       []Task    `json:"tasks"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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
	TotalPoints     int          `json:"total_points"`
	OpenPoints      int          `json:"open_points"`
	SizeCounts      map[Size]int `json:"size_counts"`       // Total tasks by size
	OpenSizeCounts  map[Size]int `json:"open_size_counts"`  // Open tasks by size
	TierCounts      map[Tier]int `json:"tier_counts"`       // Open tasks by tier
	TotalTierCounts map[Tier]int `json:"total_tier_counts"` // Total tasks by tier
	Groups          []string     `json:"groups"`
}
