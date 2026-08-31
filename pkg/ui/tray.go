package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/altenwald/backlog/pkg/store"
)

type TrayManager struct {
	app        fyne.App
	window     fyne.Window
	store      *store.Store
	iconRes    fyne.Resource
	onAddClick func()
}

func NewTrayManager(app fyne.App, window fyne.Window, st *store.Store, onAddClick func()) *TrayManager {
	tm := &TrayManager{
		app:        app,
		window:     window,
		store:      st,
		iconRes:    createDefaultTrayIcon(),
		onAddClick: onAddClick,
	}

	if desk, ok := app.(desktop.App); ok {
		desk.SetSystemTrayIcon(tm.iconRes)
	}

	tm.Refresh()
	return tm
}

func (tm *TrayManager) Refresh() {
	desk, ok := tm.app.(desktop.App)
	if !ok {
		return
	}

	activeSlug := tm.store.GetActiveProjectSlug()
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

	// Status Header Item
	statusTitle := fmt.Sprintf("[%s] %d/%d Pending", activeProject.Name, openCount, totalCount)
	statusItem := fyne.NewMenuItem(statusTitle, func() {
		tm.window.Show()
		tm.window.RequestFocus()
	})

	// Top 5 Prioritarias
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

	quitItem := fyne.NewMenuItem("Quit", func() {
		tm.app.Quit()
	})

	menuItems := []*fyne.MenuItem{
		statusItem,
		fyne.NewMenuItemSeparator(),
	}
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

func createDefaultTrayIcon() fyne.Resource {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	// Draw background circle
	blue := color.NRGBA{R: 59, G: 125, B: 212, A: 255}
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}

	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.Transparent}, image.Point{}, draw.Src)

	// Simple crisp checkmark icon
	for x := 4; x < 28; x++ {
		for y := 4; y < 28; y++ {
			dx := x - 16
			dy := y - 16
			if dx*dx+dy*dy <= 144 {
				img.Set(x, y, blue)
			}
		}
	}
	// Checkmark dots
	for i := 0; i < 4; i++ {
		img.Set(10+i, 16+i, white)
		img.Set(10+i, 17+i, white)
	}
	for i := 0; i < 8; i++ {
		img.Set(14+i, 20-i, white)
		img.Set(14+i, 21-i, white)
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return fyne.NewStaticResource("tray.png", buf.Bytes())
}
