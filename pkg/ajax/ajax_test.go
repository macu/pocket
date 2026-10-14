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
		check(path)
	}
	for path := range ajaxHandlersAuthOptional[http.MethodPost] {
		check(path)
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
		{http.MethodPost, "/ajax/login", http.StatusBadRequest},
		{http.MethodPost, "/ajax/signup", http.StatusBadRequest},
		{http.MethodPost, "/ajax/post", http.StatusForbidden},
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
