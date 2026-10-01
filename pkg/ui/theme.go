package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// BacklogTheme uses the semi-monospaced Recursive reading font (with Fyne's
// emoji fallback) and vector icons, with a restrained palette that follows the
// system's light/dark preference.
type BacklogTheme struct{ fyne.Theme }

func NewBacklogTheme() fyne.Theme { return &BacklogTheme{theme.DefaultTheme()} }

func (t *BacklogTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	dark := variant == theme.VariantDark
	pair := func(light, night uint32) color.Color {
		v := light
		if dark {
			v = night
		}
		return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}
	}
	switch name {
	case theme.ColorNameBackground:
		return pair(0xF6F7F9, 0x171C24)
	case theme.ColorNameForeground:
		return pair(0x253140, 0xE5EAF0)
	case theme.ColorNamePrimary:
		return pair(0x267B79, 0x75C5BD)
	case theme.ColorNameButton:
		return pair(0xE8EDF0, 0x2A333F)
	case theme.ColorNameInputBackground, theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return pair(0xFFFFFF, 0x202732)
	case theme.ColorNameInputBorder, theme.ColorNameSeparator:
		return pair(0xD6DEE5, 0x35404E)
	case theme.ColorNameForegroundOnPrimary:
		return pair(0xFFFFFF, 0x152D2C)
	case theme.ColorNamePlaceHolder, theme.ColorNameDisabled:
		return pair(0x647181, 0xA0ADBC)
	case theme.ColorNameSelection:
		return pair(0xD5EAE7, 0x304B50)
	case theme.ColorNameHover:
		return pair(0xEAF0F2, 0x293440)
	case theme.ColorNameFocus:
		return pair(0xC6E0DD, 0x3C6565)
	}
	return t.Theme.Color(name, variant)
}

func (t *BacklogTheme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Monospace || style.Symbol {
		return t.Theme.Font(style)
	}
	return specFont(style)
}

func (t *BacklogTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 13
	case theme.SizeNamePadding:
		return 6
	case theme.SizeNameInnerPadding:
		return 9
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 6
	case theme.SizeNameHeadingText:
		return 24
	case theme.SizeNameSubHeadingText:
		return 18
	}
	return t.Theme.Size(name)
}

// DependencyIcon is independent of emoji and font fallback support.
func DependencyIcon() fyne.Resource {
	return theme.NewThemedResource(fyne.NewStaticResource("dependency.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="#000" d="M3.9 12c0-1.71 1.39-3.1 3.1-3.1h4V7H7a5 5 0 0 0 0 10h4v-1.9H7A3.1 3.1 0 0 1 3.9 12ZM8 13h8v-2H8v2Zm9-6h-4v1.9h4a3.1 3.1 0 0 1 0 6.2h-4V17h4a5 5 0 0 0 0-10Z"/></svg>`)))
}

// typographyTheme changes density without replacing the active color palette.
type typographyTheme struct {
	fyne.Theme
	textSize, inset float32
}

func (t typographyTheme) Size(n fyne.ThemeSizeName) float32 {
	if n == theme.SizeNameText && t.textSize > 0 {
		return t.textSize
	}
	if n == theme.SizeNameInnerPadding {
		return t.inset
	}
	return t.Theme.Size(n)
}

func branchIcon() fyne.Resource {
	return theme.NewThemedResource(fyne.NewStaticResource("branch.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="#000" d="M5 3h2v10h10l-4-4 1.4-1.4L21 14l-6.6 6.4L13 19l4-4H5Z"/></svg>`)))
}
