package network

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// NormalizeAddress accepts a host, IP, or pasted URL; transport always uses TLS.
func NormalizeAddress(address string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", errors.New("enter a computer address")
	}
	if strings.Contains(address, "://") {
		u, err := url.Parse(address)
		if err != nil {
			return "", errors.New("invalid computer address")
		}
		switch u.Scheme {
		case "http", "https", "ws", "wss":
		default:
			return "", errors.New("use a host name or IP address")
		}
		if u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return "", errors.New("use only the computer address and port")
		}
		address = u.Host
	}
	if strings.ContainsAny(address, "/?# \t\r\n") {
		return "", errors.New("invalid computer address")
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		host, port = strings.TrimSuffix(strings.TrimPrefix(address, "["), "]"), "8486"
		ipHost, _, _ := strings.Cut(host, "%")
		if strings.Contains(host, ":") && net.ParseIP(ipHost) == nil {
			return "", errors.New("invalid computer address or port")
		}
	}
	n, err := strconv.Atoi(port)
	if host == "" || err != nil || n < 1 || n > 65535 {
		return "", errors.New("invalid computer address or port")
	}
	return net.JoinHostPort(host, port), nil
}
