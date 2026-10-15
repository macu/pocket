package moderation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func withServer(t *testing.T, scores map[string]float64, gotInput *string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected Authorization header %q", r.Header.Get("Authorization"))
		}
		var req moderationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != Model {
			t.Errorf("model = %q, want %q", req.Model, Model)
		}
		if gotInput != nil {
			*gotInput = req.Input
		}
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"category_scores": scores}},
		})
	}))
	t.Cleanup(server.Close)
	original := moderationURL
	moderationURL = server.URL
	t.Cleanup(func() { moderationURL = original })
	t.Setenv("OPENAI_SECRET_KEY", "test-key")
}

func TestCheckTextPasses(t *testing.T) {
	withServer(t, map[string]float64{"violence": 0.1, "hate": ScoreThreshold}, nil)
	if err := CheckText(context.Background(), "hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckTextRejects(t *testing.T) {
	withServer(t, map[string]float64{"violence": 0.9, "hate": 0.1}, nil)
	err := CheckText(context.Background(), "bad")
	if !errors.Is(err, ErrRejected) {
		t.Fatalf("expected ErrRejected, got %v", err)
	}
	var rejected *RejectedError
	if !errors.As(err, &rejected) || len(rejected.Categories) != 1 || rejected.Categories["violence"] != 0.9 {
		t.Fatalf("unexpected rejected categories: %+v", rejected)
	}
}

func TestCheckPostPrependsTopics(t *testing.T) {
	var input string
	withServer(t, map[string]float64{}, &input)
	if err := CheckPost(context.Background(), []string{"cats", "dogs"}, "post body"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(input, "Topics: cats, dogs\n\n") || !strings.HasSuffix(input, "post body") {
		t.Fatalf("unexpected input %q", input)
	}
}
