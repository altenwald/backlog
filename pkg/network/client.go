package network

import (
	"context"
	"crypto/hmac"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func dial(ctx context.Context, address, path, pin, token string) (*websocket.Conn, string, error) {
	address, err := NormalizeAddress(address)
	if err != nil {
		return nil, "", err
	}
	var fp string
	tr := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, InsecureSkipVerify: true, // verified by pin, or bound to the OTP proof during bootstrap
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errors.New("missing certificate")
			}
			fp = fingerprint(cs.PeerCertificates[0].Raw)
			if pin != "" && fp != pin {
				return errors.New("remote identity changed")
			}
			return nil
		}}}
	h := http.Header{}
	if token != "" {
		h.Set("Authorization", "Bearer "+token)
	}
	c, _, e := websocket.Dial(ctx, "wss://"+address+path, &websocket.DialOptions{HTTPClient: &http.Client{Transport: tr}, HTTPHeader: h})
	if e != nil {
		tr.CloseIdleConnections()
		if errors.Is(e, http.ErrSchemeMismatch) {
			return nil, "", fmt.Errorf("%s is not a Backlog TLS port; use the LAN port (default 8486)", address)
		}
		return nil, "", e
	}
	c.SetReadLimit(64 << 20)
	return c, fp, nil
}

// Pair never sends the OTP itself. Its proof is bound to the TLS certificate and
// a fresh challenge, so an intermediary cannot relay it to a different endpoint.
func Pair(ctx context.Context, peer Peer, name string, getOTP func(context.Context) (string, error)) (Peer, error) {
	address, e := NormalizeAddress(peer.Address)
	if e != nil {
		return peer, e
	}
	peer.Address = address
	c, fp, e := dial(ctx, peer.Address, "/pair", "", "")
	if e != nil {
		return peer, e
	}
	defer c.CloseNow()
	if e = wsjson.Write(ctx, c, map[string]string{"name": name}); e != nil {
		return peer, e
	}
	var challenge map[string]string
	if e = wsjson.Read(ctx, c, &challenge); e != nil {
		return peer, e
	}
	if challenge["error"] != "" {
		return peer, errors.New(challenge["error"])
	}
	otp, e := getOTP(ctx)
	if e != nil {
		return peer, e
	}
	if e = wsjson.Write(ctx, c, map[string]string{"proof": proof(otp, fp, challenge["nonce"])}); e != nil {
		return peer, e
	}
	var response map[string]string
	if e = wsjson.Read(ctx, c, &response); e != nil {
		return peer, e
	}
	if response["error"] != "" {
		return peer, errors.New(response["error"])
	}
	if response["token"] == "" || response["id"] != fp || !hmac.Equal([]byte(response["proof"]), []byte(proof(otp, fp, "server:"+challenge["nonce"]+":"+response["token"]))) {
		return peer, errors.New("invalid pairing response")
	}
	peer.ID = fp
	peer.Token = response["token"]
	peer.Name = response["name"]
	return peer, nil
}

type Remote struct {
	listeners map[chan store.Event]struct{}

	mu        sync.RWMutex
	peer      Peer
	conn      *websocket.Conn
	pending   map[string]chan Message
	projects  []*model.Project
	snapshots map[string]*model.Project
	active    string
	connected bool
	refresh   chan struct{}
	onEvent   func(store.Event)
	cancel    context.CancelFunc
}

