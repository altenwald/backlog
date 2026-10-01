package ui

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

// The UI uses a semi-monospaced instance of Recursive
// (SIL OFL 1.1, see assets/fonts/OFL.txt): MONO=0.5, CASL=0, with its code
// ligatures moved from 'dlig' to 'liga' so Fyne's shaper applies them.
// tools/fonts/build_spec_font.py regenerates the files.
// readingLeading is the extra line height (in em) built into the font's
// vertical metrics: 5 px at the 13 px UI text size.
const readingLeading = 5.0 / 13

var (
	//go:embed assets/fonts/RecursiveSemimono-Regular.ttf
	specFontRegular []byte
	//go:embed assets/fonts/RecursiveSemimono-Bold.ttf
	specFontBold []byte
	//go:embed assets/fonts/RecursiveSemimono-Italic.ttf
	specFontItalic []byte
	//go:embed assets/fonts/RecursiveSemimono-BoldItalic.ttf
	specFontBoldItalic []byte

	specFontRegularRes    = fyne.NewStaticResource("RecursiveSemimono-Regular.ttf", specFontRegular)
	specFontBoldRes       = fyne.NewStaticResource("RecursiveSemimono-Bold.ttf", specFontBold)
	specFontItalicRes     = fyne.NewStaticResource("RecursiveSemimono-Italic.ttf", specFontItalic)
	specFontBoldItalicRes = fyne.NewStaticResource("RecursiveSemimono-BoldItalic.ttf", specFontBoldItalic)
)

// specFont returns the reading font for a text style. It is applied app-wide
// by BacklogTheme: Fyne measures text with the application theme font, so a
// per-panel theme override with a different font gets clipped layouts.
func specFont(style fyne.TextStyle) fyne.Resource {
	switch {
	case style.Bold && style.Italic:
		return specFontBoldItalicRes
	case style.Bold:
		return specFontBoldRes
	case style.Italic:
		return specFontItalicRes
	default:
		return specFontRegularRes
	}
}
