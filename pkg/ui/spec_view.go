package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
)

const (
	specEmptyMarkdown     = "*No specification yet. Click 'Edit' to write the main page, or update it via MCP.*"
	specNoProjectMarkdown = "*No project selected.*"
	specEmptyPageMarkdown = "*This page is empty. Click 'Edit' to write it.*"
	specMissingSuffix     = " (new page)"
)

// SpecView shows the project specification as a wiki: a main page that links
// to other pages, which can link to each other. Only the open page is loaded
// and rendered, so large specifications stay cheap.
type SpecView struct {
	Container fyne.CanvasObject

	store  *store.Store
	window fyne.Window

	pageList   *widget.List
	index      []model.SpecSectionInfo
	unlinked   map[string]bool
	history    []string // pages visited before the open one, for Back
	navigating bool     // true while the view selects a list row itself

	titleEntry   *widget.Entry
	titleLabel   *widget.Label
	richText     *widget.RichText
	richScroll   *container.Scroll
	bodyEntry    *widget.Entry
	contentStack *fyne.Container
	editFields   *fyne.Container
	status       *widget.Label

	backBtn, previewBtn, editBtn, saveBtn, linkBtn *widget.Button
	upBtn, downBtn, deleteBtn                      *widget.Button

	slug       string
	selectedID string
	loaded     model.SpecSection // last stored version of the open page
	editMode   bool
	modified   bool
	syncing    bool // true while the view itself sets entry text
}

func NewSpecView(st *store.Store, win fyne.Window) *SpecView {
	v := &SpecView{store: st, window: win}

	v.pageList = widget.NewList(
		func() int { return len(v.index) },
		func() fyne.CanvasObject {
			l := widget.NewLabel("Page")
			l.Truncation = fyne.TextTruncateEllipsis
			return l
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			if id >= len(v.index) {
				return
			}
			info := v.index[id]
			l := o.(*widget.Label)
			l.TextStyle = fyne.TextStyle{Bold: info.ID == model.SpecMainID, Italic: v.unlinked[info.ID]}
			text := info.Title
			if v.unlinked[info.ID] {
				text += " (unlinked)"
			}
			l.SetText(text)
		},
	)
	v.pageList.OnSelected = func(id widget.ListItemID) {
		if !v.navigating && id < len(v.index) && v.index[id].ID != v.selectedID {
			v.Open(v.index[id].ID)
		}
	}

	v.richText = widget.NewRichTextFromMarkdown("")
	v.richText.Wrapping = fyne.TextWrapWord
	v.richScroll = container.NewVScroll(container.NewPadded(v.richText))

	v.titleLabel = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	v.titleLabel.Truncation = fyne.TextTruncateEllipsis
	v.titleEntry = widget.NewEntry()
	v.titleEntry.SetPlaceHolder("Page title")
	v.bodyEntry = widget.NewMultiLineEntry()
	v.bodyEntry.Wrapping = fyne.TextWrapWord
	v.bodyEntry.SetPlaceHolder("Write this page in markdown...\n\nLink other pages with [text](spec:page-id); a link to a page that does not exist yet lets you create it.\nReference ticket IDs explicitly (e.g. #1, #2). Tickets not referenced in the specification are considered out of scope or deprecated.")
	v.titleEntry.OnChanged = func(string) { v.updateModified() }
	v.bodyEntry.OnChanged = func(string) { v.updateModified() }

	v.linkBtn = widget.NewButtonWithIcon("Insert link", theme.MailAttachmentIcon(), v.showInsertLink)
	v.linkBtn.Importance = widget.LowImportance
	v.editFields = container.NewBorder(
		container.NewBorder(nil, nil, nil, v.linkBtn, v.titleEntry),
		nil, nil, nil, v.bodyEntry,
	)
	v.contentStack = container.NewStack(v.richScroll, v.editFields)

	v.status = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	v.backBtn = widget.NewButtonWithIcon("", theme.NavigateBackIcon(), v.Back)
	v.backBtn.Importance = widget.LowImportance
	v.previewBtn = widget.NewButtonWithIcon("Preview", theme.VisibilityIcon(), func() { v.setEditMode(false) })
	v.editBtn = widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), func() { v.setEditMode(true) })
	v.saveBtn = widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() { v.Save() })
	v.saveBtn.Importance = widget.HighImportance

	addBtn := widget.NewButtonWithIcon("New page", theme.ContentAddIcon(), func() { v.showNewPage("", "") })
	addBtn.Importance = widget.LowImportance
	v.upBtn = widget.NewButtonWithIcon("", theme.MoveUpIcon(), func() { v.moveSelected(-1) })
	v.downBtn = widget.NewButtonWithIcon("", theme.MoveDownIcon(), func() { v.moveSelected(1) })
	v.deleteBtn = widget.NewButtonWithIcon("", theme.DeleteIcon(), v.confirmDelete)
	for _, b := range []*widget.Button{v.upBtn, v.downBtn, v.deleteBtn} {
		b.Importance = widget.LowImportance
	}

	sidebar := container.NewBorder(
		container.NewVBox(widget.NewLabelWithStyle("Pages", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), widget.NewSeparator()),
		container.NewVBox(widget.NewSeparator(), container.NewHBox(addBtn, layout.NewSpacer(), v.upBtn, v.downBtn, v.deleteBtn)),
		nil, nil,
		v.pageList,
	)
	header := container.NewBorder(nil, nil, v.backBtn,
		container.NewHBox(v.status, v.previewBtn, v.editBtn, v.saveBtn),
		v.titleLabel,
	)
	editor := container.NewBorder(container.NewVBox(header, widget.NewSeparator()), nil, nil, nil, v.contentStack)

	split := container.NewHSplit(sidebar, editor)
	split.SetOffset(0.28)
	v.Container = split

	v.setEditMode(false)
	v.updateButtons()
	return v
}

