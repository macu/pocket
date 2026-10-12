package recaptcha

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestVerifyRecaptchaLocalBypass(t *testing.T) {
	os.Setenv("APP_ENGINE", "0")

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	ok, err := VerifyRecaptcha(req)
	if err != nil {
		t.Fatalf("unexpected error on local bypass: %v", err)
	}
	if !ok {
		t.Fatalf("expected true on local bypass, got false")
	}

	okWithAction, err := VerifyRecaptcha(req, "login")
	if err != nil {
		t.Fatalf("unexpected error on local bypass with action: %v", err)
	}
	if !okWithAction {
		t.Fatalf("expected true on local bypass with action, got false")
	}
}

func TestVerifyRecaptchaMissingTokenNonLocal(t *testing.T) {
	os.Setenv("APP_ENGINE", "1")
	defer os.Setenv("APP_ENGINE", "0")

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	ok, err := VerifyRecaptcha(req)
	if err == nil {
		t.Fatalf("expected error for missing token on non-local, got nil")
	}
	if ok {
		t.Fatalf("expected ok=false for missing token")
	}
}

func TestVerifyRecaptchaV3(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse Response
		serverStatus   int
		expectedAction string
		minScore       float64
		expectedOK     bool
		expectErr      bool
	}{
		{
			name: "valid high score matches action",
			serverResponse: Response{
				Success: true,
				Score:   0.9,
				Action:  "login",
			},
			serverStatus:   http.StatusOK,
			expectedAction: "login",
			minScore:       0.5,
			expectedOK:     true,
			expectErr:      false,
		},
		{
			name: "low score fails default threshold",
			serverResponse: Response{
				Success: true,
				Score:   0.3,
				Action:  "login",
			},
			serverStatus:   http.StatusOK,
			expectedAction: "login",
			minScore:       0.5,
			expectedOK:     false,
			expectErr:      false,
		},
		{
			name: "action mismatch fails",
			serverResponse: Response{
				Success: true,
				Score:   0.9,
				Action:  "signup",
			},
			serverStatus:   http.StatusOK,
			expectedAction: "login",
			minScore:       0.5,
			expectedOK:     false,
			expectErr:      false,
		},
		{
			name: "success false fails",
			serverResponse: Response{
				Success:    false,
				ErrorCodes: []string{"invalid-input-response"},
			},
			serverStatus:   http.StatusOK,
			expectedAction: "",
			minScore:       0.5,
			expectedOK:     false,
			expectErr:      false,
		},
		{
			name: "server non-200 returns error",
			serverResponse: Response{
				Success: false,
			},
			serverStatus:   http.StatusInternalServerError,
			expectedAction: "",
			minScore:       0.5,
			expectedOK:     false,
			expectErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.serverStatus)
				_ = json.NewEncoder(w).Encode(tc.serverResponse)
			}))
			defer server.Close()

			origURL := siteVerifyURL
			siteVerifyURL = server.URL
			defer func() { siteVerifyURL = origURL }()

			os.Setenv("RECAPTCHA_SECRET", "test-secret")
			os.Setenv("APP_ENGINE", "0")

			form := url.Values{}
			form.Set("g-recaptcha-response", "sample-token")
			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			var ok bool
			var err error
			if tc.expectedAction != "" {
				ok, err = VerifyRecaptchaWithMinScore(req, tc.minScore, tc.expectedAction)
			} else {
				ok, err = VerifyRecaptchaWithMinScore(req, tc.minScore)
			}

			if tc.expectErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.expectErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok != tc.expectedOK {
				t.Fatalf("expected ok=%v, got %v", tc.expectedOK, ok)
			}
		})
	}
}
