package network

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
	"github.com/coder/websocket/wsjson"
)

func host(t *testing.T) (*Service, *httptest.Server, *store.Store) {
	t.Helper()
	st, e := store.NewStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { st.Close() })
	s, e := NewService(st)
	if e != nil {
		t.Fatal(e)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/pair", s.pair)
	mux.HandleFunc("/ws", s.socket)
	ts := httptest.NewUnstartedServer(mux)
	ts.TLS = &tls.Config{Certificates: []tls.Certificate{s.cert}, MinVersion: tls.VersionTLS13}
	ts.StartTLS()
	t.Cleanup(ts.Close)
	return s, ts, st
}
func paired(t *testing.T, s *Service, ts *httptest.Server) Peer {
	t.Helper()
	otp := make(chan string, 1)
	s.OnPair = func(r PairRequest) { r.Approve(true) }
	s.OnOTP = func(_ string, code string) { otp <- code }
	ctx, stop := context.WithTimeout(context.Background(), 3*time.Second)
	defer stop()
	p, e := Pair(ctx, Peer{Address: strings.TrimPrefix(ts.URL, "https://")}, "Client", func(ctx context.Context) (string, error) {
		select {
		case code := <-otp:
			return code, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func eventually(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition did not become true")
}
func TestPairingAuthSnapshotEventsAndReconnect(t *testing.T) {
	s, ts, st := host(t)
	st.CreateProject("public", "Public", "")
	st.CreateProject("secret", "Secret", "")
	st.SetProjectOpen("public", true)
	task, _ := st.AddTask("public", model.Task{Title: "initial"})
	p := paired(t, s, ts)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	var events atomic.Int64
	r := NewRemote(ctx, p, func(store.Event) { events.Add(1) })
	defer r.Close()
	r.Select("public")
	eventually(t, func() bool { v, e := r.GetProject("public"); return e == nil && len(v.Tasks) == 1 })
	if ps := r.ListProjects(); len(ps) != 1 || ps[0].Slug != "public" {
		t.Fatalf("private metadata leaked: %+v", ps)
	}
	if _, e := r.ReloadAndGet("secret"); !errors.Is(e, store.ErrClosed) {
		t.Fatalf("private read allowed: %v", e)
	}
	_, e := r.UpdateTask("public", model.TaskUpdate{ID: task.ID, Title: "remote edit", ExpectedUpdatedAt: &task.UpdatedAt})
	if e != nil {
		t.Fatal(e)
	}
	eventually(t, func() bool { v, e := r.GetProject("public"); return e == nil && v.Tasks[0].Title == "remote edit" })
	if _, e = r.UpdateTask("public", model.TaskUpdate{ID: task.ID, Title: "stale", ExpectedUpdatedAt: &task.UpdatedAt}); !errors.Is(e, store.ErrStale) {
		t.Fatalf("stale accepted: %v", e)
	}
	r.mu.RLock()
	c := r.conn
	r.mu.RUnlock()
	c.CloseNow()
	st.AddTask("public", model.Task{Title: "while disconnected"})
	eventually(t, func() bool { v, e := r.GetProject("public"); return e == nil && len(v.Tasks) == 2 })
	st.SetProjectOpen("public", false)
	eventually(t, func() bool { _, e := r.GetProject("public"); return len(r.ListProjects()) == 0 && e != nil })
	if events.Load() == 0 {
		t.Fatal("no GUI events")
	}
	for id := range s.Trusted() {
		if e = s.Revoke(id); e != nil {
			t.Fatal(e)
		}
	}
	st.Publish(store.Event{Type: "resync"})
	eventually(t, func() bool { return !r.Connected() })
}
func (r *Remote) ReloadAndGet(slug string) (*model.Project, error) {
	if e := r.ReloadProject(slug); e != nil {
		return nil, e
	}
	return r.GetProject(slug)
}
func TestPairRejectWrongOTPAndIdentityPin(t *testing.T) {
	s, ts, _ := host(t)
	ctx, stop := context.WithTimeout(context.Background(), 3*time.Second)
	defer stop()
	s.OnPair = func(r PairRequest) { r.Approve(false) }
	peer := Peer{Address: strings.TrimPrefix(ts.URL, "https://")}
	if _, e := Pair(ctx, peer, "Client", func(context.Context) (string, error) { t.Fatal("OTP requested despite decline"); return "", nil }); e == nil {
		t.Fatal("declined pairing succeeded")
	}
	s.OnPair = func(r PairRequest) { r.Approve(true) }
	if _, e := Pair(ctx, peer, "Client", func(context.Context) (string, error) { return "wrong", nil }); e == nil {
		t.Fatal("wrong OTP accepted")
	}
	if len(s.Trusted()) != 0 {
		t.Fatal("wrong OTP created trust")
	}
	if c, _, e := dial(ctx, peer.Address, "/ws", strings.Repeat("0", 64), "bad"); e == nil {
		c.CloseNow()
		t.Fatal("wrong certificate accepted")
	}
	if c, _, e := dial(ctx, peer.Address, "/ws", s.ID, "bad"); e == nil {
		c.CloseNow()
		t.Fatal("unauthenticated socket accepted")
	}
}
func TestReceiptSurvivesLostReply(t *testing.T) {
	s, ts, st := host(t)
	st.CreateProject("p", "P", "")
	st.SetProjectOpen("p", true)
	p := paired(t, s, ts)
	project, _ := st.GetProject("p")
	ctx, stop := context.WithTimeout(context.Background(), 3*time.Second)
	defer stop()
	c, _, e := dial(ctx, p.Address, "/ws", p.ID, p.Token)
	if e != nil {
		t.Fatal(e)
	}
	defer c.CloseNow()
	a, _ := json.Marshal("p")
	b, _ := json.Marshal(model.Task{Title: "once"})
	req := Message{ID: "same-request", Method: "AddTask", Args: []json.RawMessage{a, b}, Precondition: store.Precondition{UpdatedAt: project.UpdatedAt}}
	response := func() Message {
		for {
			var m Message
			if e := wsjson.Read(ctx, c, &m); e != nil {
				t.Fatal(e)
			}
			if m.ID == req.ID {
				return m
			}
		}
	}
	if e = wsjson.Write(ctx, c, req); e != nil {
		t.Fatal(e)
	}
	if m := response(); m.Error != "" {
		t.Fatal(m.Error)
	}
	if e = wsjson.Write(ctx, c, req); e != nil {
		t.Fatal(e)
	}
	if m := response(); m.Code != "already_applied" {
		t.Fatalf("duplicate response: %+v", m)
	}
	tasks, _ := st.ListTasks("p", model.TaskFilter{})
	if len(tasks) != 1 {
		t.Fatal("request duplicated task")
	}
}
func TestOTPProofBindsCertificate(t *testing.T) {
	if proof("secret", "cert-A", "nonce") == proof("secret", "cert-B", "nonce") {
		t.Fatal("proof can be relayed across identities")
	}
}

func TestCommandsAccessUnselectedRemoteProject(t *testing.T) {
	s, ts, owner := host(t)
	owner.CreateProject("same", "Remote", "")
	owner.SetProjectOpen("same", true)
	task, _ := owner.AddTask("same", model.Task{Title: "remote"})
	peer := paired(t, s, ts)
	local, e := store.NewStore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer local.Close()
	local.CreateProject("same", "Local", "")
	svc, e := NewService(local)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := NewHub(ctx, local, svc)
	h.add(peer)
	eventually(t, func() bool { return len(h.ListProjects()) == 2 })
	c := NewCommands(h)
	ref := peer.ID + "::same"
	p, e := c.GetProject(ref)
	if e != nil || p.Slug != ref || p.Name != "Remote" {
		t.Fatalf("remote project: %+v %v", p, e)
	}
	_, e = c.UpdateTask(ref, model.TaskUpdate{ID: task.ID, Title: "CLI edit", ExpectedUpdatedAt: &task.UpdatedAt})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.UpdateTask(ref, model.TaskUpdate{ID: task.ID, Title: "stale CLI edit", ExpectedUpdatedAt: &task.UpdatedAt}); !errors.Is(e, store.ErrStale) {
		t.Fatalf("stale CLI: %v", e)
	}
	if local.GetActiveProjectSlug() != "same" || h.GetActiveProjectSlug() != "same" {
		t.Fatal("command changed GUI selection")
	}
	localProject, _ := local.GetProject("same")
	if len(localProject.Tasks) != 0 {
		t.Fatal("remote data copied to local database")
	}
	s2, e := NewService(owner)
	if e != nil {
		t.Fatal(e)
	}
	if s2.ID != s.ID || !s2.authorized(peer.Token) {
		t.Fatal("pairing did not persist across service restart")
	}
}
