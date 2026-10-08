// Package network implements paired LAN access. Databases remain on their owner.
package network

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/altenwald/backlog/pkg/model"
	"github.com/altenwald/backlog/pkg/store"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/grandcat/zeroconf"
)

const discoveryService = "_backlog._tcp"

type Peer struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Token   string `json:"token,omitempty"`
}
type Message struct {
	ID           string             `json:"id,omitempty"`
	Method       string             `json:"method,omitempty"`
	Args         []json.RawMessage  `json:"args,omitempty"`
	Precondition store.Precondition `json:"precondition,omitempty"`
	Data         json.RawMessage    `json:"data,omitempty"`
	Error        string             `json:"error,omitempty"`
	Code         string             `json:"code,omitempty"`
	Event        *store.Event       `json:"event,omitempty"`
}
type PairRequest struct {
	Name    string
	Address string
	Approve func(bool)
	ShowOTP func(string)
}
type Service struct {
	Store            *store.Store
	Name, ID         string
	OnPair           func(PairRequest)
	OnOTP            func(string, string)
	OnPeer           func(Peer)
	mu               sync.Mutex
	trusted          map[string]string // token hash -> requesting machine name
	server           *http.Server
	advertisement    *zeroconf.Server
	cert             tls.Certificate
	pending          chan struct{}
	refreshDiscovery chan struct{}
	listenAddress    string
	discoveryError   string
}

func randomToken() string {
	var b [16]byte
	_, e := rand.Read(b[:])
	if e != nil {
		panic(e)
	}
	return hex.EncodeToString(b[:])
}
func fingerprint(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func proof(otp, fp, nonce string) string {
	m := hmac.New(sha256.New, []byte(strings.ToLower(strings.ReplaceAll(strings.TrimSpace(otp), "-", ""))))
	m.Write([]byte("backlog-pair-v1:" + fp + ":" + nonce))
	return hex.EncodeToString(m.Sum(nil))
}
func NewService(st *store.Store) (*Service, error) {
	name, _ := os.Hostname()
	s := &Service{Store: st, Name: name, trusted: map[string]string{}, pending: make(chan struct{}, 3), refreshDiscovery: make(chan struct{}, 1)}
	_ = json.Unmarshal([]byte(st.LocalSetting("network.trusted")), &s.trusted)
	certPath := filepath.Join(st.GetDataDir(), "network-cert.pem")
	keyPath := filepath.Join(st.GetDataDir(), "network-key.pem")
	if _, e := os.Stat(certPath); os.IsNotExist(e) {
		key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			return nil, e
		}
		serial, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		if e != nil {
			return nil, e
		}
		template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "Backlog LAN"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(10, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
		der, e := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
		if e != nil {
			return nil, e
		}
		kb, e := x509.MarshalECPrivateKey(key)
		if e != nil {
			return nil, e
		}
		if e = os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb}), 0600); e != nil {
			return nil, e
		}
		if e = os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); e != nil {
			return nil, e
		}
	}
	cert, e := tls.LoadX509KeyPair(certPath, keyPath)
	if e != nil {
		return nil, e
	}
	s.cert = cert
	s.ID = fingerprint(cert.Certificate[0])
	return s, nil
}
func (s *Service) Start(ctx context.Context, port int) error {
	ln, err := tls.Listen("tcp", fmt.Sprintf(":%d", port), &tls.Config{Certificates: []tls.Certificate{s.cert}, MinVersion: tls.VersionTLS13})
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/pair", s.pair)
	mux.HandleFunc("/ws", s.socket)
	s.server = &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }}
	s.mu.Lock()
	s.listenAddress = ln.Addr().String()
	s.mu.Unlock()
	log.Printf("[Backlog LAN] TLS listening on %s", ln.Addr())
	go func() { <-ctx.Done(); s.server.Close() }()
	go func() {
		if err := s.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[Backlog LAN] listener: %v", err)
		}
	}()
	go s.discover(ctx, ln.Addr().(*net.TCPAddr).Port)
	return nil
}

func (s *Service) ListenAddress() string  { s.mu.Lock(); defer s.mu.Unlock(); return s.listenAddress }
func (s *Service) DiscoveryError() string { s.mu.Lock(); defer s.mu.Unlock(); return s.discoveryError }
func (s *Service) setDiscoveryError(err error) {
	message := ""
	if err != nil {
		message = err.Error()
	}
	s.mu.Lock()
	changed := s.discoveryError != message
	s.discoveryError = message
	s.mu.Unlock()
	if changed && err != nil {
		log.Printf("[Backlog LAN] discovery: %v", err)
	}
}

