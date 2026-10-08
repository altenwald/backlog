package ui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/network"
	"github.com/altenwald/backlog/pkg/store"
)

func (ba *BacklogApp) configureNetwork() {
	h, ok := ba.store.(*network.Hub)
	if !ok {
		return
	}
	h.Service.OnPair = func(req network.PairRequest) {
		fyne.Do(func() {
			// An independent window keeps the request visible when the main app is in the tray.
			w := ba.fyneApp.NewWindow("Backlog connection request")
			answered := false
			answer := func(yes bool) {
				if answered {
					return
				}
				answered = true
				req.Approve(yes)
				w.Close()
			}
			w.SetCloseIntercept(func() {
				if !answered {
					answered = true
					req.Approve(false)
				}
				w.SetCloseIntercept(nil)
				w.Close()
			})
			w.SetContent(container.NewVBox(widget.NewLabel(fmt.Sprintf("Accept a connection from %s?\n%s\nThe name is supplied by the requesting machine.", req.Name, req.Address)), container.NewHBox(widget.NewButton("Decline", func() { answer(false) }), widget.NewButton("Accept", func() { answer(true) }))))
			w.Resize(fyne.NewSize(480, 180))
			w.Show()
		})
	}
	h.Service.OnOTP = func(name, otp string) {
		fyne.Do(func() {
			w := ba.fyneApp.NewWindow("Backlog pairing code")
			entry := widget.NewEntry()
			entry.SetText(otp)
			w.SetContent(container.NewVBox(widget.NewLabel("Enter this one-time code on "+name+".\nIt expires with the pairing request after two minutes."), entry, widget.NewButton("Copy", func() { w.Clipboard().SetContent(otp) }), widget.NewButton("Close", w.Close)))
			w.Resize(fyne.NewSize(520, 190))
			w.Show()
		})
	}
}
func (ba *BacklogApp) showNetwork() {
	h, ok := ba.store.(*network.Hub)
	if !ok {
		dialog.ShowInformation("Network", "Network service is not enabled.", ba.window)
		return
	}
	rows := container.NewVBox()
	var refresh func()
	connect := func(p network.Peer) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		waiting := dialog.NewCustom("Connecting", "Cancel", widget.NewLabel("Waiting for approval on "+p.Name+"…"), ba.window)
		waiting.SetOnClosed(cancel)
		waiting.Show()
		go func() {
			e := h.Connect(ctx, p, func(ctx context.Context) (string, error) {
				result := make(chan string, 1)
				fyne.Do(func() {
					entry := widget.NewEntry()
					entry.SetPlaceHolder("One-time code shown on the other machine")
					d := dialog.NewForm("Pair with "+p.Name, "Connect", "Cancel", []*widget.FormItem{widget.NewFormItem("OTP", entry)}, func(ok bool) {
						if !ok {
							cancel()
							return
						}
						select {
						case result <- entry.Text:
						default:
						}
					}, ba.window)
					d.Resize(fyne.NewSize(560, 160))
					d.Show()
				})
				select {
				case otp := <-result:
					return otp, nil
				case <-ctx.Done():
					return "", ctx.Err()
				}
			})
			fyne.Do(func() {
				waiting.Hide()
				if e != nil {
					dialog.ShowError(e, ba.window)
				} else {
					refresh()
				}
			})
			cancel()
		}()
	}
	refresh = func() {
		rows.Objects = nil
		for _, p := range h.Peers() {
			peer := p
			status := "Discovered"
			if p.Token != "" {
				status = "Paired"
			}
			action := widget.NewButton("Connect", func() { connect(peer) })
			if p.Token != "" {
				action = widget.NewButton("Forget", func() {
					if e := h.Forget(peer.ID); e != nil {
						dialog.ShowError(e, ba.window)
					}
					refresh()
				})
			}
			rows.Add(container.NewBorder(nil, nil, nil, action, widget.NewLabel(p.Name+" · "+status+"\n"+p.Address)))
		}
		for id, name := range h.Service.Trusted() {
			key := id
			rows.Add(container.NewBorder(nil, nil, nil, widget.NewButton("Revoke access", func() {
				if e := h.Service.Revoke(key); e != nil {
					dialog.ShowError(e, ba.window)
				}
				refresh()
			}), widget.NewLabel("Allowed incoming: "+name)))
		}
		if len(rows.Objects) == 0 {
			rows.Add(widget.NewLabel("No machines discovered yet. Refresh or enter an address."))
		}
		rows.Refresh()
	}
	refresh()
	address := widget.NewEntry()
	address.SetPlaceHolder("IP address:port (default port 8486)")
	manual := widget.NewButton("Connect by address", func() {
		a := strings.TrimSpace(address.Text)
		if a == "" {
			return
		}
		if !strings.Contains(a, ":") {
			a += ":8486"
		}
		connect(network.Peer{Name: a, Address: a})
	})
	content := container.NewBorder(widget.NewLabel("Nearby Backlog machines · "+h.Service.Name), container.NewVBox(widget.NewButton("Refresh", refresh), address, manual), nil, nil, container.NewVScroll(rows))
	d := dialog.NewCustom("Network", "Close", content, ba.window)
	d.Resize(fyne.NewSize(660, 440))
	d.Show()
}
func (ba *BacklogApp) showSharing() {
	slug := ba.store.GetActiveProjectSlug()
	if slug == "" {
		return
	}
	if strings.Contains(slug, "::") {
		dialog.ShowInformation("Sharing", "Change sharing on the machine that owns this project.", ba.window)
		return
	}
	p, e := ba.store.GetProject(slug)
	if e != nil {
		dialog.ShowError(e, ba.window)
		return
	}
	open := widget.NewCheck("Open to paired machines (read and write)", nil)
	open.SetChecked(p.Open)
	d := dialog.NewCustomConfirm("Project sharing", "Save", "Cancel", container.NewVBox(widget.NewLabel(p.Name), open), func(ok bool) {
		if ok {
			ba.runMutation(func() error { return ba.store.SetProjectOpen(slug, open.Checked) })
		}
	}, ba.window)
	d.Show()
}
func (ba *BacklogApp) runMutation(fn func() error) {
	go func() {
		e := fn()
		if e != nil {
			fyne.Do(func() { dialog.ShowError(e, ba.window) })
		}
	}()
}
func (ba *BacklogApp) editTask(slug string, task model.Task) {
	ShowEditTaskDialog(ba.window, task, func(updated model.Task) {
		go func() {
			_, e := ba.store.UpdateTask(slug, model.TaskUpdate{ID: updated.ID, Title: updated.Title, Description: &updated.Description, ParentID: updated.ParentID, DependsOn: updated.DependsOn, Size: updated.Size, Tier: updated.Tier, Resolution: updated.Resolution, Assignee: updated.Assignee, ExpectedUpdatedAt: &task.UpdatedAt})
			if e == nil {
				return
			}
			fyne.Do(func() {
				if errors.Is(e, store.ErrStale) {
					dialog.ShowConfirm("Task changed", "Your edit was not saved. Read the current task before applying your draft again?", func(ok bool) {
						if !ok {
							ba.editTask(slug, updated)
							return
						}
						go func() {
							if r, ok := ba.store.(interface{ ReloadProject(string) error }); ok {
								if e := r.ReloadProject(slug); e != nil {
									fyne.Do(func() { dialog.ShowError(e, ba.window); ba.editTask(slug, updated) })
									return
								}
							}
							p, err := ba.store.GetProject(slug)
							fyne.Do(func() {
								if err != nil {
									dialog.ShowError(err, ba.window)
									return
								}
								for _, current := range p.Tasks {
									if current.ID == task.ID {
										msg := fmt.Sprintf("Current title: %s\n\nCurrent description:\n%s\n\nAssignee: %s · Size: %s · Tier: %d\nParent: %s · Dependencies: %v\nResolution: %s\n\nReapply only the fields you changed to this version?", current.Title, current.Description, current.Assignee, current.Size, current.Tier, current.ParentID, current.DependsOn, current.Resolution)
										dialog.ShowConfirm("Current task", ""+msg, func(again bool) {
											if again {
												ba.editTask(slug, reapplyTaskDraft(task, updated, current))
											} else {
												ba.editTask(slug, updated)
											}
										}, ba.window)
										return
									}
								}
								dialog.ShowError(errors.New("task no longer exists; your draft remains available to copy"), ba.window)
								ba.editTask(slug, updated)
							})
						}()
					}, ba.window)
				} else {
					dialog.ShowError(e, ba.window)
					ba.editTask(slug, updated)
				}
			})
		}()
	})
}