// Refresh reloads the page index for the active project and the open page
// when it changed in the store (e.g. via MCP, CLI or API), keeping unsaved
// local edits untouched.
func (v *SpecView) Refresh() {
	slug := v.store.GetActiveProjectSlug()
	if slug != v.slug {
		if v.modified {
			v.Save()
		}
		v.slug = slug
		v.selectedID = ""
		v.history = nil
		v.loaded = model.SpecSection{}
		v.modified = false
		v.status.SetText("")
	}

	v.reloadIndex()

	if len(v.index) == 0 {
		// No pages yet: the main page is created on first save.
		v.selectedID = ""
		v.loaded = model.SpecSection{}
		if slug == "" {
			v.titleLabel.SetText("")
			v.richText.ParseMarkdown(specNoProjectMarkdown)
		} else {
			v.selectedID = model.SpecMainID
			v.loaded = model.SpecSection{ID: model.SpecMainID, Title: model.SpecMainTitle}
			v.titleLabel.SetText(model.SpecMainTitle)
			v.setEntries(model.SpecMainTitle, "")
			v.richText.ParseMarkdown(specEmptyMarkdown)
		}
		v.updateButtons()
		return
	}

	target := v.selectedID
	if v.indexOf(target) < 0 {
		target = model.SpecMainID
		v.modified = false
	}
	if target != v.selectedID || v.loaded.ID != target {
		v.selectedID = ""
		v.Open(target)
		return
	}

	// Same page: reload only if the stored version is newer.
	if info := v.index[v.indexOf(target)]; !info.UpdatedAt.Equal(v.loaded.UpdatedAt) {
		if v.modified {
			v.status.SetText("Changed elsewhere")
		} else {
			v.loadPage(target)
			v.status.SetText("Updated")
		}
	} else if !v.editMode {
		v.renderPreview() // links or backlinks may have changed
	}
	v.selectListRow(target)
	v.updateButtons()
}

func (v *SpecView) reloadIndex() {
	v.index = nil
	if v.slug != "" {
		v.index, _ = v.store.ListSpecSections(v.slug)
	}
	v.unlinked = model.SpecUnreachable(v.index)
	v.pageList.Refresh()
}

func (v *SpecView) indexOf(id string) int {
	for i, info := range v.index {
		if info.ID == id {
			return i
		}
	}
	return -1
}

func (v *SpecView) selectListRow(id string) {
	v.navigating = true
	if i := v.indexOf(id); i >= 0 {
		v.pageList.Select(i)
	} else {
		v.pageList.UnselectAll()
	}
	v.navigating = false
}

// Open saves pending edits and navigates to a page, remembering the current
// one for Back.
func (v *SpecView) Open(id string) {
	if id == v.selectedID {
		return
	}
	if v.modified {
		v.Save()
	}
	if v.selectedID != "" {
		v.history = append(v.history, v.selectedID)
	}
	v.show(id)
}

