package ui

import (
	"fyne.io/fyne/v2"
)

// specTheme renders the specification panel with its own reading font while
// keeping every color and size of the base theme. Emoji keep working through
// Fyne's bundled emoji fallback font.
type specTheme struct {
	fyne.Theme
}

func newSpecTheme(base fyne.Theme) fyne.Theme { return &specTheme{Theme: base} }

func (t *specTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Monospace || style.Symbol {
		return t.Theme.Font(style)
	}
	if res := specFont(style); res != nil {
		return res
	}
	return t.Theme.Font(style)
}

// specFont returns the embedded reading font for style, or nil while none is bundled.
func specFont(style fyne.TextStyle) fyne.Resource { return nil }
