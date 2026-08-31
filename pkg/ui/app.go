package ui

import (
	"fmt"
	"strings"

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
	tasksList     *fyne.Container
	projectSelect *widget.Select
	currentFilter model.TaskFilter
}

func NewBacklogApp(st *store.Store) *BacklogApp {
	a := app.NewWithID("com.altenwald.backlog")
	w := a.NewWindow("Backlog")
	w.Resize(fyne.NewSize(920, 720))

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
	ba.summaryBar = NewSummaryBar()

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
		ba.projectSelect,
		newProjectBtn,
	)

	header := container.NewBorder(nil, nil, headerLeft, addTaskBtn)

	// Tasks container
	ba.tasksList = container.NewVBox()
	scroll := container.NewVScroll(container.NewPadded(ba.tasksList))

	topSection := container.NewVBox(
		header,
		widget.NewSeparator(),
		ba.summaryBar.CanvasObject(),
		ba.filterBar.CanvasObject(),
		widget.NewSeparator(),
	)

	content := container.NewBorder(topSection, nil, nil, nil, scroll)
	ba.window.SetContent(content)

	// Setup Tray
	ba.tray = NewTrayManager(ba.fyneApp, ba.window, ba.store, func() {
		ba.showAddTask()
	})

	ba.refreshAll()
}

func (ba *BacklogApp) showAddTask() {
	activeSlug := ba.store.GetActiveProjectSlug()
	ShowAddTaskDialog(ba.window, activeSlug, func(task model.Task) {
		_, _ = ba.store.AddTask(activeSlug, task)
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

	ba.tasksList.Objects = nil

	if len(tasks) == 0 {
		emptyMsg := widget.NewLabelWithStyle("No tasks match the current filter.", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
		ba.tasksList.Add(container.NewPadded(emptyMsg))
		ba.tasksList.Refresh()
		return
	}

	// Group tasks by category
	groupsMap := make(map[string][]model.Task)
	var groupOrder []string

	for _, t := range tasks {
		grp := t.Group
		if grp == "" {
			grp = "General"
		}
		if _, exists := groupsMap[grp]; !exists {
			groupOrder = append(groupOrder, grp)
		}
		groupsMap[grp] = append(groupsMap[grp], t)
	}

	callbacks := TaskCardCallbacks{
		OnToggleDone: func(taskID string, done bool) {
			_, _ = ba.store.CompleteTask(activeSlug, taskID, done)
		},
		OnEdit: func(task model.Task) {
			ShowEditTaskDialog(ba.window, task, func(updated model.Task) {
				_, _ = ba.store.UpdateTask(activeSlug, updated)
			})
		},
		OnDelete: func(taskID string) {
			_ = ba.store.DeleteTask(activeSlug, taskID)
		},
	}

	for _, grp := range groupOrder {
		groupTasks := groupsMap[grp]
		openInGroup := 0
		for _, gt := range groupTasks {
			if !gt.Done {
				openInGroup++
			}
		}

		groupHeader := widget.NewLabelWithStyle(
			fmt.Sprintf("▼  %s (%d open / %d total)", strings.ToUpper(grp), openInGroup, len(groupTasks)),
			fyne.TextAlignLeading,
			fyne.TextStyle{Bold: true},
		)
		ba.tasksList.Add(groupHeader)

		for _, t := range groupTasks {
			ba.tasksList.Add(NewTaskRow(t, callbacks))
		}
		ba.tasksList.Add(widget.NewSeparator())
	}

	ba.tasksList.Refresh()
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
		// Run UI updates on main thread
		ba.refreshAll()
	}
}

func (ba *BacklogApp) Run() {
	ba.window.ShowAndRun()
}