// Back returns to the previously open page.
func (v *SpecView) Back() {
	for len(v.history) > 0 {
		id := v.history[len(v.history)-1]
		v.history = v.history[:len(v.history)-1]
		if v.indexOf(id) >= 0 {
			if v.modified {
				v.Save()
			}
			v.show(id)
			return
		}
	}
	v.updateButtons()
}

func (v *SpecView) show(id string) {
	v.selectedID = id
	v.loadPage(id)
	v.status.SetText("")
	v.setEditMode(false)
	v.selectListRow(id)
	v.updateButtons()
}

func (v *SpecView) loadPage(id string) {
	pages, err := v.store.GetSpecSections(v.slug, []string{id})
	if err != nil || len(pages) == 0 {
		return
	}
	v.loaded = pages[0]
	v.modified = false
	v.titleLabel.SetText(v.loaded.Title)
	v.setEntries(v.loaded.Title, v.loaded.Body)
	v.renderPreview()
}

func (v *SpecView) setEntries(title, body string) {
	v.syncing = true
	v.titleEntry.SetText(title)
	v.bodyEntry.SetText(body)
	v.syncing = false
}

func (v *SpecView) updateModified() {
	if v.syncing {
		return
	}
	v.modified = v.selectedID != "" &&
		(v.titleEntry.Text != v.loaded.Title || v.bodyEntry.Text != v.loaded.Body)
	if v.modified {
		v.status.SetText("Unsaved changes")
	} else {
		v.status.SetText("")
	}
}

// renderPreview renders the open page with its "Linked from" footer and makes
// spec: links navigate inside the wiki.
func (v *SpecView) renderPreview() {
	if v.selectedID == "" {
		return
	}
	body := strings.TrimSpace(v.bodyEntry.Text)
	if body == "" {
		if len(v.index) == 0 {
			body = specEmptyMarkdown
		} else {
			body = specEmptyPageMarkdown
		}
	}
	if back := model.SpecBacklinks(v.index, v.selectedID); len(back) > 0 {
		links := make([]string, 0, len(back))
		for _, info := range back {
			links = append(links, model.SpecLink(info.Title, info.ID))
		}
		body += "\n\n---\n\n*Linked from:* " + strings.Join(links, ", ")
	}
	v.richText.ParseMarkdown(body)
	v.hookLinks(v.richText.Segments)
	v.richText.Refresh()
	v.richScroll.ScrollToTop()
}

// hookLinks makes spec: links open pages, or offer to create missing ones.
func (v *SpecView) hookLinks(segs []widget.RichTextSegment) {
	for _, seg := range segs {
		switch s := seg.(type) {
		case *widget.HyperlinkSegment:
			if s.URL == nil || s.URL.Scheme != model.SpecLinkScheme {
				continue
			}
			id, text := s.URL.Opaque, s.Text
			if v.indexOf(id) < 0 {
				s.Text += specMissingSuffix
				s.OnTapped = func() { v.showNewPage(id, text) }
			} else {
				s.OnTapped = func() { v.Open(id) }
			}
		case *widget.ParagraphSegment:
			v.hookLinks(s.Texts)
		case *widget.ListSegment:
			v.hookLinks(s.Items)
		}
	}
}

func (v *SpecView) setEditMode(editing bool) {
	v.editMode = editing
	if editing {
		v.richScroll.Hide()
		v.editFields.Show()
		v.previewBtn.Importance = widget.LowImportance
		v.editBtn.Importance = widget.HighImportance
	} else {
		v.renderPreview()
		v.editFields.Hide()
		v.richScroll.Show()
		v.previewBtn.Importance = widget.HighImportance
		v.editBtn.Importance = widget.LowImportance
	}
	v.previewBtn.Refresh()
	v.editBtn.Refresh()
	v.contentStack.Refresh()
}

func (v *SpecView) updateButtons() {
	i := v.indexOf(v.selectedID)
	isPage := i > 0 // an existing page other than main
	setEnabled := func(b *widget.Button, on bool) {
		if on {
			b.Enable()
		} else {
			b.Disable()
		}
	}
	setEnabled(v.backBtn, len(v.history) > 0)
	setEnabled(v.editBtn, v.selectedID != "")
	setEnabled(v.saveBtn, v.selectedID != "")
	setEnabled(v.deleteBtn, isPage)
	setEnabled(v.upBtn, i > 1)
	setEnabled(v.downBtn, isPage && i < len(v.index)-1)
}

