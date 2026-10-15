package net

import (
	"net/url"
	"testing"
)

func TestBuildAbsoluteURL(t *testing.T) {
	cases := []struct {
		name      string
		appEngine string
		domain    string
		publicURL string
		want      string
		wantErr   bool
	}{
		{"local domain with port", "", "localhost:2026", "", "http://localhost:2026/verify-signup?token=abc", false},
		{"domain takes precedence over PUBLIC_URL", "", "localhost:2026", "http://localhost:9999", "http://localhost:2026/verify-signup?token=abc", false},
		{"local explicit https scheme", "", "https://dev.example", "", "https://dev.example/verify-signup?token=abc", false},
		{"app engine uses https", "1", "pocket.example", "", "https://pocket.example/verify-signup?token=abc", false},
		{"app engine rejects http", "1", "http://pocket.example", "", "", true},
		{"falls back to PUBLIC_URL", "", "", "http://localhost:2026", "http://localhost:2026/verify-signup?token=abc", false},
		{"missing configuration", "", "", "", "", true},
		{"rejects path", "", "localhost:2026/app", "", "", true},
		{"rejects credentials", "", "user:pass@localhost:2026", "", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("APP_ENGINE", c.appEngine)
			t.Setenv("DOMAIN", c.domain)
			t.Setenv("PUBLIC_URL", c.publicURL)
			got, err := BuildAbsoluteURL("/verify-signup", url.Values{"token": {"abc"}})
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}
