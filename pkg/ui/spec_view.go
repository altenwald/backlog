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
	specEmptyMarkdown     = "*No specification sections yet. Click 'Add section' or update via MCP to define project architecture & scope.*"
	specNoProjectMarkdown = "*No project selected.*"
)

// SpecView shows the project specification as a list of sections. Only the
// selected section is loaded and rendered, so large specifications stay cheap.
type SpecView struct {
	Container fyne.CanvasObject

	store  *store.Store
	window fyne.Window

	sectionList *widget.List
	index       []model.SpecSectionInfo

	titleEntry   *widget.Entry
	titleLabel   *widget.Label
	richText     *widget.RichText
	richScroll   *container.Scroll
	bodyEntry    *widget.Entry
	contentStack *fyne.Container
	editFields   *fyne.Container
	status       *widget.Label

	previewBtn, editBtn, saveBtn *widget.Button
	upBtn, downBtn, deleteBtn    *widget.Button

	slug       string
	selectedID string
	loaded     model.SpecSection // last stored version of the selected section
	editMode   bool
	modified   bool
	syncing    bool // true while the view itself sets entry text
}

func NewSpecView(st *store.Store, win fyne.Window) *SpecView {
	v := &SpecView{store: st, window: win}

	v.sectionList = widget.NewList(
		func() int { return len(v.index) },
		func() fyne.CanvasObject {
			l := widget.NewLabel("Section")
			l.Truncation = fyne.TextTruncateEllipsis
			return l
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			if id < len(v.index) {
				o.(*widget.Label).SetText(v.index[id].Title)
			}
		},
	)
	v.sectionList.OnSelected = func(id widget.ListItemID) {
		if id < len(v.index) && v.index[id].ID != v.selectedID {
			v.selectSection(v.index[id].ID)
		}
	}

	v.richText = widget.NewRichTextFromMarkdown("")
	v.richText.Wrapping = fyne.TextWrapWord
	v.richScroll = container.NewVScroll(container.NewPadded(v.richText))

	v.titleLabel = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	v.titleLabel.Truncation = fyne.TextTruncateEllipsis
	v.titleEntry = widget.NewEntry()
	v.titleEntry.SetPlaceHolder("Section title")
	v.bodyEntry = widget.NewMultiLineEntry()
	v.bodyEntry.Wrapping = fyne.TextWrapWord
	v.bodyEntry.SetPlaceHolder("Write this section in markdown...\n\nReference ticket IDs explicitly (e.g. #1, #2). Tickets not referenced in the specification are considered out of scope or deprecated.")
	v.titleEntry.OnChanged = func(string) { v.updateModified() }
	v.bodyEntry.OnChanged = func(string) { v.updateModified() }

	v.editFields = container.NewBorder(v.titleEntry, nil, nil, nil, v.bodyEntry)
	v.contentStack = container.NewStack(v.richScroll, v.editFields)

	v.status = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Italic: true})
	v.previewBtn = widget.NewButtonWithIcon("Preview", theme.VisibilityIcon(), func() { v.setEditMode(false) })
	v.editBtn = widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), func() { v.setEditMode(true) })
	v.saveBtn = widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() { v.Save() })
	v.saveBtn.Importance = widget.HighImportance

	addBtn := widget.NewButtonWithIcon("Add section", theme.ContentAddIcon(), v.showAddSection)
	addBtn.Importance = widget.LowImportance
	v.upBtn = widget.NewButtonWithIcon("", theme.MoveUpIcon(), func() { v.moveSelected(-1) })
	v.downBtn = widget.NewButtonWithIcon("", theme.MoveDownIcon(), func() { v.moveSelected(1) })
	v.deleteBtn = widget.NewButtonWithIcon("", theme.DeleteIcon(), v.confirmDelete)
	for _, b := range []*widget.Button{v.upBtn, v.downBtn, v.deleteBtn} {
		b.Importance = widget.LowImportance
	}

	sidebar := container.NewBorder(
		container.NewVBox(widget.NewLabelWithStyle("Sections", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), widget.NewSeparator()),
		container.NewVBox(widget.NewSeparator(), container.NewHBox(addBtn, layout.NewSpacer(), v.upBtn, v.downBtn, v.deleteBtn)),
		nil, nil,
		v.sectionList,
	)
	header := container.NewBorder(nil, nil, nil,
		container.NewHBox(v.status, v.previewBtn, v.editBtn, v.saveBtn),
		v.titleLabel,
	)
	editor := container.NewBorder(container.NewVBox(header, widget.NewSeparator()), nil, nil, nil, v.contentStack)

	split := container.NewHSplit(sidebar, editor)
	split.SetOffset(0.28)
	v.Container = container.NewThemeOverride(split, newSpecTheme(fyne.CurrentApp().Settings().Theme()))

	v.setEditMode(false)
	v.updateButtons()
	return v
}

