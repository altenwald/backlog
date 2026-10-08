package network

import (
	"errors"
	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
	"sync"
)

func (r *Remote) emit(ev store.Event) {
	r.mu.Lock()
	for ch := range r.listeners {
		select {
		case ch <- ev:
		default:
		drain:
			for {
				select {
				case <-ch:
				default:
					break drain
				}
			}
			ch <- store.Event{Type: "resync"}
		}
	}
	r.mu.Unlock()
	if r.onEvent != nil {
		r.onEvent(ev)
	}
}
func (r *Remote) Watch() (<-chan store.Event, func()) {
	ch := make(chan store.Event, 32)
	r.mu.Lock()
	r.listeners[ch] = struct{}{}
	r.mu.Unlock()
	var once sync.Once
	return ch, func() { once.Do(func() { r.mu.Lock(); delete(r.listeners, ch); close(ch); r.mu.Unlock() }) }
}
func (r *Remote) Subscribe() <-chan store.Event      { ch, _ := r.Watch(); return ch }
func (r *Remote) GetActiveProjectSlug() string       { r.mu.RLock(); defer r.mu.RUnlock(); return r.active }
func (r *Remote) SetActiveProject(slug string) error { r.Select(slug); return nil }
func (r *Remote) CreateProject(string, string, string) (*model.Project, error) {
	return nil, errors.New("create projects on the owning machine")
}
func (r *Remote) DeleteProject(string) error {
	return errors.New("delete projects on the owning machine")
}
func (r *Remote) SetProjectOpen(string, bool) error {
	return errors.New("change sharing on the owning machine")
}
func (r *Remote) GetMCPUserInstructions() string { return "" }
func (r *Remote) SaveMCPUserInstructions(string) error {
	return errors.New("settings belong to the local machine")
}

var _ store.Backend = (*Remote)(nil)
var _ store.Backend = (*Hub)(nil)
var _ store.Backend = (*Commands)(nil)
