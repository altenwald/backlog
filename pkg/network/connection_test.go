package network

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/altenwald/backlog/pkg/store"
	"github.com/grandcat/zeroconf"
)

func TestNormalizeAddress(t *testing.T) {
	for input, want := range map[string]string{
		" computer.local ":             "computer.local:8486",
		"192.168.2.15:8486":            "192.168.2.15:8486",
		"https://computer.local:8486/": "computer.local:8486",
		"http://computer.local:8486":   "computer.local:8486",
		"wss://computer.local":         "computer.local:8486",
		"::1":                          "[::1]:8486",
		"[fe80::1%en0]:8486":           "[fe80::1%en0]:8486",
	} {
		got, err := NormalizeAddress(input)
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"", "https://", "host:abc", "host:70000", "https://user:pass@host", "https://host/pair", "https://host?secret=x", "host name"} {
		if _, err := NormalizeAddress(input); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}

// Exercise the production listener, rather than giving the handlers TLS via httptest.
func TestServiceStartUsesTLSAndPairsWithPastedURL(t *testing.T) {
	st, err := store.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s, err := NewService(st)
	if err != nil {
		t.Fatal(err)
	}
	otp := make(chan string, 1)
	s.OnPair = func(r PairRequest) { r.Approve(true) }
	s.OnOTP = func(_ string, code string) { otp <- code }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.Start(ctx, 0); err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(s.ListenAddress())
	address := net.JoinHostPort("127.0.0.1", port)
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("http://" + address + "/pair")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 400 || !strings.Contains(string(body), "HTTPS") {
		t.Fatalf("plaintext was not rejected: %d %s", response.StatusCode, body)
	}
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS13}}
	defer tr.CloseIdleConnections()
	client.Transport = tr
	response, err = client.Get("https://" + address + "/pair")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.TLS == nil || response.TLS.Version != tls.VersionTLS13 || response.StatusCode != http.StatusUpgradeRequired {
		t.Fatal("TLS WebSocket endpoint not available")
	}
	pairCtx, stop := context.WithTimeout(ctx, 3*time.Second)
	defer stop()
	peer, err := Pair(pairCtx, Peer{Address: "https://" + address + "/"}, "Test client", func(ctx context.Context) (string, error) {
		select {
		case code := <-otp:
			return code, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if peer.Address != address || peer.ID != s.ID || peer.Token == "" {
		t.Fatalf("bad pairing: %+v", peer)
	}
}

func TestDiscoverySkipsUnscopedLinkLocalAddress(t *testing.T) {
	e := zeroconf.NewServiceEntry("computer", discoveryService, "local.")
	e.Port = 8486
	e.HostName = "computer.local."
	e.Text = []string{"id=remote", "name=Computer"}
	e.AddrIPv6 = []net.IP{net.ParseIP("fe80::1"), net.ParseIP("fd00::2")}
	p, ok := discoveredPeer(e)
	if !ok || p.Address != "[fd00::2]:8486" {
		t.Fatalf("bad IPv6 address: %+v", p)
	}
	e.AddrIPv6 = e.AddrIPv6[:1]
	p, ok = discoveredPeer(e)
	if !ok || p.Address != "computer.local:8486" {
		t.Fatalf("missing hostname fallback: %+v", p)
	}
	e.AddrIPv4 = []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("192.168.2.15")}
	p, ok = discoveredPeer(e)
	if !ok || p.Address != "192.168.2.15:8486" {
		t.Fatalf("bad IPv4 preference: %+v", p)
	}
}
