package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// RenderMarkdown parses markdown text into a rich text Fyne widget with full
// formatting support (headings, bold, italics, inline code, code blocks, lists, links).
func RenderMarkdown(content string) fyne.CanvasObject {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return widget.NewLabel("")
	}
	rt := widget.NewRichTextFromMarkdown(trimmed)
	rt.Wrapping = fyne.TextWrapWord
	return rt
}
