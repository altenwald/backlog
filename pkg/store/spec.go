package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/altenwald/backlog/pkg/model"
)

// Spec pages live in spec_sections, ordered by position with the main page
// at 0. The links of each page are derived from its body on write and kept
// in spec_links; a link may point to a page that does not exist yet.

func loadSections(q querier, slug string) ([]model.SpecSection, error) {
	rows, err := q.Query(`SELECT id, title, body, updated_at FROM spec_sections
		WHERE project_slug = ? ORDER BY position`, slug)
	if err != nil {
		return nil, err
	}
	var sections []model.SpecSection
	for rows.Next() {
		var sec model.SpecSection
		var updated string
		if err := rows.Scan(&sec.ID, &sec.Title, &sec.Body, &updated); err != nil {
			rows.Close()
			return nil, err
		}
		sec.UpdatedAt = parseTime(updated)
		sections = append(sections, sec)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	links, err := loadLinks(q, slug)
	if err != nil {
		return nil, err
	}
	for i := range sections {
		sections[i].Links = links[sections[i].ID]
	}
	return sections, nil
}

func loadLinks(q querier, slug string) (map[string][]string, error) {
	rows, err := q.Query(`SELECT from_id, to_id FROM spec_links
		WHERE project_slug = ? ORDER BY from_id, position`, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	links := make(map[string][]string)
	for rows.Next() {
		var from, to string
		if err := rows.Scan(&from, &to); err != nil {
			return nil, err
		}
		links[from] = append(links[from], to)
	}
	return links, rows.Err()
}

func saveLinks(q querier, slug, id, body string) error {
	if _, err := q.Exec(`DELETE FROM spec_links WHERE project_slug = ? AND from_id = ?`, slug, id); err != nil {
		return err
	}
	for i, to := range model.ParseSpecLinks(body) {
		if _, err := q.Exec(`INSERT INTO spec_links(project_slug, from_id, to_id, position) VALUES(?, ?, ?, ?)`,
			slug, id, to, i); err != nil {
			return err
		}
	}
	return nil
}

func insertSection(q querier, slug string, sec model.SpecSection, position int) error {
	if _, err := q.Exec(`INSERT INTO spec_sections(project_slug, id, position, title, body, updated_at)
		VALUES(?, ?, ?, ?, ?, ?)`, slug, sec.ID, position, sec.Title, sec.Body, formatTime(sec.UpdatedAt)); err != nil {
		return err
	}
	return saveLinks(q, slug, sec.ID, sec.Body)
}

// saveSections replaces all pages of a project with the given list.
func saveSections(q querier, slug string, sections []model.SpecSection) error {
	if _, err := q.Exec(`DELETE FROM spec_sections WHERE project_slug = ?`, slug); err != nil {
		return err
	}
	for i, sec := range sections {
		if err := insertSection(q, slug, sec, i); err != nil {
			return err
		}
	}
	return nil
}

func reorderSections(q querier, slug string, ids []string) error {
	for i, id := range ids {
		if _, err := q.Exec(`UPDATE spec_sections SET position = ? WHERE project_slug = ? AND id = ?`,
			i, slug, id); err != nil {
			return err
		}
	}
	return nil
}

// normalizeStoredSpecs makes every project's spec follow the wiki model
// (main page first, links derived). Used when upgrading databases created
// before spec pages had links.
func normalizeStoredSpecs(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT slug FROM projects`)
	if err != nil {
		return err
	}
	var slugs []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			rows.Close()
			return err
		}
		slugs = append(slugs, slug)
	}
	rows.Close()

	for _, slug := range slugs {
		sections, err := loadSections(tx, slug)
		if err != nil {
			return err
		}
		if err := saveSections(tx, slug, model.NormalizeSpec(sections, time.Now())); err != nil {
			return err
		}
	}
	return nil
}

func sectionIndex(sections []model.SpecSection, id string) int {
	for i, sec := range sections {
		if sec.ID == id {
			return i
		}
	}
	return -1
}

func sectionIDs(sections []model.SpecSection) []string {
	ids := make([]string, len(sections))
	for i, sec := range sections {
		ids[i] = sec.ID
	}
	return ids
}

// withSpecTx resolves the project and runs fn in a write transaction with the
// current pages, then notifies subscribers.
func (s *Store) withSpecTx(slug, sectionID string, fn func(tx *sql.Tx, slug string, sections []model.SpecSection) (string, error)) error {
	err := s.withTx(func(tx *sql.Tx) error {
		slug = s.resolveSlug(tx, slug)
		if err := projectExists(tx, slug); err != nil {
			return err
		}
		sections, err := loadSections(tx, slug)
		if err != nil {
			return err
		}
		id, err := fn(tx, slug, sections)
		if err != nil {
			return err
		}
		if id != "" {
			sectionID = id
		}
		return touchProject(tx, slug, time.Now())
	})
	if err != nil {
		return err
	}
	go s.notify(Event{Type: EventProjectUpdated, ProjectSlug: slug, SectionID: sectionID})
	return nil
}

func (s *Store) readSections(slug string) ([]model.SpecSection, string, error) {
	slug = s.resolveSlug(s.db, slug)
	if err := projectExists(s.db, slug); err != nil {
		return nil, slug, err
	}
	sections, err := loadSections(s.db, slug)
	return sections, slug, err
}

// GetProjectSpecification returns the whole specification as one markdown document.
func (s *Store) GetProjectSpecification(slug string) (string, error) {
	sections, _, err := s.readSections(slug)
	if err != nil {
		return "", err
	}
	return model.JoinSpec(sections), nil
}

// UpdateProjectSpecification replaces the whole specification, splitting the
// markdown into pages at each "## " heading.
func (s *Store) UpdateProjectSpecification(slug, spec string) error {
	return s.withSpecTx(slug, "", func(tx *sql.Tx, slug string, _ []model.SpecSection) (string, error) {
		return "", saveSections(tx, slug, model.SplitSpec(spec, time.Now()))
	})
}

// ListSpecSections returns the page index (no bodies) in display order.
func (s *Store) ListSpecSections(slug string) ([]model.SpecSectionInfo, error) {
	slug = s.resolveSlug(s.db, slug)
	if err := projectExists(s.db, slug); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT id, title, length(CAST(body AS BLOB)), updated_at FROM spec_sections
		WHERE project_slug = ? ORDER BY position`, slug)
	if err != nil {
		return nil, err
	}
	infos := []model.SpecSectionInfo{}
	for rows.Next() {
		var info model.SpecSectionInfo
		var updated string
		if err := rows.Scan(&info.ID, &info.Title, &info.Size, &updated); err != nil {
			rows.Close()
			return nil, err
		}
		info.UpdatedAt = parseTime(updated)
		infos = append(infos, info)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	links, err := loadLinks(s.db, slug)
	if err != nil {
		return nil, err
	}
	for i := range infos {
		infos[i].Links = links[infos[i].ID]
	}
	return infos, nil
}

// GetSpecSections returns the requested pages in the given order, or all of
// them when ids is empty.
func (s *Store) GetSpecSections(slug string, ids []string) ([]model.SpecSection, error) {
	sections, slug, err := s.readSections(slug)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return sections, nil
	}
	out := make([]model.SpecSection, 0, len(ids))
	for _, id := range ids {
		i := sectionIndex(sections, id)
		if i < 0 {
			return nil, fmt.Errorf("spec page '%s' not found in project '%s'", id, slug)
		}
		out = append(out, sections[i])
	}
	return out, nil
}

func newMainPage(now time.Time) model.SpecSection {
	return model.SpecSection{ID: model.SpecMainID, Title: model.SpecMainTitle, UpdatedAt: now}
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
	var sec model.SpecSection
	err := s.withSpecTx(slug, "", func(tx *sql.Tx, slug string, sections []model.SpecSection) (string, error) {
		now := time.Now()
		if len(sections) == 0 {
			main := newMainPage(now)
			if err := insertSection(tx, slug, main, 0); err != nil {
				return "", err
			}
			sections = []model.SpecSection{main}
		}
		taken := make(map[string]bool, len(sections))
		for _, existing := range sections {
			taken[existing.ID] = true
		}
		if id = strings.TrimSpace(id); id == "" {
			id = model.UniqueSectionID(title, taken)
		} else if id != model.SlugifySectionTitle(id) {
			return "", fmt.Errorf("invalid page id '%s': use lowercase letters, digits and dashes", id)
		} else if taken[id] {
			return "", fmt.Errorf("spec page '%s' already exists in project '%s'", id, slug)
		}
		body = strings.TrimSpace(body)
		sec = model.SpecSection{ID: id, Title: title, Body: body, Links: model.ParseSpecLinks(body), UpdatedAt: now}

		if position < 0 || position > len(sections) {
			position = len(sections)
		}
		position = max(position, 1) // the main page stays first
		if err := insertSection(tx, slug, sec, len(sections)); err != nil {
			return "", err
		}
		ids := sectionIDs(sections)
		ids = append(ids[:position], append([]string{sec.ID}, ids[position:]...)...)
		return sec.ID, reorderSections(tx, slug, ids)
	})
	if err != nil {
		return nil, err
	}
	return &sec, nil
}

// UpdateSpecSection changes the title and/or body of a page. A nil value
// leaves the field unchanged. The page ID stays stable across renames.
func (s *Store) UpdateSpecSection(slug, id string, title, body *string) (*model.SpecSection, error) {
	var sec model.SpecSection
	err := s.withSpecTx(slug, id, func(tx *sql.Tx, slug string, sections []model.SpecSection) (string, error) {
		created := false
		i := sectionIndex(sections, id)
		if i < 0 && len(sections) == 0 && id == model.SpecMainID {
			// The main page of an empty specification is created on first write.
			sections = []model.SpecSection{newMainPage(time.Now())}
			i, created = 0, true
		}
		if i < 0 {
			return "", fmt.Errorf("spec page '%s' not found in project '%s'", id, slug)
		}
		sec = sections[i]
		if title != nil {
			t := strings.TrimSpace(*title)
			if t == "" {
				return "", fmt.Errorf("section title cannot be empty")
			}
			sec.Title = t
		}
		if body != nil {
			sec.Body = strings.TrimSpace(*body)
			sec.Links = model.ParseSpecLinks(sec.Body)
		}
		sec.UpdatedAt = time.Now()
		if created {
			return "", insertSection(tx, slug, sec, 0)
		}
		if _, err := tx.Exec(`UPDATE spec_sections SET title = ?, body = ?, updated_at = ?
			WHERE project_slug = ? AND id = ?`, sec.Title, sec.Body, formatTime(sec.UpdatedAt), slug, id); err != nil {
			return "", err
		}
		return "", saveLinks(tx, slug, id, sec.Body)
	})
	if err != nil {
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
	return s.withSpecTx(slug, id, func(tx *sql.Tx, slug string, sections []model.SpecSection) (string, error) {
		i := sectionIndex(sections, id)
		if i < 0 {
			return "", fmt.Errorf("spec page '%s' not found in project '%s'", id, slug)
		}
		// Outgoing links go by cascade; incoming links stay.
		if _, err := tx.Exec(`DELETE FROM spec_sections WHERE project_slug = ? AND id = ?`, slug, id); err != nil {
			return "", err
		}
		ids := sectionIDs(sections)
		return "", reorderSections(tx, slug, append(ids[:i], ids[i+1:]...))
	})
}

// MoveSpecSection moves a page to position (0-based, clamped to range). The
// main page always stays first.
func (s *Store) MoveSpecSection(slug, id string, position int) error {
	if id == model.SpecMainID {
		return fmt.Errorf("the main spec page cannot be moved")
	}
	return s.withSpecTx(slug, id, func(tx *sql.Tx, slug string, sections []model.SpecSection) (string, error) {
		i := sectionIndex(sections, id)
		if i < 0 {
			return "", fmt.Errorf("spec page '%s' not found in project '%s'", id, slug)
		}
		position = max(1, min(position, len(sections)-1))
		if position == i {
			return "", nil
		}
		ids := sectionIDs(sections)
		rest := append(append([]string(nil), ids[:i]...), ids[i+1:]...)
		moved := append(append(append([]string(nil), rest[:position]...), id), rest[position:]...)
		return "", reorderSections(tx, slug, moved)
	})
}
