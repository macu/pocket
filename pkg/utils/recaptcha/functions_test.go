package recaptcha

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"google.golang.org/api/option"
	"google.golang.org/api/recaptchaenterprise/v1"
)

func TestVerifyMissingConfiguration(t *testing.T) {
	for _, appEngine := range []string{"0", "1"} {
		t.Run("APP_ENGINE="+appEngine, func(t *testing.T) {
			t.Setenv("APP_ENGINE", appEngine)
			t.Setenv("RECAPTCHA_KEY", "")
			t.Setenv("GOOGLE_CLOUD_PROJECT", "")
			req := httptest.NewRequest(http.MethodPost, "/ajax/post", nil)
			if ok, err := VerifyRecaptcha(req); ok != (appEngine == "0") || (err == nil) != (appEngine == "0") {
				t.Fatalf("empty key bypass must be local only: ok=%v err=%v", ok, err)
			}
			req.Header.Set("X-Recaptcha-Token", "token")
			if ok, err := VerifyRecaptcha(req); ok != (appEngine == "0") || (err == nil) != (appEngine == "0") {
				t.Fatalf("empty key bypass must be local only: ok=%v err=%v", ok, err)
			}
			t.Setenv("RECAPTCHA_KEY", "site-key")
			if ok, err := VerifyRecaptcha(req); ok || err == nil {
				t.Fatal("missing project must fail")
			}
			req.Header.Del("X-Recaptcha-Token")
			if ok, err := VerifyRecaptcha(req); ok || err == nil {
				t.Fatal("configured key must require a token, including locally")
			}
		})
	}
}

func TestVerifyEnterprise(t *testing.T) {
	tests := []struct {
		name     string
		response string
		status   int
		wantOK   bool
	}{
		{"human", `{"tokenProperties":{"valid":true,"action":"post_topic_vote"},"riskAnalysis":{"score":0.9}}`, 200, true},
		{"threshold inclusive", `{"tokenProperties":{"valid":true,"action":"post_topic_vote"},"riskAnalysis":{"score":0.5}}`, 200, true},
		{"below threshold", `{"tokenProperties":{"valid":true,"action":"post_topic_vote"},"riskAnalysis":{"score":0.49}}`, 200, false},
		{"invalid token", `{"tokenProperties":{"valid":false,"invalidReason":"EXPIRED"},"riskAnalysis":{"score":0.9}}`, 200, false},
		{"wrong action", `{"tokenProperties":{"valid":true,"action":"logout"},"riskAnalysis":{"score":0.9}}`, 200, false},
		{"missing properties", `{"riskAnalysis":{"score":0.9}}`, 200, false},
		{"missing analysis", `{"tokenProperties":{"valid":true,"action":"post_topic_vote"}}`, 200, false},
		{"zero score", `{"tokenProperties":{"valid":true,"action":"post_topic_vote"},"riskAnalysis":{}}`, 200, false},
		{"invalid score", `{"tokenProperties":{"valid":true,"action":"post_topic_vote"},"riskAnalysis":{"score":1.1}}`, 200, false},
		{"malformed response", `not json`, 200, false},
		{"API failure", `{"error":{"message":"unavailable"}}`, 503, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RECAPTCHA_KEY", "site-key")
			t.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/v1/projects/test-project/assessments" {
					t.Errorf("unexpected assessment endpoint: %s %s", r.Method, r.URL.Path)
				}
				var body recaptchaenterprise.GoogleCloudRecaptchaenterpriseV1Assessment
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body.Event == nil || body.Event.Token != "sample-token" ||
					body.Event.SiteKey != "site-key" || body.Event.ExpectedAction != "post_topic_vote" ||
					body.Event.UserAgent != "test-browser" {
					t.Errorf("unexpected assessment event: %+v", body.Event)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.response))
			}))
			defer server.Close()
			original := newAssessmentService
			newAssessmentService = func(ctx context.Context) (*recaptchaenterprise.Service, error) {
				return recaptchaenterprise.NewService(ctx, option.WithEndpoint(server.URL+"/"),
					option.WithHTTPClient(server.Client()), option.WithoutAuthentication())
			}
			t.Cleanup(func() { newAssessmentService = original })

			req := httptest.NewRequest(http.MethodPost, "/ajax/post/topic/vote", nil)
			req.Header.Set("X-Recaptcha-Token", "sample-token")
			req.Header.Set("User-Agent", "test-browser")
			ok, err := VerifyRecaptcha(req)
			if ok != tc.wantOK || (err == nil) != tc.wantOK {
				t.Fatalf("got ok=%v err=%v, want ok=%v", ok, err, tc.wantOK)
			}
			if !tc.wantOK {
				var verificationError *VerificationError
				if !errors.As(err, &verificationError) {
					t.Fatalf("expected diagnostic verification error, got %v", err)
				}
				var assessment recaptchaenterprise.GoogleCloudRecaptchaenterpriseV1Assessment
				decodeErr := json.Unmarshal([]byte(tc.response), &assessment)
				if tc.status == http.StatusOK && decodeErr == nil && assessment.RiskAnalysis != nil {
					if verificationError.Score == nil || *verificationError.Score != assessment.RiskAnalysis.Score {
						t.Fatalf("score not preserved: %+v", verificationError)
					}
				} else if verificationError.Score != nil {
					t.Fatalf("expected unavailable score, got %g", *verificationError.Score)
				}
				if tc.name == "below threshold" && verificationError.Reason != "low_score" {
					t.Fatalf("unexpected reason: %s", verificationError.Reason)
				}
			}
		})
	}
}

func TestExtractToken(t *testing.T) {
	for _, field := range []string{"g-recaptcha-response", "recaptcha_token"} {
		form := url.Values{field: {"form-token"}}
		req := httptest.NewRequest(http.MethodPost, "/ajax/post", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if got := extractToken(req); got != "form-token" {
			t.Fatalf("got %q for %s", got, field)
		}
		req.Header.Set("X-Recaptcha-Token", "header-token")
		if got := extractToken(req); got != "header-token" {
			t.Fatalf("header should take precedence, got %q", got)
		}
	}
}

func TestActionForPath(t *testing.T) {
	for path, want := range map[string]string{
		"/ajax/post":                      "post",
		"/ajax/logout":                    "logout",
		"/ajax/post/topic/vote":           "post_topic_vote",
		"/ajax/admin/user/delete-content": "admin_user_delete_content",
		"/ajax/signup-verify":             "signup_verify",
	} {
		if got := ActionForPath(path); got != want {
			t.Errorf("ActionForPath(%q) = %q, want %q", path, got, want)
		}
	}
}
