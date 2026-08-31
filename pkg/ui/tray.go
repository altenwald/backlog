package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/systray"
	"github.com/altenwald/backlog/pkg/store"
)

type TrayManager struct {
	app        fyne.App
	window     fyne.Window
	store      *store.Store
	iconBytes  []byte
	iconRes    fyne.Resource
	onAddClick func()
}

func NewTrayManager(app fyne.App, window fyne.Window, st *store.Store, onAddClick func()) *TrayManager {
	rawPNG := createBacklogLineLogo(32)
	baseRes := fyne.NewStaticResource("backlog_tray.png", rawPNG)
	// Use ThemedResource so Fyne sets it as macOS template icon on initial load
	themedRes := theme.NewThemedResource(baseRes)

	tm := &TrayManager{
		app:        app,
		window:     window,
		store:      st,
		iconBytes:  rawPNG,
		iconRes:    themedRes,
		onAddClick: onAddClick,
	}

	if desk, ok := app.(desktop.App); ok {
		desk.SetSystemTrayIcon(tm.iconRes)
	}

	tm.Refresh()

	// Ensure title and template icon are synced as soon as native macOS Cocoa event loop initializes
	go func() {
		for _, delay := range []time.Duration{50 * time.Millisecond, 150 * time.Millisecond, 300 * time.Millisecond, 600 * time.Millisecond, 1000 * time.Millisecond} {
			time.Sleep(delay)
			tm.Refresh()
		}
	}()

	return tm
}

func (tm *TrayManager) Refresh() {
	desk, ok := tm.app.(desktop.App)
	if !ok {
		return
	}

	activeSlug := tm.store.GetActiveProjectSlug()
	if activeSlug == "" {
		activeSlug = "dymmer"
		_ = tm.store.SetActiveProject("dymmer")
	}

	activeProject, err := tm.store.GetProject(activeSlug)
	if err != nil || activeProject == nil {
		return
	}

	summary, _ := tm.store.GetSummary(activeSlug)
	openCount := 0
	totalCount := 0
	if summary != nil {
		openCount = summary.OpenTasks
		totalCount = summary.TotalTasks
	}

	// Update Menu bar title and template icon
	systray.SetTemplateIcon(tm.iconBytes, tm.iconBytes)
	systray.SetTitle(fmt.Sprintf(" %s %d/%d", activeProject.Name, openCount, totalCount))
	systray.SetTooltip(fmt.Sprintf("Backlog — [%s] %d/%d pending tasks", activeProject.Name, openCount, totalCount))

	// Top 5 Priorities
	topTasks, _ := tm.store.GetTopPriorities(activeSlug, 5)
	var topMenuItems []*fyne.MenuItem
	if len(topTasks) == 0 {
		topMenuItems = append(topMenuItems, fyne.NewMenuItem("✨ All caught up! No pending tasks", nil))
	} else {
		for i, t := range topTasks {
			taskCopy := t
			title := fmt.Sprintf("%d. [%s] [%s] %s", i+1, taskCopy.Size, taskCopy.Tier.ShortLabel(), taskCopy.Title)
			item := fyne.NewMenuItem(title, func() {
				tm.window.Show()
				tm.window.RequestFocus()
			})
			topMenuItems = append(topMenuItems, item)
		}
	}

	// Projects Submenu
	projects := tm.store.ListProjects()
	var projectMenuItems []*fyne.MenuItem
	for _, p := range projects {
		pSlug := p.Slug
		pName := p.Name
		pSum, _ := tm.store.GetSummary(pSlug)
		label := fmt.Sprintf("%s (%d/%d)", pName, pSum.OpenTasks, pSum.TotalTasks)
		if pSlug == activeSlug {
			label = "● " + label
		} else {
			label = "   " + label
		}
		item := fyne.NewMenuItem(label, func() {
			_ = tm.store.SetActiveProject(pSlug)
		})
		projectMenuItems = append(projectMenuItems, item)
	}
	projectsSubmenu := fyne.NewMenuItem("📁 Switch Project", nil)
	projectsSubmenu.ChildMenu = fyne.NewMenu("Projects", projectMenuItems...)

	// Actions
	addItem := fyne.NewMenuItem("➕ New Task...", func() {
		tm.window.Show()
		tm.window.RequestFocus()
		if tm.onAddClick != nil {
			tm.onAddClick()
		}
	})

	openItem := fyne.NewMenuItem("🪟 Open Backlog", func() {
		tm.window.Show()
		tm.window.RequestFocus()
	})

	// Set IsQuit to true to avoid duplicate localized system quit item
	quitItem := fyne.NewMenuItem("Quit Backlog", func() {
		tm.app.Quit()
	})
	quitItem.IsQuit = true

	var menuItems []*fyne.MenuItem
	menuItems = append(menuItems, topMenuItems...)
	menuItems = append(menuItems,
		fyne.NewMenuItemSeparator(),
		projectsSubmenu,
		addItem,
		openItem,
		fyne.NewMenuItemSeparator(),
		quitItem,
	)

	menu := fyne.NewMenu("Backlog", menuItems...)
	desk.SetSystemTrayMenu(menu)
}

