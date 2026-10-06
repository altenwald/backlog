package ui

import (
	"runtime"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/altenwald/backlog/pkg/model"
)

func heapInUseMB() float64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.HeapInuse) / (1 << 20)
}

// Recreating task rows must not keep loading fonts: a ThemeOverride per row
// gave each one its own Fyne font cache scope (~14 MB) that is never freed.
func TestTaskRowsDoNotLeakFonts(t *testing.T) {
	a := test.NewApp()
	a.Settings().SetTheme(NewBacklogTheme())
	w := a.NewWindow("rows")
	w.Resize(fyne.NewSize(400, 80))

	render := func(n int) {
		for i := 0; i < n; i++ {
			item := NewTaskRowItem(nil)
			item.Bind(model.Task{ID: "1", Title: "Título con emoji 🚀", Size: model.SizeM, Tier: model.Tier2})
			w.SetContent(item)
			_ = w.Canvas().Capture()
		}
	}

	render(5) // warm up the shared font cache
	before := heapInUseMB()
	render(40)
	if grown := heapInUseMB() - before; grown > 20 {
		t.Fatalf("heap grew %.1f MB after recreating 40 rows", grown)
	}
}
