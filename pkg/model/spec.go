package model

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// The project specification works like a wiki: a main page (always first,
// ID SpecMainID) plus pages that link to each other with markdown links of
// the form [text](spec:<page-id>), forming a graph.
const (
	SpecMainID       = "main"
	SpecMainTitle    = "Main"
	SpecLinkScheme   = "spec"
	legacyOverviewID = "overview"
)

var specLinkRe = regexp.MustCompile(`\]\(spec:([A-Za-z0-9_-]+)\)`)

// SpecSection is one independently stored and edited page of a project
// specification. Pages are kept in display order, main page first.
type SpecSection struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Links     []string  `json:"links,omitempty"` // IDs of pages linked from Body, derived on write
	UpdatedAt time.Time `json:"updated_at"`
}

// SpecSectionInfo is the lightweight index entry of a page, without body.
type SpecSectionInfo struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Size      int       `json:"size"` // body length in bytes
	Links     []string  `json:"links,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s SpecSection) Info() SpecSectionInfo {
	return SpecSectionInfo{ID: s.ID, Title: s.Title, Size: len(s.Body), Links: s.Links, UpdatedAt: s.UpdatedAt}
}

// SpecLink returns the markdown link to a page.
func SpecLink(title, id string) string {
	return "[" + title + "](" + SpecLinkScheme + ":" + id + ")"
}

// ParseSpecLinks returns the page IDs linked from a markdown body, in order
// of first appearance.
func ParseSpecLinks(body string) []string {
	var links []string
	seen := map[string]bool{}
	for _, m := range specLinkRe.FindAllStringSubmatch(body, -1) {
		if id := m[1]; !seen[id] {
			seen[id] = true
			links = append(links, id)
		}
	}
	return links
}

// SpecBacklinks returns the pages that link to id.
func SpecBacklinks(index []SpecSectionInfo, id string) []SpecSectionInfo {
	var out []SpecSectionInfo
	for _, info := range index {
		for _, l := range info.Links {
			if l == id && info.ID != id {
				out = append(out, info)
				break
			}
		}
	}
	return out
}

// SpecUnreachable returns the IDs of pages that cannot be reached by
// following links from the main page.
func SpecUnreachable(index []SpecSectionInfo) map[string]bool {
	links := make(map[string][]string, len(index))
	for _, info := range index {
		links[info.ID] = info.Links
	}
	seen := map[string]bool{SpecMainID: true}
	queue := []string{SpecMainID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, l := range links[id] {
			if !seen[l] {
				seen[l] = true
				queue = append(queue, l)
			}
		}
	}
	out := map[string]bool{}
	for _, info := range index {
		if !seen[info.ID] {
			out[info.ID] = true
		}
	}
	return out
}

// NormalizeSpec makes sure the main page exists and comes first, and derives
// the links of every page. When the main page has to be created (legacy
// specifications), it links to every other page so none becomes unreachable.
func NormalizeSpec(sections []SpecSection, now time.Time) []SpecSection {
	return normalizeSpec(sections, now, false)
}

func normalizeSpec(sections []SpecSection, now time.Time, linkAll bool) []SpecSection {
	out := make([]SpecSection, 0, len(sections)+1)
	mainIdx := -1
	for i, s := range sections {
		if s.ID == SpecMainID {
			mainIdx = i
		}
	}
	created := false
	var main SpecSection
	switch {
	case mainIdx >= 0:
		main = sections[mainIdx]
	case len(sections) > 0 && sections[0].ID == legacyOverviewID:
		main = sections[0]
		main.ID, main.Title = SpecMainID, SpecMainTitle
		mainIdx, created = 0, true
	case len(sections) > 0:
		main = SpecSection{ID: SpecMainID, Title: SpecMainTitle, UpdatedAt: now}
		created = true
	default:
		return nil
	}
	for i, s := range sections {
		if i != mainIdx {
			s.Links = ParseSpecLinks(s.Body)
			out = append(out, s)
		}
	}
	if (created || linkAll) && len(out) > 0 {
		linked := map[string]bool{}
		for _, id := range ParseSpecLinks(main.Body) {
			linked[id] = true
		}
		var lines []string
		for _, s := range out {
			if !linked[s.ID] {
				lines = append(lines, "- "+SpecLink(s.Title, s.ID))
			}
		}
		if len(lines) > 0 {
			main.Body = strings.TrimSpace(main.Body + "\n\n" + strings.Join(lines, "\n"))
		}
	}
	main.Links = ParseSpecLinks(main.Body)
	return append([]SpecSection{main}, out...)
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

// SplitSpec splits a markdown specification into pages at every "## "
// heading outside fenced code blocks. Text before the first heading becomes
// the main page, which gets links to the pages it does not reference yet.
func SplitSpec(text string, now time.Time) []SpecSection {
	var sections []SpecSection
	taken := map[string]bool{SpecMainID: true}
	title := ""
	var body []string
	started := false

	flush := func() {
		content := strings.TrimSpace(strings.Join(body, "\n"))
		if !started {
			sections = append(sections, SpecSection{ID: SpecMainID, Title: SpecMainTitle, Body: content, UpdatedAt: now})
			return
		}
		id := UniqueSectionID(title, taken)
		taken[id] = true
		sections = append(sections, SpecSection{ID: id, Title: title, Body: content, UpdatedAt: now})
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
	if len(sections) == 1 && sections[0].Body == "" {
		return nil
	}
	return normalizeSpec(sections, now, true)
}

// JoinSpec renders all pages into a single markdown document: the main page
// body first (without heading), then every other page under a "## " heading,
// mirroring SplitSpec.
func JoinSpec(sections []SpecSection) string {
	parts := make([]string, 0, len(sections))
	for _, s := range sections {
		if s.ID == SpecMainID {
			if body := strings.TrimSpace(s.Body); body != "" {
				parts = append(parts, body)
			}
			continue
		}
		parts = append(parts, s.Markdown())
	}
	return strings.Join(parts, "\n\n")
}
