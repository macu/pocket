package moderation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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
			text, ok := req.Input.(string)
			if !ok {
				t.Errorf("expected text input, got %T", req.Input)
			}
			*gotInput = text
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

func TestCheckProfileIncludesPublicFields(t *testing.T) {
	var input string
	withServer(t, map[string]float64{}, &input)
	if err := CheckProfile(context.Background(), "Matt_99", "Matt Smith"); err != nil {
		t.Fatal(err)
	}
	if input != "Handle: Matt_99\nDisplay name: Matt Smith" {
		t.Fatalf("unexpected input %q", input)
	}
}

func TestCheckProfileRejects(t *testing.T) {
	withServer(t, map[string]float64{"hate": 0.9}, nil)
	if err := CheckProfile(context.Background(), "handle", "Name"); !errors.Is(err, ErrRejected) {
		t.Fatalf("expected ErrRejected, got %v", err)
	}
}

func TestCheckImagePayload(t *testing.T) {
	var flagged atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Model string `json:"model"`
			Input []struct {
				Type     string `json:"type"`
				ImageURL struct {
					URL string `json:"url"`
				} `json:"image_url"`
			} `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		want := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("processed image"))
		if payload.Model != Model || len(payload.Input) != 1 || payload.Input[0].Type != "image_url" || payload.Input[0].ImageURL.URL != want {
			t.Errorf("unexpected payload: %+v", payload)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"category_scores": map[string]float64{"violence": 0.1}, "flagged": flagged.Load()}},
		})
	}))
	defer server.Close()
	original := moderationURL
	moderationURL = server.URL
	t.Cleanup(func() { moderationURL = original })
	t.Setenv("OPENAI_SECRET_KEY", "test-key")
	if err := CheckImage(context.Background(), []byte("processed image"), "image/png"); err != nil {
		t.Fatal(err)
	}
	flagged.Store(true)
	if err := CheckImage(context.Background(), []byte("processed image"), "image/png"); !errors.Is(err, ErrRejected) {
		t.Fatalf("flagged image accepted despite low scores: %v", err)
	}
}

func TestCheckImageFailsClosed(t *testing.T) {
	t.Run("missing local key", func(t *testing.T) {
		t.Setenv("APP_ENGINE", "0")
		t.Setenv("OPENAI_SECRET_KEY", "")
		if err := CheckImage(context.Background(), []byte("image"), "image/png"); err == nil {
			t.Fatal("unmoderated local image accepted")
		}
	})
	t.Run("rejected", func(t *testing.T) {
		withServer(t, map[string]float64{"violence": 0.9}, nil)
		if err := CheckImage(context.Background(), []byte("image"), "image/png"); !errors.Is(err, ErrRejected) {
			t.Fatalf("expected rejection: %v", err)
		}
	})
	t.Run("missing scores", func(t *testing.T) {
		withServer(t, map[string]float64{}, nil)
		if err := CheckImage(context.Background(), []byte("image"), "image/png"); err == nil {
			t.Fatal("empty moderation result accepted")
		}
	})
}
