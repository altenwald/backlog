package ui

import (
	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
)

//go:embed assets/icon.png
var appIconBytes []byte

type BacklogApp struct {
	fyneApp       fyne.App
	window        fyne.Window
	store         store.Backend
	tray          *TrayManager
	summaryBar    *SummaryBar
	filterBar     *FilterBar
	projectSelect *widget.Button
	currentFilter model.TaskFilter

	tasksList      *widget.List
	tasksEmpty     *fyne.Container
	detailView     *TaskDetailView
	burnUpChart    *BurnUpChart
	displayedTasks []model.Task
	taskPlacements map[string]taskPlacement
	selectedTaskID string

	specView      *SpecView
	addTaskButton *widget.Button
}

func GetAppIconResource() fyne.Resource {
	return fyne.NewStaticResource("icon.png", appIconBytes)
}

func NewBacklogApp(st store.Backend) *BacklogApp {
	a := app.NewWithID("com.altenwald.backlog")
	a.Settings().SetTheme(NewBacklogTheme())
	iconRes := GetAppIconResource()
	a.SetIcon(iconRes)

	w := a.NewWindow("Backlog")
	w.SetIcon(iconRes)
	w.Resize(fyne.NewSize(1180, 780))

	bApp := &BacklogApp{
		fyneApp: a,
		window:  w,
		store:   st,
	}

	bApp.buildUI()
	bApp.configureNetwork()

	// Set Main Menu with About and Settings items (intercepts macOS application menu)
	aboutMenuItem := fyne.NewMenuItem("About", func() {
		ShowAboutDialog(w)
	})
	settingsMenuItem := fyne.NewMenuItem("Settings…", func() {
		ShowSettingsDialog(w, st)
	})
	settingsMenuItem.Shortcut = &desktop.CustomShortcut{
		KeyName:  fyne.KeyComma,
		Modifier: fyne.KeyModifierShortcutDefault,
	}
	appMenu := fyne.NewMenu("Backlog", aboutMenuItem, settingsMenuItem)
	mainMenu := fyne.NewMainMenu(appMenu)
	w.SetMainMenu(mainMenu)

	// Stay resident in tray on window close
	w.SetCloseIntercept(func() {
		w.Hide()
	})

	// Subscribe to store changes to refresh UI live
	go bApp.listenEvents()

	return bApp
}

