package network

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
)

// Hub routes a machine/project reference. Selection and credentials stay local.
type Hub struct {
	store.Backend
	Local      *store.Store
	Service    *Service
	ctx        context.Context
	mu         sync.RWMutex
	remotes    map[string]*Remote
	discovered map[string]Peer
	active     string
}

func NewHub(ctx context.Context, local *store.Store, service *Service) *Hub {
	h := &Hub{Backend: local, Local: local, Service: service, ctx: ctx, remotes: map[string]*Remote{}, discovered: map[string]Peer{}}
	var ps []Peer
	_ = json.Unmarshal([]byte(local.LocalSetting("network.peers")), &ps)
	for _, p := range ps {
		h.add(p)
	}
	service.OnPeer = func(p Peer) {
		h.mu.Lock()
		h.discovered[p.ID] = p
		r := h.remotes[p.ID]
		h.mu.Unlock()
		if r != nil {
			r.Address(p.Address)
		}
	}
	return h
}
func (h *Hub) add(p Peer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if old := h.remotes[p.ID]; old != nil {
		old.Close()
	}
	h.remotes[p.ID] = NewRemote(h.ctx, p, func(e store.Event) {
		e.Source = p.ID
		if e.ProjectSlug != "" {
			e.ProjectSlug = p.ID + "::" + e.ProjectSlug
		}
		h.Local.Publish(e)
	})
}
func (h *Hub) save() error {
	h.mu.RLock()
	var ps []Peer
	for _, r := range h.remotes {
		ps = append(ps, r.Peer())
	}
	h.mu.RUnlock()
	b, _ := json.Marshal(ps)
	return h.Local.SetLocalSetting("network.peers", string(b))
}
func (h *Hub) Connect(ctx context.Context, p Peer, getOTP func(context.Context) (string, error)) error {
	p, e := Pair(ctx, p, h.Service.Name, getOTP)
	if e != nil {
		return e
	}
	h.add(p)
	return h.save()
}
func (h *Hub) Forget(id string) error {
	h.mu.Lock()
	if r := h.remotes[id]; r != nil {
		r.Close()
		delete(h.remotes, id)
	}
	h.mu.Unlock()
	h.Local.Publish(store.Event{Type: "resync", Source: id})
	return h.save()
}
func (h *Hub) Peers() []Peer {
	h.mu.RLock()
	defer h.mu.RUnlock()
	m := map[string]Peer{}
	for k, v := range h.discovered {
		m[k] = v
	}
	for k, r := range h.remotes {
		m[k] = r.Peer()
	}
	var out []Peer
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
func (h *Hub) GetActiveProjectSlug() string {
	h.mu.RLock()
	v := h.active
	h.mu.RUnlock()
	if v != "" {
		return v
	}
	return h.Local.GetActiveProjectSlug()
}
func (h *Hub) SetActiveProject(slug string) error {
	b, raw, e := h.resolve(slug)
	if e != nil {
		return e
	}
	if r, ok := b.(*Remote); ok {
		r.Select(raw)
	} else {
		if e = h.Local.SetActiveProject(raw); e != nil {
			return e
		}
	}
	h.mu.Lock()
	h.active = slug
	h.mu.Unlock()
	h.Local.Publish(store.Event{Type: store.EventProjectSelected, ProjectSlug: slug, Source: "selection"})
	return nil
}
func (h *Hub) resolve(slug string) (store.Backend, string, error) {
	if slug == "" {
		slug = h.GetActiveProjectSlug()
	}
	id, raw, remote := strings.Cut(slug, "::")
	if !remote {
		return h.Local, slug, nil
	}
	h.mu.RLock()
	r := h.remotes[id]
	h.mu.RUnlock()
	if r == nil {
		return nil, "", errors.New("remote machine not paired")
	}
	return r, raw, nil
}
func (h *Hub) ListProjects() []*model.Project {
	out := h.Local.ListProjects()
	h.mu.RLock()
	var rs []*Remote
	for _, r := range h.remotes {
		rs = append(rs, r)
	}
	h.mu.RUnlock()
	sort.Slice(rs, func(i, j int) bool { return rs[i].Peer().Name < rs[j].Peer().Name })
	for _, r := range rs {
		peer := r.Peer()
		for _, p := range r.ListProjects() {
			p.Slug = peer.ID + "::" + p.Slug
			p.Machine = peer.Name
			p.RemoteID = peer.ID
			p.Disconnected = !r.Connected()
			out = append(out, p)
		}
	}
	return out
}
func (h *Hub) SetProjectOpen(slug string, open bool) error {
	if strings.Contains(slug, "::") {
		return errors.New("sharing can only be changed on the owning machine")
	}
	return h.Local.SetProjectOpen(slug, open)
}
func (h *Hub) DeleteProject(slug string) error {
	if strings.Contains(slug, "::") {
		return errors.New("delete the project on its owning machine")
	}
	h.mu.Lock()
	if h.active == slug {
		h.active = ""
	}
	h.mu.Unlock()
	return h.Local.DeleteProject(slug)
}

func (h *Hub) ReloadProject(slug string) error {
	b, raw, e := h.resolve(slug)
	if e != nil {
		return e
	}
	if r, ok := b.(*Remote); ok {
		return r.ReloadProject(raw)
	}
	_, e = b.GetProject(raw)
	return e
}
