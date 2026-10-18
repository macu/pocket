package moderation

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"pocket/pkg/env"
)

const (
	Model = "omni-moderation-latest"

	requestTimeout = 10 * time.Second
)

var skipWarning sync.Once

var moderationURL = "https://api.openai.com/v1/moderations"

// ErrRejected is returned when content fails the moderation check.
var ErrRejected = errors.New("content rejected by moderation")

type RejectedError struct {
	Categories map[string]float64
}

func (e *RejectedError) Error() string {
	names := make([]string, 0, len(e.Categories))
	for name, score := range e.Categories {
		names = append(names, fmt.Sprintf("%s=%.3f", name, score))
	}
	sort.Strings(names)
	return fmt.Sprintf("%s: %s", ErrRejected.Error(), strings.Join(names, ", "))
}

func (e *RejectedError) Unwrap() error {
	return ErrRejected
}

type moderationRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"`
}

type moderationResponse struct {
	Results []struct {
		CategoryScores map[string]float64 `json:"category_scores"`
		Flagged        *bool              `json:"flagged"`
	} `json:"results"`
}

// CheckText sends text to the OpenAI moderation API and returns a *RejectedError
// (matching ErrRejected) if OpenAI flags the content.
// Other errors indicate the check could not be performed.
func CheckText(ctx context.Context, text string) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return check(ctx, text, false)
}

// CheckImage fails closed even locally: unmoderated images must never be public.
func CheckImage(ctx context.Context, data []byte, contentType string) error {
	input := []map[string]any{{
		"type":      "image_url",
		"image_url": map[string]string{"url": "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data)},
	}}
	return check(ctx, input, true)
}

func check(ctx context.Context, input any, required bool) error {
	apiKey := env.GetOpenAISecretKey()
	if apiKey == "" {
		if env.IsLocal() && !required {
			skipWarning.Do(func() {
				log.Println("WARNING: OPENAI_SECRET_KEY not set; skipping AI moderation")
			})
			return nil
		}
		return fmt.Errorf("OpenAI secret key undefined")
	}

	payload, err := json.Marshal(moderationRequest{Model: Model, Input: input})
	if err != nil {
		return fmt.Errorf("encoding moderation request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, moderationURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("creating moderation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending moderation request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("OpenAI moderation returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result moderationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decoding moderation response: %w", err)
	}
	if len(result.Results) == 0 {
		return fmt.Errorf("moderation response contained no results")
	}

	for _, r := range result.Results {
		if r.Flagged == nil {
			return fmt.Errorf("moderation response contained no flagged decision")
		}
		if *r.Flagged {
			return &RejectedError{Categories: r.CategoryScores}
		}
	}
	return nil
}

// CheckProfile moderates the public profile fields together in a single request.
func CheckProfile(ctx context.Context, handle, displayName string) error {
	return CheckText(ctx, fmt.Sprintf("Handle: %s\nDisplay name: %s", handle, displayName))
}

// CheckPost moderates a post's topics and text together in a single request,
// with the topics listed at the start of the text.
func CheckPost(ctx context.Context, topicNames []string, text string) error {
	var b strings.Builder
	if len(topicNames) > 0 {
		b.WriteString("Topics: ")
		b.WriteString(strings.Join(topicNames, ", "))
		b.WriteString("\n\n")
	}
	b.WriteString(text)
	return CheckText(ctx, b.String())
}
