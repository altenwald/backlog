package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/altenwald/backlog/pkg/model"
)

// Spec pages (sections) are replaced copy-on-write: every mutation builds a new slice,
// so projects returned by GetProject keep a consistent view.

// specProject resolves a project slug (empty means the active project).
// The caller must hold s.mu.
func (s *Store) specProject(slug string) (*model.Project, string, error) {
	if slug == "" {
		slug = s.config.ActiveProject
	}
	slug = strings.ToLower(slug)
	p, ok := s.projects[slug]
	if !ok {
		return nil, slug, fmt.Errorf("project '%s' not found", slug)
	}
	return p, slug, nil
}

func sectionIndex(sections []model.SpecSection, id string) int {
	for i, sec := range sections {
		if sec.ID == id {
			return i
		}
	}
	return -1
}

// commitSpec stores the new sections, persists the project and notifies
// subscribers. The caller must hold s.mu for writing.
func (s *Store) commitSpec(p *model.Project, slug, sectionID string, sections []model.SpecSection) error {
	p.Spec = sections
	p.UpdatedAt = time.Now()
	if err := s.saveProject(p); err != nil {
		return err
	}
	go s.notify(Event{Type: EventProjectUpdated, ProjectSlug: slug, SectionID: sectionID})
	return nil
}

// GetProjectSpecification returns the whole specification as one markdown document.
func (s *Store) GetProjectSpecification(slug string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, _, err := s.specProject(slug)
	if err != nil {
		return "", err
	}
	return model.JoinSpec(p.Spec), nil
}

// UpdateProjectSpecification replaces the whole specification, splitting the
// markdown into sections at each "## " heading.
func (s *Store) UpdateProjectSpecification(slug, spec string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, slug, err := s.specProject(slug)
	if err != nil {
		return err
	}
	return s.commitSpec(p, slug, "", model.SplitSpec(spec, time.Now()))
}

// ListSpecSections returns the section index (no bodies) in display order.
func (s *Store) ListSpecSections(slug string) ([]model.SpecSectionInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, _, err := s.specProject(slug)
	if err != nil {
		return nil, err
	}
	infos := make([]model.SpecSectionInfo, 0, len(p.Spec))
	for _, sec := range p.Spec {
		infos = append(infos, sec.Info())
	}
	return infos, nil
}

// GetSpecSections returns the requested sections in the given order, or all
// of them when ids is empty.
func (s *Store) GetSpecSections(slug string, ids []string) ([]model.SpecSection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, slug, err := s.specProject(slug)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return append([]model.SpecSection(nil), p.Spec...), nil
	}
	out := make([]model.SpecSection, 0, len(ids))
	for _, id := range ids {
		i := sectionIndex(p.Spec, id)
		if i < 0 {
			return nil, fmt.Errorf("spec page '%s' not found in project '%s'", id, slug)
		}
		out = append(out, p.Spec[i])
	}
	return out, nil
}