// Refresh reloads the index for the active project and the selected section
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
		v.loaded = model.SpecSection{}
		v.modified = false
		v.status.SetText("")
	}

	v.index = nil
	if slug != "" {
		v.index, _ = v.store.ListSpecSections(slug)
	}
	v.sectionList.Refresh()

	if len(v.index) == 0 {
		v.selectedID = ""
		v.loaded = model.SpecSection{}
		v.titleLabel.SetText("")
		v.setEntries("", "")
		if slug == "" {
			v.richText.ParseMarkdown(specNoProjectMarkdown)
		} else {
			v.richText.ParseMarkdown(specEmptyMarkdown)
		}
		v.updateButtons()
		return
	}

	target := v.selectedID
	if v.indexOf(target) < 0 {
		target = v.index[0].ID
		v.modified = false
	}
	if target != v.selectedID {
		v.selectSection(target)
		return
	}

	// Same section: reload only if the stored version is newer.
	if info := v.index[v.indexOf(target)]; !info.UpdatedAt.Equal(v.loaded.UpdatedAt) {
		if v.modified {
			v.status.SetText("Changed elsewhere")
		} else {
			v.loadSection(target)
			v.status.SetText("Updated")
		}
	}
	v.sectionList.Select(v.indexOf(target))
	v.updateButtons()
}

func (v *SpecView) indexOf(id string) int {
	for i, info := range v.index {
		if info.ID == id {
			return i
		}
	}
	return -1
}

// selectSection saves pending edits and loads another section.
func (v *SpecView) selectSection(id string) {
	if v.modified {
		v.Save()
	}
	v.selectedID = id
	v.loadSection(id)
	v.status.SetText("")
	if i := v.indexOf(id); i >= 0 {
		v.sectionList.Select(i)
	}
	v.updateButtons()
}

func (v *SpecView) loadSection(id string) {
	sections, err := v.store.GetSpecSections(v.slug, []string{id})
	if err != nil || len(sections) == 0 {
		return
	}
	v.loaded = sections[0]
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

func (v *SpecView) renderPreview() {
	body := v.bodyEntry.Text
	if v.selectedID == "" {
		return
	}
	if strings.TrimSpace(body) == "" {
		body = "*This section is empty. Click 'Edit' to write it.*"
	}
	v.richText.ParseMarkdown(body)
	v.richScroll.ScrollToTop()
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
	setEnabled := func(b *widget.Button, on bool) {
		if on {
			b.Enable()
		} else {
			b.Disable()
		}
	}
	setEnabled(v.editBtn, i >= 0)
	setEnabled(v.saveBtn, i >= 0)
	setEnabled(v.deleteBtn, i >= 0)
	setEnabled(v.upBtn, i > 0)
	setEnabled(v.downBtn, i >= 0 && i < len(v.index)-1)
}

// Save stores the selected section if it has local changes.
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
	v.setEditMode(false)
}

func (v *SpecView) showAddSection() {
	if v.slug == "" {
		return
	}
	title := widget.NewEntry()
	title.SetPlaceHolder("e.g. Architecture")
	d := dialog.NewForm("Add section", "Add", "Cancel",
		[]*widget.FormItem{widget.NewFormItem("Title", title)},
		func(ok bool) {
			if !ok || strings.TrimSpace(title.Text) == "" {
				return
			}
			if v.modified {
				v.Save()
			}
			position := -1
			if i := v.indexOf(v.selectedID); i >= 0 {
				position = i + 1
			}
			sec, err := v.store.AddSpecSection(v.slug, title.Text, "", position)
			if err != nil {
				dialog.ShowError(err, v.window)
				return
			}
			v.index, _ = v.store.ListSpecSections(v.slug)
			v.sectionList.Refresh()
			v.selectSection(sec.ID)
			v.setEditMode(true)
		}, v.window)
	d.Resize(fyne.NewSize(420, 160))
	d.Show()
}

func (v *SpecView) moveSelected(delta int) {
	i := v.indexOf(v.selectedID)
	if i < 0 {
		return
	}
	if v.modified {
		v.Save()
	}
	if err := v.store.MoveSpecSection(v.slug, v.selectedID, i+delta); err != nil {
		v.status.SetText("Could not move")
		return
	}
	v.index, _ = v.store.ListSpecSections(v.slug)
	v.sectionList.Refresh()
	v.sectionList.Select(v.indexOf(v.selectedID))
	v.updateButtons()
}

func (v *SpecView) confirmDelete() {
	if v.selectedID == "" {
		return
	}
	id, title := v.selectedID, v.loaded.Title
	dialog.ShowConfirm("Delete section",
		fmt.Sprintf("Delete the section %q? This cannot be undone.", title),
		func(ok bool) {
			if !ok {
				return
			}
			if err := v.store.DeleteSpecSection(v.slug, id); err != nil {
				dialog.ShowError(err, v.window)
				return
			}
			v.modified = false
			v.selectedID = ""
			v.Refresh()
		}, v.window)
}