func (ba *BacklogApp) buildUI() {
	ba.summaryBar = NewSummaryBar(func(size *model.Size) {
		ba.filterBar.SetSizeFilter(size)
	})

	ba.filterBar = NewFilterBar(func(filter model.TaskFilter) {
		ba.currentFilter = filter
		ba.refreshTasks()
	})
	ba.currentFilter = ba.filterBar.CurrentFilter()

	// Project selector
	ba.projectSelect = widget.NewButton("Select project", func() {
		projects := ba.store.ListProjects()
		items := []*fyne.MenuItem{}
		group := ""
		for _, p := range projects {
			if p.RemoteID != group {
				group = p.RemoteID
				items = append(items, fyne.NewMenuItemSeparator())
				heading := fyne.NewMenuItem(p.Machine, nil)
				heading.Disabled = true
				items = append(items, heading)
			}
			slug := p.Slug
			label := p.Name
			if p.Disconnected {
				label += " (disconnected)"
			}
			item := fyne.NewMenuItem(label, func() {
				if ba.specView != nil && ba.specView.modified {
					dialog.ShowInformation("Unsaved page", "Save or discard your page edits before switching projects.", ba.window)
					return
				}
				if err := ba.store.SetActiveProject(slug); err != nil {
					dialog.ShowError(err, ba.window)
				}
			})
			items = append(items, item)
		}
		menu := widget.NewPopUpMenu(fyne.NewMenu("Projects", items...), ba.window.Canvas())
		menu.ShowAtPosition(fyne.CurrentApp().Driver().AbsolutePositionForObject(ba.projectSelect).Add(fyne.NewPos(0, ba.projectSelect.Size().Height)))
	})

	newProjectBtn := widget.NewButtonWithIcon("New project", theme.FolderNewIcon(), func() {
		ShowNewProjectDialog(ba.window, func(slug, name, desc string) {
			if _, err := ba.store.CreateProject(slug, name, desc); err == nil {
				_ = ba.store.SetActiveProject(slug)
			}
		})
	})
	newProjectBtn.Importance = widget.LowImportance

	deleteProjectBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		activeSlug := ba.store.GetActiveProjectSlug()
		if activeSlug == "" {
			return
		}
		p, err := ba.store.GetProject(activeSlug)
		pName := activeSlug
		if err == nil && p != nil {
			pName = p.Name
		}
		ShowDeleteProjectDialog(ba.window, pName, activeSlug, func() {
			ba.selectedTaskID = ""
			_ = ba.store.DeleteProject(activeSlug)
		})
	})
	deleteProjectBtn.Importance = widget.LowImportance

	addTaskBtn := widget.NewButtonWithIcon("New task", theme.ContentAddIcon(), func() {
		ba.showAddTask()
	})
	addTaskBtn.Importance = widget.HighImportance
	ba.addTaskButton = addTaskBtn

	headerLeft := container.NewHBox(
		widget.NewLabelWithStyle("Backlog", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWrap(fyne.NewSize(210, 38), ba.projectSelect),
		newProjectBtn,
		deleteProjectBtn,
		widget.NewButton("Sharing", ba.showSharing),
		widget.NewButton("Network", ba.showNetwork),
	)

	header := container.NewBorder(nil, nil, headerLeft, addTaskBtn)

	// Virtualized task list
	ba.tasksList = widget.NewList(
		func() int {
			return len(ba.displayedTasks)
		},
		func() fyne.CanvasObject {
			return NewTaskRowItem(func(taskID string, done bool) {
				activeSlug := ba.store.GetActiveProjectSlug()
				ba.toggleTask(activeSlug, taskID, &done, nil)
			})
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < 0 || id >= len(ba.displayedTasks) {
				return
			}
			item := obj.(*TaskRowItem)
			task := ba.displayedTasks[id]
			item.bindPlacement(task, ba.taskPlacements[task.ID])
		},
	)

	ba.tasksList.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && id < len(ba.displayedTasks) {
			task := ba.displayedTasks[id]
			ba.selectedTaskID = task.ID
			ba.detailView.ShowTask(task)
		}
	}

	// Left section (List pane)
	leftHeader := container.NewVBox(
		ba.summaryBar.CanvasObject(),
		ba.filterBar.CanvasObject(),
		widget.NewSeparator(),
	)

	emptyTitle := widget.NewLabelWithStyle("No tasks to show", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	emptyHint := widget.NewLabel("Try another filter or create a new task.")
	emptyHint.Alignment = fyne.TextAlignCenter
	emptyHint.Importance = widget.LowImportance
	ba.tasksEmpty = container.NewCenter(container.NewVBox(widget.NewIcon(theme.ListIcon()), emptyTitle, emptyHint))
	ba.tasksEmpty.Hide()
	leftPane := container.NewPadded(container.NewBorder(leftHeader, nil, nil, nil, container.NewStack(ba.tasksList, ba.tasksEmpty)))

	// Right section (Tabs: 1. Tasks & Details, 2. Specification Text Editor)
	detailCallbacks := TaskDetailCallbacks{
		OnToggleDone: func(taskID string, done bool) { ba.toggleTask(ba.store.GetActiveProjectSlug(), taskID, &done, nil) },
		OnDeprecate: func(taskID string, deprecated bool) {
			ba.toggleTask(ba.store.GetActiveProjectSlug(), taskID, nil, &deprecated)
		},
		OnEdit: func(task model.Task) { ba.editTask(ba.store.GetActiveProjectSlug(), task) },
		OnDelete: func(taskID string) {
			activeSlug := ba.store.GetActiveProjectSlug()
			for _, task := range ba.displayedTasks {
				if task.ID == taskID {
					version := task.UpdatedAt
					ba.runMutation(func() error { return ba.store.DeleteTask(activeSlug, taskID, version) })
					break
				}
			}
			ba.selectedTaskID = ""
			ba.detailView.Clear()
		},
	}
	ba.burnUpChart = NewBurnUpChart()
	ba.detailView = NewTaskDetailView(detailCallbacks)

	// Specification: section index plus a per-section preview/editor
	ba.specView = NewSpecView(ba.store, ba.window)

	rightTabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Task", theme.ListIcon(), container.NewPadded(ba.detailView.Container)),
		container.NewTabItemWithIcon("Progress", theme.HistoryIcon(), container.NewPadded(ba.burnUpChart.Container)),
		container.NewTabItemWithIcon("Specification", theme.DocumentIcon(), container.NewPadded(ba.specView.Container)),
	)

	// Master-detail Split view
	split := container.NewHSplit(leftPane, rightTabs)
	split.SetOffset(0.40)

	ba.window.SetContent(container.NewPadded(container.NewBorder(container.NewVBox(header, widget.NewSeparator()), nil, nil, nil, split)))

	// Setup Tray
	ba.tray = NewTrayManager(ba.fyneApp, ba.window, ba.store, func() {
		ba.showAddTask()
	})

	ba.refreshAll()
}

