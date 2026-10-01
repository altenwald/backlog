package ui

import (
	"bytes"
	"testing"

	"fyne.io/fyne/v2"
	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/language"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

func shapeGlyphs(t *testing.T, data []byte, text string) int {
	t.Helper()
	face, err := font.ParseTTF(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	runes := []rune(text)
	out := (&shaping.HarfbuzzShaper{}).Shape(shaping.Input{
		Text: runes, RunStart: 0, RunEnd: len(runes),
		Direction: di.DirectionLTR, Face: face, Size: fixed.I(16),
		Script: language.Latin, Language: language.NewLanguage("en"),
	})
	return len(out.Glyphs)
}

// Code ligatures must be on by default, since Fyne cannot enable 'dlig'.
func TestSpecFontLigatures(t *testing.T) {
	for _, style := range []fyne.TextStyle{{}, {Bold: true}, {Italic: true}, {Bold: true, Italic: true}} {
		data := specFont(style).Content()
		for _, s := range []string{"->", "=>", "!=", "<="} {
			if n := shapeGlyphs(t, data, s); n >= len([]rune(s)) {
				t.Errorf("style %+v: %q shaped into %d glyphs, expected a ligature", style, s, n)
			}
		}
	}
}
