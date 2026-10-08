package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/altenwald/backlog/pkg/model"
)

// legacyConfig is the config.json written by versions that stored data as JSON.
type legacyConfig struct {
	ActiveProject       string `json:"active_project"`
	MCPUserInstructions string `json:"mcp_user_instructions,omitempty"`
}

func hasLegacyData(dataDir string) bool {
	if _, err := os.Stat(filepath.Join(dataDir, "config.json")); err == nil {
		return true
	}
	matches, _ := filepath.Glob(filepath.Join(dataDir, "projects", "*.json"))
	return len(matches) > 0
}

// readLegacy parses config.json and projects/*.json. Any unreadable file is
// an error: importing a partial set would look like data loss.
func readLegacy(dataDir string) (legacyConfig, []*model.Project, error) {
	var cfg legacyConfig
	configPath := filepath.Join(dataDir, "config.json")
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, nil, fmt.Errorf("cannot parse %s: %w", configPath, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return cfg, nil, fmt.Errorf("cannot read %s: %w", configPath, err)
	}

	paths, err := filepath.Glob(filepath.Join(dataDir, "projects", "*.json"))
	if err != nil {
		return cfg, nil, err
	}
	sort.Strings(paths)
	var projects []*model.Project
	seen := make(map[string]string)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return cfg, nil, fmt.Errorf("cannot read %s: %w", path, err)
		}
		var p model.Project
		if err := json.Unmarshal(data, &p); err != nil {
			return cfg, nil, fmt.Errorf("cannot parse %s: %w", path, err)
		}
		p.Slug = strings.ToLower(strings.TrimSpace(p.Slug))
		if p.Slug == "" {
			return cfg, nil, fmt.Errorf("cannot load %s: missing project slug", path)
		}
		if other, dup := seen[p.Slug]; dup {
			return cfg, nil, fmt.Errorf("%s and %s both define project '%s'", other, path, p.Slug)
		}
		seen[p.Slug] = path
		projects = append(projects, &p)
	}
	return cfg, projects, nil
}

// importProject inserts a project and its tasks. Dangling parent or
// dependency references fail the import, preserving the original source for repair.
func importProject(q querier, p *model.Project) error {
	now := time.Now()
	if p.InsertedAt.IsZero() {
		p.InsertedAt = now
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = p.InsertedAt
	}

	ids := make(map[string]bool, len(p.Tasks))
	nextID := 1
	for _, t := range p.Tasks {
		if t.ID == "" {
			return fmt.Errorf("project '%s': task without ID (%q)", p.Slug, t.Title)
		}
		if ids[t.ID] {
			return fmt.Errorf("project '%s': duplicate task ID '%s'", p.Slug, t.ID)
		}
		ids[t.ID] = true
		if n, err := strconv.Atoi(t.ID); err == nil && n >= nextID {
			nextID = n + 1
		}
	}

	if err := insertProject(q, p, nextID); err != nil {
		return fmt.Errorf("project '%s': %w", p.Slug, err)
	}

	for _, t := range p.Tasks {
		if t.ParentID != "" && !ids[t.ParentID] {
			return fmt.Errorf("project %s task %s: missing parent %s; original data preserved", p.Slug, t.ID, t.ParentID)
		}
		var deps []string
		seen := make(map[string]bool)
		for _, d := range t.DependsOn {
			d = strings.TrimSpace(d)
			if d == "" || seen[d] {
				continue
			}
			seen[d] = true
			if !ids[d] || d == t.ID {
				return fmt.Errorf("project %s task %s: invalid dependency %s; original data preserved", p.Slug, t.ID, d)
			}
			deps = append(deps, d)
		}
		t.DependsOn = deps

		if t.Size == "" {
			t.Size = model.SizeM
		}
		t.Size = model.Size(strings.ToUpper(string(t.Size)))
		if t.Tier <= 0 || t.Tier > 5 {
			t.Tier = model.Tier3
		}
		if t.InsertedAt.IsZero() {
			t.InsertedAt = p.InsertedAt
		}
		if t.UpdatedAt.IsZero() {
			t.UpdatedAt = t.InsertedAt
		}
		if err := insertTask(q, p.Slug, t); err != nil {
			return fmt.Errorf("project '%s' task #%s: %w", p.Slug, t.ID, err)
		}
	}

	return saveSections(q, p.Slug, p.Spec)
}
