package net

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"pocket/pkg/env"
)

// getUserIP returns the IP address of the remote connection.
func GetUserIP(r *http.Request) string {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

func IsAjax(r *http.Request) bool {
	return strings.ToLower(r.Header.Get("X-Requested-With")) == "xmlhttprequest"
}

// getLocalPort returns the server (local) port number for the given request.
func GetLocalPort(r *http.Request) (string, error) {
	a, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok {
		return "", fmt.Errorf("getting local address from request context")
	}
	_, port, err := net.SplitHostPort(a.String())
	if err != nil {
		return "", fmt.Errorf("extracting port: %w", err)
	}
	return port, nil
}

// BaseURL returns the public base URL used for links to the site, such as in emails.
// It is built from DOMAIN (e.g. "localhost:2026" or "pocket.example"), using https in
// App Engine and http locally; DOMAIN may also include an explicit scheme.
// PUBLIC_URL is used only when DOMAIN is unset.
func BaseURL() (*url.URL, error) {
	base := strings.TrimSpace(env.GetDomain())
	if base != "" && !strings.Contains(base, "://") {
		scheme := "http"
		if env.IsAppEngine() {
			scheme = "https"
		}
		base = scheme + "://" + base
	}
	if base == "" {
		base = strings.TrimSpace(env.GetPublicURL())
	}
	u, err := url.Parse(base)
	if base == "" || err != nil || u.Host == "" || (u.Path != "" && u.Path != "/") ||
		(u.Scheme != "https" && !(env.IsLocal() && u.Scheme == "http")) ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("DOMAIN must be a valid host (with optional port) for the public site")
	}
	u.Path = "/"
	return u, nil
}

// BuildAbsoluteURL returns an absolute public URL to the given path and query.
func BuildAbsoluteURL(path string, query url.Values) (string, error) {
	base, err := BaseURL()
	if err != nil {
		return "", err
	}
	return base.ResolveReference(&url.URL{
		Path:     "/" + strings.TrimPrefix(path, "/"),
		RawQuery: query.Encode(),
	}).String(), nil
}
