package model

import (
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// OverviewSectionTitle names the section that holds any text found before the
// first "## " heading when a legacy single-text specification is split.
const OverviewSectionTitle = "Overview"

// SpecSection is one independently stored and edited part of a project
// specification. Sections are kept in display order.
type SpecSection struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SpecSectionInfo is the lightweight index entry of a section, without body.
type SpecSectionInfo struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Size      int       `json:"size"` // body length in bytes
	UpdatedAt time.Time `json:"updated_at"`
}

func (s SpecSection) Info() SpecSectionInfo {
	return SpecSectionInfo{ID: s.ID, Title: s.Title, Size: len(s.Body), UpdatedAt: s.UpdatedAt}
}

// Markdown renders the section as a "## " heading followed by its body.
func (s SpecSection) Markdown() string {
	body := strings.TrimSpace(s.Body)
	if body == "" {
		return "## " + s.Title
	}
	return "## " + s.Title + "\n\n" + body
}

// SlugifySectionTitle builds a section ID from its title: lowercase ASCII
// letters and digits separated by single dashes.
func SlugifySectionTitle(title string) string {
	var b strings.Builder
	dash := false
	// Decompose accented letters (e.g. "ó" -> "o" + combining mark) and drop the marks.
	for _, r := range norm.NFD.String(strings.ToLower(title)) {
		switch {
		case unicode.Is(unicode.Mn, r):
		case r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			dash = false
		case b.Len() > 0 && !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.TrimSuffix(b.String(), "-")
	if slug == "" {
		slug = "section"
	}
	return slug
}

// UniqueSectionID returns a slug for title that is not present in taken.
func UniqueSectionID(title string, taken map[string]bool) string {
	base := SlugifySectionTitle(title)
	id := base
	for n := 2; taken[id]; n++ {
		id = base + "-" + strconv.Itoa(n)
	}
	return id
}

// SplitSpec splits a markdown specification into sections at every "## "
// heading outside fenced code blocks. Text before the first heading becomes
// an "Overview" section.
func SplitSpec(text string, now time.Time) []SpecSection {
	var sections []SpecSection
	taken := map[string]bool{}
	title := ""
	var body []string
	started := false

	flush := func() {
		content := strings.TrimSpace(strings.Join(body, "\n"))
		if !started && content == "" {
			return
		}
		t := title
		if !started {
			t = OverviewSectionTitle
		}
		id := UniqueSectionID(t, taken)
		taken[id] = true
		sections = append(sections, SpecSection{ID: id, Title: t, Body: content, UpdatedAt: now})
	}

	inFence := false
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
		}
		if !inFence && strings.HasPrefix(line, "## ") {
			flush()
			title = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			body = nil
			started = true
			continue
		}
		body = append(body, line)
	}
	flush()
	return sections
}

// JoinSpec renders sections back into a single markdown document. A leading
// "Overview" section is written without its heading, mirroring SplitSpec, so
// that splitting and joining round-trips.
func JoinSpec(sections []SpecSection) string {
	parts := make([]string, 0, len(sections))
	for i, s := range sections {
		if i == 0 && s.Title == OverviewSectionTitle {
			if body := strings.TrimSpace(s.Body); body != "" {
				parts = append(parts, body)
			}
			continue
		}
		parts = append(parts, s.Markdown())
	}
	return strings.Join(parts, "\n\n")
}