func (ba *BacklogApp) showAddTask() {
	activeSlug := ba.store.GetActiveProjectSlug()
	ShowAddTaskDialog(ba.window, activeSlug, func(task model.Task) {
		ba.runMutation(func() error { _, err := ba.store.AddTask(activeSlug, task); return err })
	})
}

func (ba *BacklogApp) refreshProjects() {
	projects := ba.store.ListProjects()
	activeSlug := ba.store.GetActiveProjectSlug()

	label := "Select project"
	for _, p := range projects {
		if p.Slug == activeSlug {
			label = p.Name
			if p.Machine != "" {
				label = p.Machine + " / " + label
			}
			if p.Disconnected {
				label += " (disconnected)"
			}
			break
		}
	}
	ba.projectSelect.SetText(label)
}

func (ba *BacklogApp) refreshTasks() {
	defer func() {
		if ba.tasksEmpty != nil {
			if len(ba.displayedTasks) == 0 {
				ba.tasksEmpty.Show()
			} else {
				ba.tasksEmpty.Hide()
			}
		}
	}()
	activeSlug := ba.store.GetActiveProjectSlug()
	if activeSlug == "" {
		ba.displayedTasks = nil
		ba.selectedTaskID = ""
		ba.tasksList.UnselectAll()
		ba.tasksList.Refresh()
		ba.detailView.Clear()
		if ba.burnUpChart != nil {
			ba.burnUpChart.Update(nil)
		}
		return
	}

	tasks, err := ba.store.ListTasks(activeSlug, ba.currentFilter)
	if ba.addTaskButton != nil {
		if err != nil {
			ba.addTaskButton.Disable()
		} else {
			ba.addTaskButton.Enable()
		}
	}
	if err != nil {
		ba.displayedTasks = nil
		ba.selectedTaskID = ""
		ba.tasksList.UnselectAll()
		ba.tasksList.Refresh()
		ba.detailView.Clear()
		if ba.burnUpChart != nil {
			ba.burnUpChart.Update(nil)
		}
		return
	}

	ba.displayedTasks, ba.taskPlacements = arrangeTaskOutline(tasks)
	ba.tasksList.Refresh()

	// Update Burn-up chart with full project scope
	if ba.burnUpChart != nil {
		if allTasks, err := ba.store.ListTasks(activeSlug, model.TaskFilter{}); err == nil {
			ba.burnUpChart.Update(allTasks)
		}
	}

	if len(ba.displayedTasks) == 0 {
		ba.selectedTaskID = ""
		ba.tasksList.UnselectAll()
		ba.detailView.Clear()
		return
	}

	selectedIndex := -1
	if ba.selectedTaskID != "" {
		for i, t := range ba.displayedTasks {
			if t.ID == ba.selectedTaskID {
				selectedIndex = i
				ba.detailView.ShowTask(t)
				break
			}
		}
	}

	if selectedIndex >= 0 {
		ba.tasksList.Select(selectedIndex)
	} else {
		ba.selectedTaskID = ba.displayedTasks[0].ID
		ba.tasksList.UnselectAll()
		ba.tasksList.Select(0)
		ba.detailView.ShowTask(ba.displayedTasks[0])
	}
}

func (ba *BacklogApp) refreshSpec() {
	if ba.specView != nil {
		ba.specView.Refresh()
	}
}

func (ba *BacklogApp) refreshAll() {
	ba.refreshProjects()
	activeSlug := ba.store.GetActiveProjectSlug()
	sum, _ := ba.store.GetSummary(activeSlug)
	ba.summaryBar.Update(sum)
	ba.filterBar.UpdateCounts(sum)
	ba.refreshTasks()
	ba.refreshSpec()
	if ba.tray != nil {
		ba.tray.Refresh()
	}
}

func (ba *BacklogApp) listenEvents() {
	ch, cancel := ba.store.Watch()
	defer cancel()
	for range ch {
		fyne.Do(func() {
			ba.refreshAll()
		})
	}
}

func (ba *BacklogApp) Run() {
	ba.window.ShowAndRun()
}