// RefreshDiscovery interrupts the current scan or retry delay and starts a new query.
func (s *Service) RefreshDiscovery() {
	select {
	case s.refreshDiscovery <- struct{}{}:
	default:
	}
}

func (s *Service) discover(ctx context.Context, port int) {
	defer func() {
		if s.advertisement != nil {
			s.advertisement.Shutdown()
		}
	}()
	for ctx.Err() == nil {
		var advertiseErr error
		if s.advertisement == nil {
			s.advertisement, advertiseErr = zeroconf.Register(s.ID[:16], discoveryService, "local.", port, []string{"id=" + s.ID, "name=" + s.Name, "v=1"}, nil)
		}
		resolver, err := zeroconf.NewResolver(nil)
		if err == nil {
			scan, cancel := context.WithTimeout(ctx, 8*time.Second)
			entries := make(chan *zeroconf.ServiceEntry, 16)
			err = resolver.Browse(scan, discoveryService, "local.", entries)
			s.setDiscoveryError(errors.Join(advertiseErr, err))
			if err == nil {
			scanLoop:
				for {
					select {
					case entry, ok := <-entries:
						if !ok {
							break scanLoop
						}
						if p, ok := discoveredPeer(entry); ok && p.ID != s.ID && s.OnPeer != nil {
							s.OnPeer(p)
						}
					case <-s.refreshDiscovery:
						break scanLoop
					case <-scan.Done():
						break scanLoop
					}
				}
			}
			cancel()
			// The resolver closes entries on cancellation. Drain so its producer can exit.
			for range entries {
			}
		} else {
			s.setDiscoveryError(errors.Join(advertiseErr, err))
		}
		select {
		case <-ctx.Done():
			return
		case <-s.refreshDiscovery:
		case <-time.After(time.Second):
		}
	}
}

