package ajax

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	ajaxutil "pocket/pkg/utils/ajax"
)

func TestAuthenticatedPostsRequireRecaptcha(t *testing.T) {
	t.Setenv("APP_ENGINE", "0")
	t.Setenv("RECAPTCHA_KEY", "site-key")
	auth := &ajaxutil.Auth{UserID: 1, Role: "admin"}
	check := func(path string) {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			AjaxHandler(nil, auth, w, httptest.NewRequest(http.MethodPost, path, nil))
			if w.Code != http.StatusForbidden {
				t.Fatalf("got HTTP %d, want 403", w.Code)
			}
			var payload ajaxutil.AjaxErrorPayload
			if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.ErrorCode != "recaptcha_failed" {
				t.Fatalf("unexpected payload: %+v", payload)
			}
			var diagnostics map[string]json.RawMessage
			if err := json.Unmarshal(w.Body.Bytes(), &diagnostics); err != nil {
				t.Fatal(err)
			}
			if string(diagnostics["score"]) != "null" || string(diagnostics["reason"]) != `"assessment_failed"` {
				t.Fatalf("unexpected diagnostics: %s", w.Body.String())
			}
		})
	}
	for path := range ajaxHandlersAuthRequired[http.MethodPost] {
		if path == "/ajax/logout" {
			continue
		}
		check(path)
	}
	for path := range ajaxHandlersAuthOptional[http.MethodPost] {
		check(path)
	}
}

func TestUnauthenticatedOptionalPostsRequireRecaptcha(t *testing.T) {
	t.Setenv("APP_ENGINE", "0")
	t.Setenv("RECAPTCHA_KEY", "site-key")
	for path := range ajaxHandlersAuthOptional[http.MethodPost] {
		t.Run(path, func(t *testing.T) {
			w := httptest.NewRecorder()
			AjaxHandler(nil, nil, w, httptest.NewRequest(http.MethodPost, path, nil))
			if w.Code != http.StatusForbidden {
				t.Fatalf("got HTTP %d, want 403", w.Code)
			}
			var payload ajaxutil.AjaxErrorPayload
			if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.ErrorCode != "recaptcha_failed" {
				t.Fatalf("unexpected payload: %+v", payload)
			}
		})
	}
}

func TestRecaptchaRequirementPolicy(t *testing.T) {
	auth := &ajaxutil.Auth{UserID: 1}
	for _, tc := range []struct {
		name   string
		method string
		path   string
		auth   *ajaxutil.Auth
		want   bool
	}{
		{"unauthenticated login", http.MethodPost, "/ajax/login", nil, true},
		{"authenticated post", http.MethodPost, "/ajax/post", auth, true},
		{"logout", http.MethodPost, "/ajax/logout", auth, false},
		{"unauthenticated signup", http.MethodPost, "/ajax/signup", nil, true},
		{"get request", http.MethodGet, "/ajax/login", auth, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := requiresRecaptcha(tc.method, tc.path, tc.auth); got != tc.want {
				t.Fatalf("requiresRecaptcha(%q, %q, auth) = %t, want %t", tc.method, tc.path, got, tc.want)
			}
		})
	}
}

func TestUnchangedRequests(t *testing.T) {
	t.Setenv("APP_ENGINE", "0")
	for _, tc := range []struct {
		method string
		path   string
		status int
	}{
		{http.MethodGet, "/ajax/load-login", http.StatusOK},
		{http.MethodPost, "/ajax/post", http.StatusForbidden},
		{http.MethodPost, "/ajax/post/delete", http.StatusForbidden},
		{http.MethodPost, "/ajax/unknown", http.StatusNotFound},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			AjaxHandler(nil, nil, w, httptest.NewRequest(tc.method, tc.path, nil))
			if w.Code != tc.status {
				t.Fatalf("got HTTP %d, want %d", w.Code, tc.status)
			}
			if w.Body.String() == "{\"errorCode\":\"recaptcha_failed\"}\n" {
				t.Fatal("anonymous or unknown request should not be verified")
			}
		})
	}
}
