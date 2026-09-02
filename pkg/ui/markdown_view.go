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
			itemLabel := widget.NewLabel(itemText)
			itemLabel.Wrapping = fyne.TextWrapWord
			row := container.NewBorder(nil, nil, bullet, nil, itemLabel)
			box.Add(row)
			continue
		}

		// Numbered lists (1. item, 2. item)
		if len(trimmed) > 3 && trimmed[0] >= '0' && trimmed[0] <= '9' && trimmed[1] == '.' && trimmed[2] == ' ' {
			numPrefix := trimmed[:2]
			itemText := strings.TrimSpace(trimmed[3:])
			numLabel := widget.NewLabelWithStyle(numPrefix, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			itemLabel := widget.NewLabel(itemText)
			itemLabel.Wrapping = fyne.TextWrapWord
			row := container.NewBorder(nil, nil, numLabel, nil, itemLabel)
			box.Add(row)
			continue
		}

		// Standard paragraphs
		paraLabel := widget.NewLabel(trimmed)
		paraLabel.Wrapping = fyne.TextWrapWord
		box.Add(paraLabel)
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