func NewRemote(parent context.Context, p Peer, onEvent func(store.Event)) *Remote {
	ctx, cancel := context.WithCancel(parent)
	r := &Remote{listeners: map[chan store.Event]struct{}{}, peer: p, pending: map[string]chan Message{}, snapshots: map[string]*model.Project{}, refresh: make(chan struct{}, 1), onEvent: onEvent, cancel: cancel}
	go r.run(ctx)
	return r
}
func (r *Remote) Close() {
	r.cancel()
	r.mu.Lock()
	if r.conn != nil {
		r.conn.CloseNow()
	}
	r.mu.Unlock()
}
func (r *Remote) Peer() Peer             { r.mu.RLock(); defer r.mu.RUnlock(); return r.peer }
func (r *Remote) Address(address string) { r.mu.Lock(); r.peer.Address = address; r.mu.Unlock() }
func (r *Remote) Connected() bool        { r.mu.RLock(); defer r.mu.RUnlock(); return r.connected }
func (r *Remote) signal() {
	select {
	case r.refresh <- struct{}{}:
	default:
	}
}
func (r *Remote) Select(slug string) { r.mu.Lock(); r.active = slug; r.mu.Unlock(); r.signal() }
func (r *Remote) run(ctx context.Context) {
	delay := time.Second
	for ctx.Err() == nil {
		p := r.Peer()
		attempt, stop := context.WithTimeout(ctx, 8*time.Second)
		c, _, e := dial(attempt, p.Address, "/ws", p.ID, p.Token)
		stop()
		if e == nil {
			delay = time.Second
			r.mu.Lock()
			r.conn = c
			r.connected = true
			r.mu.Unlock()
			session, cancel := context.WithCancel(ctx)
			go r.refreshLoop(session)
			r.signal()
			r.emit(store.Event{Type: "connected"})
			for {
				var m Message
				if wsjson.Read(session, c, &m) != nil {
					break
				}
				if m.Event != nil {
					r.signal()
					continue
				}
				r.mu.RLock()
				ch := r.pending[m.ID]
				r.mu.RUnlock()
				if ch != nil {
					select {
					case ch <- m:
					default:
					}
				}
			}
			cancel()
			c.CloseNow()
			r.mu.Lock()
			r.connected = false
			r.conn = nil
			for _, ch := range r.pending {
				select {
				case ch <- Message{Error: "connection lost; reload to check whether the change was applied", Code: "disconnected"}:
				default:
				}
			}
			r.mu.Unlock()
			r.emit(store.Event{Type: "disconnected"})
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		if delay < 16*time.Second {
			delay *= 2
		}
	}
}
func (r *Remote) request(ctx context.Context, method string, args []any, pre store.Precondition, out any) error {
	req := Message{ID: randomToken(), Method: method, Precondition: pre}
	for _, arg := range args {
		b, e := json.Marshal(arg)
		if e != nil {
			return e
		}
		req.Args = append(req.Args, b)
	}
	ch := make(chan Message, 1)
	r.mu.Lock()
	c := r.conn
	if c == nil || !r.connected {
		r.mu.Unlock()
		return errors.New("remote machine disconnected")
	}
	r.pending[req.ID] = ch
	r.mu.Unlock()
	defer func() { r.mu.Lock(); delete(r.pending, req.ID); r.mu.Unlock() }()
	if e := wsjson.Write(ctx, c, req); e != nil {
		return e
	}
	select {
	case m := <-ch:
		if m.Code == "stale" {
			r.signal()
			return store.ErrStale
		}
		if m.Code == "closed" {
			r.signal()
			return store.ErrClosed
		}
		if m.Error != "" {
			return errors.New(m.Error)
		}
		if out != nil {
			return json.Unmarshal(m.Data, out)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("request interrupted; reload before retrying: %w", ctx.Err())
	}
}
func (r *Remote) refreshLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.refresh:
			t, stop := context.WithTimeout(ctx, 15*time.Second)
			var ps []*model.Project
			e := r.request(t, "ListProjects", nil, store.Precondition{}, &ps)
			if e == nil {
				r.mu.Lock()
				r.projects = ps
				active := r.active
				visible := false
				for _, p := range ps {
					if p.Slug == active {
						visible = true
					}
				}
				if !visible {
					delete(r.snapshots, active)
				}
				r.mu.Unlock()
				if visible {
					var p model.Project
					e = r.request(t, "Snapshot", []any{active}, store.Precondition{}, &p)
					if e == nil {
						r.mu.Lock()
						r.snapshots = map[string]*model.Project{active: &p}
						r.mu.Unlock()
					} else {
						r.mu.Lock()
						delete(r.snapshots, active)
						r.mu.Unlock()
					}
				}
				r.emit(store.Event{Type: "resync"})
			}
			stop()
		}
	}
}
func (r *Remote) snapshot(slug string) (*model.Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.connected {
		return nil, errors.New("remote machine disconnected")
	}
	p := r.snapshots[slug]
	if p == nil {
		return nil, errors.New("project is loading or no longer shared")
	}
	// Callers receive their own objects; UI edits must never mutate the cache.
	b, _ := json.Marshal(p)
	var copy model.Project
	_ = json.Unmarshal(b, &copy)
	return &copy, nil
}
func (r *Remote) call(method, slug, id string, args []any, expected *time.Time, out any) error {
	p, e := r.snapshot(slug)
	if e != nil {
		return e
	}
	pre := store.Precondition{Slug: slug, UpdatedAt: p.UpdatedAt}
	if id != "" {
		for _, t := range p.Tasks {
			if t.ID == id {
				pre.UpdatedAt = t.UpdatedAt
			}
		}
		if method == "UpdateSpecSection" || method == "DeleteSpecSection" {
			for _, s := range p.Spec {
				if s.ID == id {
					pre.UpdatedAt = s.UpdatedAt
				}
			}
		}
	}
	if expected != nil {
		pre.UpdatedAt = *expected
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	e = r.request(ctx, method, args, pre, out)
	r.signal()
	return e
}

func (r *Remote) ReloadProject(slug string) error {
	ctx, stop := requestContext()
	defer stop()
	var p model.Project
	if e := r.request(ctx, "Snapshot", []any{slug}, store.Precondition{}, &p); e != nil {
		return e
	}
	r.mu.Lock()
	r.snapshots[slug] = &p
	r.mu.Unlock()
	r.emit(store.Event{Type: "resync", ProjectSlug: slug})
	return nil
}