// createBacklogLineLogo generates a wireframe line-art icon:
// A stroke box outline with an empty center and a checkmark that extends past the top-right corner.
func createBacklogLineLogo(dim int) []byte {
	if dim <= 0 {
		dim = 32
	}
	img := image.NewRGBA(image.Rect(0, 0, dim, dim))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.Transparent}, image.Point{}, draw.Src)

	// Scale factor relative to 32x32 base
	scale := float64(dim) / 32.0
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}

	boxStroke := 2.2 * scale
	checkStroke := 2.6 * scale

	// 1. Box outline (hollow wireframe)
	// Left side
	drawLine(img, 6.0*scale, 9.0*scale, 6.0*scale, 26.0*scale, boxStroke, black)
	// Bottom side
	drawLine(img, 6.0*scale, 26.0*scale, 23.0*scale, 26.0*scale, boxStroke, black)
	// Right side (ends before checkmark)
	drawLine(img, 23.0*scale, 26.0*scale, 23.0*scale, 14.0*scale, boxStroke, black)
	// Top side (ends before checkmark)
	drawLine(img, 6.0*scale, 9.0*scale, 15.0*scale, 9.0*scale, boxStroke, black)

	// 2. Checkmark extending outside the top-right corner of the box
	// Short branch
	drawLine(img, 8.5*scale, 17.5*scale, 13.5*scale, 22.5*scale, checkStroke, black)
	// Long branch (crosses the box and extends to (27.5, 5.5))
	drawLine(img, 13.5*scale, 22.5*scale, 27.5*scale, 5.5*scale, checkStroke, black)

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func drawLine(img *image.RGBA, x0, y0, x1, y1 float64, width float64, c color.RGBA) {
	minX := int(math.Floor(math.Min(x0, x1) - width - 1))
	maxX := int(math.Ceil(math.Max(x0, x1) + width + 1))
	minY := int(math.Floor(math.Min(y0, y1) - width - 1))
	maxY := int(math.Ceil(math.Max(y0, y1) + width + 1))

	b := img.Bounds()
	if minX < b.Min.X {
		minX = b.Min.X
	}
	if minY < b.Min.Y {
		minY = b.Min.Y
	}
	if maxX > b.Max.X {
		maxX = b.Max.X
	}
	if maxY > b.Max.Y {
		maxY = b.Max.Y
	}

	dx := x1 - x0
	dy := y1 - y0
	lenSq := dx*dx + dy*dy
	halfW := width / 2.0

	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			px := float64(x) + 0.5
			py := float64(y) + 0.5

			var t float64
			if lenSq > 0 {
				t = ((px-x0)*dx + (py-y0)*dy) / lenSq
				if t < 0 {
					t = 0
				}
				if t > 1 {
					t = 1
				}
			}

			projX := x0 + t*dx
			projY := y0 + t*dy
			dist := math.Sqrt((px-projX)*(px-projX) + (py-projY)*(py-projY))

			if dist <= halfW {
				img.SetRGBA(x, y, c)
			} else if dist < halfW+1.0 {
				alphaRatio := 1.0 - (dist - halfW)
				if alphaRatio > 0 {
					targetAlpha := uint8(float64(c.A) * alphaRatio)
					orig := img.RGBAAt(x, y)
					if targetAlpha > orig.A {
						img.SetRGBA(x, y, color.RGBA{R: c.R, G: c.G, B: c.B, A: targetAlpha})
					}
				}
			}
		}
	}
}
