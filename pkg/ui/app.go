package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
)

type BacklogApp struct {
	fyneApp       fyne.App
	window        fyne.Window
	store         *store.Store
	tray          *TrayManager
	summaryBar    *SummaryBar
	filterBar     *FilterBar
	projectSelect *widget.Select
	currentFilter model.TaskFilter

	tasksList      *widget.List
	detailView     *TaskDetailView
	displayedTasks []model.Task
	selectedTaskID string
}

func NewBacklogApp(st *store.Store) *BacklogApp {
	a := app.NewWithID("com.altenwald.backlog")
	w := a.NewWindow("Backlog")
	w.Resize(fyne.NewSize(1080, 720))

	bApp := &BacklogApp{
		fyneApp: a,
		window:  w,
		store:   st,
	}

	bApp.buildUI()

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

	// Project selector
	ba.projectSelect = widget.NewSelect([]string{}, func(selectedName string) {
		activeSlug := ba.store.GetActiveProjectSlug()
		for _, p := range ba.store.ListProjects() {
			if (p.Name == selectedName || p.Slug == selectedName) && p.Slug != activeSlug {
				_ = ba.store.SetActiveProject(p.Slug)
				break
			}
		}
	})

	newProjectBtn := widget.NewButton("📁 + Project", func() {
		ShowNewProjectDialog(ba.window, func(slug, name, desc string) {
			if _, err := ba.store.CreateProject(slug, name, desc); err == nil {
				_ = ba.store.SetActiveProject(slug)
			}
		})
	})
	newProjectBtn.Importance = widget.LowImportance

	addTaskBtn := widget.NewButton("➕ New Task", func() {
		ba.showAddTask()
	})
	addTaskBtn.Importance = widget.HighImportance

	headerLeft := container.NewHBox(
		widget.NewLabelWithStyle("Project:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWrap(fyne.NewSize(150, 36), ba.projectSelect),
		newProjectBtn,
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
				_, _ = ba.store.CompleteTask(activeSlug, taskID, done)
			})
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < 0 || id >= len(ba.displayedTasks) {
				return
			}
			item := obj.(*TaskRowItem)
			item.Bind(ba.displayedTasks[id])
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
		header,
		widget.NewSeparator(),
		ba.summaryBar.CanvasObject(),
		widget.NewSeparator(),
		ba.filterBar.CanvasObject(),
		widget.NewSeparator(),
	)

	leftPane := container.NewBorder(leftHeader, nil, nil, nil, ba.tasksList)

	// Right section (Detail Inspector pane)
	detailCallbacks := TaskDetailCallbacks{
		OnToggleDone: func(taskID string, done bool) {
			activeSlug := ba.store.GetActiveProjectSlug()
			if updated, err := ba.store.CompleteTask(activeSlug, taskID, done); err == nil {
				ba.detailView.ShowTask(*updated)
			}
		},
		OnEdit: func(task model.Task) {
			activeSlug := ba.store.GetActiveProjectSlug()
			ShowEditTaskDialog(ba.window, task, func(updated model.Task) {
				if saved, err := ba.store.UpdateTask(activeSlug, updated); err == nil {
					ba.detailView.ShowTask(*saved)
				}
			})
		},
		OnDelete: func(taskID string) {
			activeSlug := ba.store.GetActiveProjectSlug()
			_ = ba.store.DeleteTask(activeSlug, taskID)
			ba.selectedTaskID = ""
			ba.detailView.Clear()
		},
	}
	ba.detailView = NewTaskDetailView(detailCallbacks)
	rightPane := container.NewPadded(ba.detailView.Container)

	// Master-detail Split view
	split := container.NewHSplit(leftPane, rightPane)
	split.SetOffset(0.44)

	ba.window.SetContent(split)

	// Setup Tray
	ba.tray = NewTrayManager(ba.fyneApp, ba.window, ba.store, func() {
		ba.showAddTask()
	})

	ba.refreshAll()
}

func (ba *BacklogApp) showAddTask() {
	activeSlug := ba.store.GetActiveProjectSlug()
	ShowAddTaskDialog(ba.window, activeSlug, func(task model.Task) {
		if added, err := ba.store.AddTask(activeSlug, task); err == nil {
			ba.selectedTaskID = added.ID
		}
	})
}

func (ba *BacklogApp) refreshProjects() {
	projects := ba.store.ListProjects()
	activeSlug := ba.store.GetActiveProjectSlug()

	var options []string
	var selectedOption string
	for _, p := range projects {
		options = append(options, p.Name)
		if p.Slug == activeSlug {
			selectedOption = p.Name
		}
	}
	ba.projectSelect.Options = options
	if ba.projectSelect.Selected != selectedOption {
		ba.projectSelect.Selected = selectedOption
		ba.projectSelect.Refresh()
	}
}

func (ba *BacklogApp) refreshTasks() {
	activeSlug := ba.store.GetActiveProjectSlug()
	tasks, err := ba.store.ListTasks(activeSlug, ba.currentFilter)
	if err != nil {
		return
	}

	ba.displayedTasks = tasks
	ba.tasksList.Refresh()

	if len(ba.displayedTasks) == 0 {
		ba.selectedTaskID = ""
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
		ba.tasksList.Select(0)
		ba.detailView.ShowTask(ba.displayedTasks[0])
	}
}

func (ba *BacklogApp) refreshAll() {
	ba.refreshProjects()
	activeSlug := ba.store.GetActiveProjectSlug()
	sum, _ := ba.store.GetSummary(activeSlug)
	ba.summaryBar.Update(sum)
	ba.filterBar.UpdateCounts(sum)
	ba.refreshTasks()
	if ba.tray != nil {
		ba.tray.Refresh()
	}
}

func (ba *BacklogApp) listenEvents() {
	ch := ba.store.Subscribe()
	for range ch {
		ba.refreshAll()
	}
}

func (ba *BacklogApp) Run() {
	ba.window.ShowAndRun()
}
