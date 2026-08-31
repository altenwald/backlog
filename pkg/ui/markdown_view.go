package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// RenderMarkdown parses markdown text into clean Fyne widgets with proper word wrapping,
// styled code blocks, inline code backticks, bold spans, bullet points, and headings.
func RenderMarkdown(content string) fyne.CanvasObject {
	lines := strings.Split(content, "\n")
	box := container.NewVBox()

	inCodeBlock := false
	var codeLines []string

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Fenced code block toggle
		if strings.HasPrefix(trimmed, "```") {
			if inCodeBlock {
				codeText := strings.Join(codeLines, "\n")
				codeWidget := widget.NewLabelWithStyle(codeText, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})
				codeWidget.Wrapping = fyne.TextWrapWord

				codeBg := canvas.NewRectangle(theme.ButtonColor())
				codeBg.CornerRadius = 6

				codeBox := container.NewStack(codeBg, container.NewPadded(codeWidget))
				box.Add(codeBox)

				codeLines = nil
				inCodeBlock = false
			} else {
				inCodeBlock = true
				codeLines = nil
			}
			continue
		}

		if inCodeBlock {
			codeLines = append(codeLines, line)
			continue
		}

		if trimmed == "" {
			continue
		}

		// Headings (# Heading, ## Subheading)
		if strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			hText := strings.TrimLeft(trimmed, "# ")
			hLabel := widget.NewLabelWithStyle(hText, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			hLabel.Wrapping = fyne.TextWrapWord
			box.Add(hLabel)
			continue
		}

		// Bullet lists (- item, * item, • item)
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "• ") {
			itemText := strings.TrimSpace(trimmed[2:])
			bullet := widget.NewLabelWithStyle("•", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			itemWidget := createInlineRichText(itemText)
			row := container.NewBorder(nil, nil, bullet, nil, itemWidget)
			box.Add(row)
			continue
		}

		// Numbered lists (1. item, 2. item)
		if len(trimmed) > 3 && trimmed[0] >= '0' && trimmed[0] <= '9' && trimmed[1] == '.' && trimmed[2] == ' ' {
			numPrefix := trimmed[:2]
			itemText := strings.TrimSpace(trimmed[3:])
			numLabel := widget.NewLabelWithStyle(numPrefix, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			itemWidget := createInlineRichText(itemText)
			row := container.NewBorder(nil, nil, numLabel, nil, itemWidget)
			box.Add(row)
			continue
		}

		// Standard paragraph with inline backtick and bold parsing
		paraWidget := createInlineRichText(trimmed)
		box.Add(paraWidget)
	}

	// Flush unclosed code block if any
	if inCodeBlock && len(codeLines) > 0 {
		codeText := strings.Join(codeLines, "\n")
		codeWidget := widget.NewLabelWithStyle(codeText, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})
		codeWidget.Wrapping = fyne.TextWrapWord

		codeBg := canvas.NewRectangle(theme.ButtonColor())
		codeBg.CornerRadius = 6

		codeBox := container.NewStack(codeBg, container.NewPadded(codeWidget))
		box.Add(codeBox)
	}

	return box
}

func createInlineRichText(text string) *widget.RichText {
	segs := parseInlineSpans(text)
	rt := widget.NewRichText(segs...)
	rt.Wrapping = fyne.TextWrapWord
	return rt
}

// parseInlineSpans extracts backtick code (`code`) and bold (**bold**) spans into rich text segments.
func parseInlineSpans(text string) []widget.RichTextSegment {
	var segs []widget.RichTextSegment
	runes := []rune(text)
	n := len(runes)

	start := 0
	i := 0

	for i < n {
		if runes[i] == '`' {
			// Flush previous text
			if i > start {
				segs = append(segs, &widget.TextSegment{
					Text:  string(runes[start:i]),
					Style: widget.RichTextStyleInline,
				})
			}
			// Search for closing backtick
			end := i + 1
			for end < n && runes[end] != '`' {
				end++
			}
			if end < n && runes[end] == '`' {
				codeContent := string(runes[i+1 : end])
				segs = append(segs, &widget.TextSegment{
					Text:  codeContent,
					Style: widget.RichTextStyleCodeInline,
				})
				i = end + 1
				start = i
				continue
			} else {
				i++
				continue
			}
		} else if i+1 < n && runes[i] == '*' && runes[i+1] == '*' {
			// Flush previous text
			if i > start {
				segs = append(segs, &widget.TextSegment{
					Text:  string(runes[start:i]),
					Style: widget.RichTextStyleInline,
				})
			}
			// Search for closing **
			end := i + 2
			for end+1 < n && !(runes[end] == '*' && runes[end+1] == '*') {
				end++
			}
			if end+1 < n && runes[end] == '*' && runes[end+1] == '*' {
				boldContent := string(runes[i+2 : end])
				segs = append(segs, &widget.TextSegment{
					Text:  boldContent,
					Style: widget.RichTextStyleStrong,
				})
				i = end + 2
				start = i
				continue
			} else {
				i++
				continue
			}
		} else {
			i++
		}
	}

	// Flush trailing text
	if start < n {
		segs = append(segs, &widget.TextSegment{
			Text:  string(runes[start:]),
			Style: widget.RichTextStyleInline,
		})
	}

	if len(segs) == 0 {
		segs = append(segs, &widget.TextSegment{
			Text:  text,
			Style: widget.RichTextStyleInline,
		})
	}

	return segs
}