// Save stores the open page if it has local changes.
func (v *SpecView) Save() {
	if v.selectedID == "" || v.slug == "" {
		return
	}
	if !v.modified {
		v.setEditMode(false)
		return
	}
	title, body := v.titleEntry.Text, v.bodyEntry.Text
	saved, err := v.store.UpdateSpecSection(v.slug, v.selectedID, &title, &body)
	if err != nil {
		v.status.SetText("Could not save")
		return
	}
	v.loaded = *saved
	v.modified = false
	v.titleLabel.SetText(saved.Title)
	v.status.SetText("Saved")
	v.reloadIndex()
	v.setEditMode(false)
	v.selectListRow(v.selectedID)
	v.updateButtons()
}

// showNewPage asks for a title and creates a page. With an id (a link to a
// missing page) the page gets that id so the link resolves.
func (v *SpecView) showNewPage(id, title string) {
	if v.slug == "" {
		return
	}
	titleEntry := widget.NewEntry()
	titleEntry.SetText(title)
	titleEntry.SetPlaceHolder("e.g. Architecture")
	heading := "New page"
	if id != "" {
		heading = "Create page '" + id + "'"
	}
	d := dialog.NewForm(heading, "Create", "Cancel",
		[]*widget.FormItem{widget.NewFormItem("Title", titleEntry)},
		func(ok bool) {
			if !ok || strings.TrimSpace(titleEntry.Text) == "" {
				return
			}
			v.createPage(id, titleEntry.Text)
		}, v.window)
	d.Resize(fyne.NewSize(420, 160))
	d.Show()
}

func (v *SpecView) createPage(id, title string) {
	if v.modified {
		v.Save()
	}
	page, err := v.store.AddSpecSection(v.slug, id, title, "", -1)
	if err != nil {
		dialog.ShowError(err, v.window)
		return
	}
	v.reloadIndex()
	v.Open(page.ID)
	v.setEditMode(true)
}

// showInsertLink inserts a link to an existing page at the cursor.
func (v *SpecView) showInsertLink() {
	var titles []string
	var ids []string
	for _, info := range v.index {
		if info.ID != v.selectedID {
			titles = append(titles, info.Title)
			ids = append(ids, info.ID)
		}
	}
	if len(ids) == 0 {
		return
	}
	sel := widget.NewSelect(titles, nil)
	sel.SetSelectedIndex(0)
	dialog.ShowForm("Insert link", "Insert", "Cancel",
		[]*widget.FormItem{widget.NewFormItem("Page", sel)},
		func(ok bool) {
			if ok && sel.SelectedIndex() >= 0 {
				i := sel.SelectedIndex()
				insertAtCursor(v.bodyEntry, model.SpecLink(titles[i], ids[i]))
			}
		}, v.window)
}

// insertAtCursor inserts text at the entry's cursor and moves the cursor
// after it.
func insertAtCursor(e *widget.Entry, text string) {
	lines := strings.Split(e.Text, "\n")
	row := min(e.CursorRow, len(lines)-1)
	line := []rune(lines[row])
	col := min(e.CursorColumn, len(line))
	lines[row] = string(line[:col]) + text + string(line[col:])
	e.SetText(strings.Join(lines, "\n"))
	e.CursorRow, e.CursorColumn = row, col+len([]rune(text))
	e.Refresh()
}

func (v *SpecView) moveSelected(delta int) {
	i := v.indexOf(v.selectedID)
	if i < 1 {
		return
	}
	if v.modified {
		v.Save()
	}
	if err := v.store.MoveSpecSection(v.slug, v.selectedID, i+delta); err != nil {
		v.status.SetText("Could not move")
		return
	}
	v.reloadIndex()
	v.selectListRow(v.selectedID)
	v.updateButtons()
}

func (v *SpecView) confirmDelete() {
	if v.indexOf(v.selectedID) < 1 {
		return
	}
	id, title := v.selectedID, v.loaded.Title
	msg := fmt.Sprintf("Delete the page %q? This cannot be undone.", title)
	if n := len(model.SpecBacklinks(v.index, id)); n > 0 {
		msg += fmt.Sprintf("\n%d page(s) link to it; those links will show as missing pages.", n)
	}
	dialog.ShowConfirm("Delete page", msg, func(ok bool) {
		if !ok {
			return
		}
		if err := v.store.DeleteSpecSection(v.slug, id); err != nil {
			dialog.ShowError(err, v.window)
			return
		}
		v.modified = false
		v.Refresh()
	}, v.window)
}