// AddSpecSection inserts a new page at position (0-based, after the main
// page); a negative or out-of-range position appends it at the end. An empty
// id is derived from the title; a given id (e.g. from a link to a missing
// page) must not exist yet.
func (s *Store) AddSpecSection(slug, id, title, body string, position int) (*model.SpecSection, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("page title is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, slug, err := s.specProject(slug)
	if err != nil {
		return nil, err
	}
	spec := model.NormalizeSpec(p.Spec, time.Now())
	if len(spec) == 0 {
		spec = []model.SpecSection{{ID: model.SpecMainID, Title: model.SpecMainTitle, UpdatedAt: time.Now()}}
	}
	taken := make(map[string]bool, len(spec))
	for _, sec := range spec {
		taken[sec.ID] = true
	}
	if id = strings.TrimSpace(id); id == "" {
		id = model.UniqueSectionID(title, taken)
	} else if id != model.SlugifySectionTitle(id) {
		return nil, fmt.Errorf("invalid page id '%s': use lowercase letters, digits and dashes", id)
	} else if taken[id] {
		return nil, fmt.Errorf("spec page '%s' already exists in project '%s'", id, slug)
	}
	body = strings.TrimSpace(body)
	sec := model.SpecSection{ID: id, Title: title, Body: body, Links: model.ParseSpecLinks(body), UpdatedAt: time.Now()}
	if position < 0 || position > len(spec) {
		position = len(spec)
	}
	position = max(position, 1) // the main page stays first
	sections := make([]model.SpecSection, 0, len(spec)+1)
	sections = append(sections, spec[:position]...)
	sections = append(sections, sec)
	sections = append(sections, spec[position:]...)
	if err := s.commitSpec(p, slug, sec.ID, sections); err != nil {
		return nil, err
	}
	return &sec, nil
}

// UpdateSpecSection changes the title and/or body of a section. A nil value
// leaves the field unchanged. The section ID stays stable across renames.
func (s *Store) UpdateSpecSection(slug, id string, title, body *string) (*model.SpecSection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, slug, err := s.specProject(slug)
	if err != nil {
		return nil, err
	}
	spec := p.Spec
	if len(spec) == 0 && id == model.SpecMainID {
		// The main page of an empty specification is created on first write.
		spec = []model.SpecSection{{ID: model.SpecMainID, Title: model.SpecMainTitle}}
	}
	i := sectionIndex(spec, id)
	if i < 0 {
		return nil, fmt.Errorf("spec page '%s' not found in project '%s'", id, slug)
	}
	sec := spec[i]
	if title != nil {
		t := strings.TrimSpace(*title)
		if t == "" {
			return nil, fmt.Errorf("section title cannot be empty")
		}
		sec.Title = t
	}
	if body != nil {
		sec.Body = strings.TrimSpace(*body)
		sec.Links = model.ParseSpecLinks(sec.Body)
	}
	sec.UpdatedAt = time.Now()
	sections := append([]model.SpecSection(nil), spec...)
	sections[i] = sec
	if err := s.commitSpec(p, slug, sec.ID, sections); err != nil {
		return nil, err
	}
	return &sec, nil
}

// DeleteSpecSection removes a page. The main page cannot be deleted; links
// to the removed page are kept and show up as missing pages.
func (s *Store) DeleteSpecSection(slug, id string) error {
	if id == model.SpecMainID {
		return fmt.Errorf("the main spec page cannot be deleted")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, slug, err := s.specProject(slug)
	if err != nil {
		return err
	}
	i := sectionIndex(p.Spec, id)
	if i < 0 {
		return fmt.Errorf("spec page '%s' not found in project '%s'", id, slug)
	}
	sections := make([]model.SpecSection, 0, len(p.Spec)-1)
	sections = append(sections, p.Spec[:i]...)
	sections = append(sections, p.Spec[i+1:]...)
	return s.commitSpec(p, slug, id, sections)
}

// MoveSpecSection moves a page to position (0-based, clamped to range). The
// main page always stays first.
func (s *Store) MoveSpecSection(slug, id string, position int) error {
	if id == model.SpecMainID {
		return fmt.Errorf("the main spec page cannot be moved")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, slug, err := s.specProject(slug)
	if err != nil {
		return err
	}
	i := sectionIndex(p.Spec, id)
	if i < 0 {
		return fmt.Errorf("spec page '%s' not found in project '%s'", id, slug)
	}
	position = max(1, min(position, len(p.Spec)-1))
	if position == i {
		return nil
	}
	sec := p.Spec[i]
	rest := make([]model.SpecSection, 0, len(p.Spec))
	rest = append(rest, p.Spec[:i]...)
	rest = append(rest, p.Spec[i+1:]...)
	sections := make([]model.SpecSection, 0, len(p.Spec))
	sections = append(sections, rest[:position]...)
	sections = append(sections, sec)
	sections = append(sections, rest[position:]...)
	return s.commitSpec(p, slug, id, sections)
}
