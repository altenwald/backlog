package network

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/altenwald/backlog/pkg/model"
)

func (r *Remote) ListProjects() []*model.Project {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*model.Project, 0, len(r.projects))
	for _, p := range r.projects {
		v := *p
		out = append(out, &v)
	}
	return out
}
func (r *Remote) GetProject(slug string) (*model.Project, error) { return r.snapshot(slug) }
func (r *Remote) ListTasks(slug string, f model.TaskFilter) ([]model.Task, error) {
	p, e := r.snapshot(slug)
	if e != nil {
		return nil, e
	}
	m := map[string]model.Task{}
	for _, t := range p.Tasks {
		m[t.ID] = t
	}
	var out []model.Task
	for _, t := range p.Tasks {
		if f.Tier != nil && t.Tier != *f.Tier || f.Size != nil && t.Size != *f.Size || f.Done != nil && t.Done != *f.Done || f.Deprecated != nil && t.Deprecated != *f.Deprecated || f.ParentID != nil && t.ParentID != *f.ParentID || f.Blocked != nil && t.IsBlocked(m) != *f.Blocked {
			continue
		}
		if f.DependsOn != nil && *f.DependsOn != "" {
			found := false
			for _, d := range t.DependsOn {
				if d == *f.DependsOn {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		if f.Assignee != nil {
			a := strings.ToLower(strings.TrimPrefix(*f.Assignee, "@"))
			if a == "unassigned" && t.Assignee != "" {
				continue
			}
			if a != "" && a != "unassigned" && a != strings.ToLower(strings.TrimPrefix(t.Assignee, "@")) {
				continue
			}
		}
		q := strings.ToLower(strings.TrimSpace(f.Search))
		if q != "" && t.ID != q && t.ParentID != q && !strings.Contains(strings.ToLower(t.Title+"\n"+t.Description+"\n"+t.Assignee), q) {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}
func (r *Remote) GetTopPriorities(slug string, limit int) ([]model.Task, error) {
	done := false
	ts, e := r.ListTasks(slug, model.TaskFilter{Done: &done})
	if e != nil {
		return nil, e
	}
	m := map[string]model.Task{}
	for _, t := range ts {
		m[t.ID] = t
	}
	sort.SliceStable(ts, func(i, j int) bool {
		a, b := ts[i], ts[j]
		if a.IsBlocked(m) != b.IsBlocked(m) {
			return !a.IsBlocked(m)
		}
		if a.Tier != b.Tier {
			return a.Tier < b.Tier
		}
		return a.Size.Weight() > b.Size.Weight()
	})
	if limit <= 0 {
		limit = 5
	}
	if len(ts) > limit {
		ts = ts[:limit]
	}
	return ts, nil
}
func (r *Remote) GetSummary(slug string) (*model.Summary, error) {
	p, e := r.snapshot(slug)
	if e != nil {
		return nil, e
	}
	s := &model.Summary{ProjectSlug: slug, ProjectName: p.Name, SizeCounts: map[model.Size]int{}, OpenSizeCounts: map[model.Size]int{}, TierCounts: map[model.Tier]int{}, TotalTierCounts: map[model.Tier]int{}}
	for _, t := range p.Tasks {

		s.TotalTasks++
		s.SizeCounts[t.Size]++
		s.TotalTierCounts[t.Tier]++
		if t.Done {
			s.CompletedTasks++
		} else {
			s.OpenTasks++
			s.OpenSizeCounts[t.Size]++
			s.TierCounts[t.Tier]++
		}
	}
	return s, nil
}
func (r *Remote) GetProjectSpecification(slug string) (string, error) {
	p, e := r.snapshot(slug)
	if e != nil {
		return "", e
	}
	return model.JoinSpec(p.Spec), nil
}
func (r *Remote) ListSpecSections(slug string) ([]model.SpecSectionInfo, error) {
	p, e := r.snapshot(slug)
	if e != nil {
		return nil, e
	}
	var out []model.SpecSectionInfo
	for _, s := range p.Spec {
		out = append(out, s.Info())
	}
	return out, nil
}
func (r *Remote) GetSpecSections(slug string, ids []string) ([]model.SpecSection, error) {
	p, e := r.snapshot(slug)
	if e != nil {
		return nil, e
	}
	if len(ids) == 0 {
		return p.Spec, nil
	}
	var out []model.SpecSection
	for _, id := range ids {
		found := false
		for _, s := range p.Spec {
			if s.ID == id {
				out = append(out, s)
				found = true
			}
		}
		if !found {
			return nil, errors.New("page not found")
		}
	}
	return out, nil
}
func (r *Remote) ExportProjectJSON(slug string) ([]byte, error) {
	p, e := r.snapshot(slug)
	if e != nil {
		return nil, e
	}
	return json.MarshalIndent(p, "", "  ")
}
func (r *Remote) ExportProjectMarkdown(slug string) (string, error) {
	var out string
	ctx, stop := requestContext()
	defer stop()
	e := r.request(ctx, "ExportProjectMarkdown", []any{slug}, emptyPrecondition(), &out)
	return out, e
}