func (ba *BacklogApp) toggleTask(slug, id string, done, deprecated *bool) {
	for _, t := range ba.displayedTasks {
		if t.ID == id {
			version := t.UpdatedAt
			ba.runMutation(func() error {
				_, e := ba.store.UpdateTask(slug, model.TaskUpdate{ID: id, Done: done, Deprecated: deprecated, ExpectedUpdatedAt: &version})
				return e
			})
			return
		}
	}
}

// A stale edit rebases only the fields actually changed by this user, after the
// user has read and accepted the current state. Unedited fields stay current.
func reapplyTaskDraft(base, draft, current model.Task) model.Task {
	if draft.Title != base.Title {
		current.Title = draft.Title
	}
	if draft.Description != base.Description {
		current.Description = draft.Description
	}
	if draft.ParentID != base.ParentID {
		current.ParentID = draft.ParentID
	}
	if !slices.Equal(draft.DependsOn, base.DependsOn) {
		current.DependsOn = append([]string(nil), draft.DependsOn...)
	}
	if draft.Size != base.Size {
		current.Size = draft.Size
	}
	if draft.Tier != base.Tier {
		current.Tier = draft.Tier
	}
	if draft.Resolution != base.Resolution {
		current.Resolution = draft.Resolution
	}
	if draft.Assignee != base.Assignee {
		current.Assignee = draft.Assignee
	}
	return current
}