func discoveredPeer(entry *zeroconf.ServiceEntry) (Peer, bool) {
	p := Peer{Name: entry.Instance}
	for _, v := range entry.Text {
		if strings.HasPrefix(v, "id=") {
			p.ID = strings.TrimPrefix(v, "id=")
		}
		if strings.HasPrefix(v, "name=") {
			p.Name = strings.TrimPrefix(v, "name=")
		}
	}
	// Prefer usable IPv4, then global/ULA IPv6. A link-local IPv6 address without
	// an interface scope cannot be dialled; let the OS resolve the mDNS hostname.
	for _, ip := range entry.AddrIPv4 {
		if !ip.IsLoopback() && !ip.IsUnspecified() {
			p.Address = net.JoinHostPort(ip.String(), fmt.Sprint(entry.Port))
			break
		}
	}
	if p.Address == "" {
		for _, ip := range entry.AddrIPv6 {
			if !ip.IsLinkLocalUnicast() && !ip.IsLoopback() && !ip.IsUnspecified() {
				p.Address = net.JoinHostPort(ip.String(), fmt.Sprint(entry.Port))
				break
			}
		}
	}
	if p.Address == "" && entry.HostName != "" {
		p.Address = net.JoinHostPort(strings.TrimSuffix(entry.HostName, "."), fmt.Sprint(entry.Port))
	}
	return p, p.ID != "" && p.Address != "" && entry.Port > 0 && entry.Port <= 65535
}
func (s *Service) pair(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != "" {
		http.Error(w, "native clients only", 403)
		return
	}
	select {
	case s.pending <- struct{}{}:
		defer func() { <-s.pending }()
	default:
		http.Error(w, "pairing busy", 429)
		return
	}
	c, e := websocket.Accept(w, r, nil)
	if e != nil {
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(4096)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	var hello struct {
		Name string `json:"name"`
	}
	if wsjson.Read(ctx, c, &hello) != nil {
		return
	}
	if len(hello.Name) > 128 {
		return
	}
	approval := make(chan bool, 1)
	if s.OnPair == nil {
		return
	}
	s.OnPair(PairRequest{Name: hello.Name, Address: r.RemoteAddr, Approve: func(ok bool) {
		select {
		case approval <- ok:
		default:
		}
	}})
	select {
	case ok := <-approval:
		if !ok {
			_ = wsjson.Write(ctx, c, map[string]string{"error": "connection declined"})
			return
		}
	case <-ctx.Done():
		return
	}
	otp := randomToken()
	nonce := randomToken()
	if s.OnOTP != nil {
		s.OnOTP(hello.Name, otp)
	}
	if wsjson.Write(ctx, c, map[string]string{"nonce": nonce, "name": s.Name}) != nil {
		return
	}
	for attempt := 0; attempt < 5; attempt++ {
		var auth struct {
			Proof string `json:"proof"`
		}
		if wsjson.Read(ctx, c, &auth) != nil {
			return
		}
		if !hmac.Equal([]byte(auth.Proof), []byte(proof(otp, s.ID, nonce))) {
			_ = wsjson.Write(ctx, c, map[string]string{"error": "invalid OTP"})
			continue
		}
		token := randomToken() + randomToken()
		s.mu.Lock()
		s.trusted[fingerprint([]byte(token))] = hello.Name
		data, _ := json.Marshal(s.trusted)
		e = s.Store.SetLocalSetting("network.trusted", string(data))
		s.mu.Unlock()
		if e != nil {
			return
		}
		_ = wsjson.Write(ctx, c, map[string]string{"token": token, "id": s.ID, "name": s.Name, "proof": proof(otp, s.ID, "server:"+nonce+":"+token)})
		return
	}
}
func (s *Service) authorized(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.trusted[fingerprint([]byte(token))]
	return ok && token != ""
}
func (s *Service) Trusted() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]string{}
	for k, v := range s.trusted {
		out[k] = v
	}
	return out
}
func (s *Service) Revoke(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.trusted, id)
	data, _ := json.Marshal(s.trusted)
	return s.Store.SetLocalSetting("network.trusted", string(data))
}
func (s *Service) socket(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if r.Header.Get("Origin") != "" || !s.authorized(token) {
		http.Error(w, "unauthorized", 401)
		return
	}
	c, e := websocket.Accept(w, r, nil)
	if e != nil {
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(8 << 20)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	var writeMu sync.Mutex
	send := func(m Message) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		t, stop := context.WithTimeout(ctx, 10*time.Second)
		defer stop()
		return wsjson.Write(t, c, m)
	}
	events, unwatch := s.Store.Watch()
	defer unwatch()
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-events:
				if !ok {
					return
				}
				if ev.Source != "" {
					continue
				}
				if !s.authorized(token) {
					c.CloseNow()
					return
				}
				if send(Message{Event: &store.Event{Type: "resync"}}) != nil {
					c.CloseNow()
					return
				}
			case <-ticker.C:
				if !s.authorized(token) {
					c.CloseNow()
					return
				}
				p, stop := context.WithTimeout(ctx, 5*time.Second)
				e := c.Ping(p)
				stop()
				if e != nil {
					c.CloseNow()
					return
				}
			}
		}
	}()
	for {
		var req Message
		if wsjson.Read(ctx, c, &req) != nil {
			return
		}
		if !s.authorized(token) {
			return
		}
		var value any
		var err error
		switch req.Method {
		case "ListProjects":
			var ps []*model.Project
			metadata, e := s.Store.ListProjectsChecked()
			if e != nil {
				err = e
				break
			}
			for _, p := range metadata {
				if p.Open {
					ps = append(ps, p)
				}
			}
			value = ps
		case "Snapshot":
			var slug string
			if len(req.Args) != 1 {
				err = errors.New("invalid arguments")
			} else if err = json.Unmarshal(req.Args[0], &slug); err == nil {
				value, err = s.Store.RemoteSnapshot(slug)
			}
		default:
			if req.ID == "" || len(req.ID) > 128 {
				err = errors.New("invalid request ID")
				break
			}
			req.Precondition.RequestID = fingerprint([]byte(token)) + ":" + req.ID
			value, err = s.Store.RemoteCall(req.Method, req.Args, req.Precondition)
		}
		resp := Message{ID: req.ID}
		if err != nil {
			resp.Error = err.Error()
			if errors.Is(err, store.ErrAlreadyApplied) {
				resp.Code = "already_applied"
			}
			if errors.Is(err, store.ErrStale) {
				resp.Code = "stale"
			}
			if errors.Is(err, store.ErrClosed) {
				resp.Code = "closed"
			}
		} else {
			resp.Data, _ = json.Marshal(value)
		}
		if send(resp) != nil {
			return
		}
	}
}
