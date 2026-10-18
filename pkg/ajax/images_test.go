package ajax

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"pocket/pkg/images"
	"pocket/pkg/moderation"
	ajaxutil "pocket/pkg/utils/ajax"
)

func imageRequest(t *testing.T, field string, count int, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for i := 0; i < count; i++ {
		file, err := writer.CreateFormFile(field, "untrusted-name.svg")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/ajax/post", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestImageRequestValidation(t *testing.T) {
	t.Setenv("APP_ENGINE", "0")
	t.Setenv("RECAPTCHA_KEY", "")
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 100, 100))); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		field string
		count int
		data  []byte
	}{
		{"image", 1, []byte("not an image")},
		{"image", 2, encoded.Bytes()},
		{"other", 1, encoded.Bytes()},
	} {
		r := imageRequest(t, test.field, test.count, test.data)
		w := httptest.NewRecorder()
		AjaxHandler(nil, &ajaxutil.Auth{UserID: 1}, w, r)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("invalid image status: %d", w.Code)
		}
		var payload ajaxutil.AjaxErrorPayload
		if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil || payload.ErrorCode != "invalid-post-image" {
			t.Fatalf("unexpected error payload: %+v %v", payload, err)
		}
	}
}

func TestImageModeratedBeforeSaving(t *testing.T) {
	t.Setenv("APP_ENGINE", "0")
	t.Setenv("RECAPTCHA_KEY", "")
	t.Setenv("OPENAI_SECRET_KEY", "")
	original := checkPostImage
	t.Cleanup(func() { checkPostImage = original })
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 2400, 1200))); err != nil {
		t.Fatal(err)
	}
	for _, failure := range []struct {
		err    error
		status int
	}{
		{moderation.ErrRejected, http.StatusUnprocessableEntity},
		{errors.New("moderation unavailable"), http.StatusInternalServerError},
	} {
		called := false
		checkPostImage = func(ctx context.Context, data []byte, contentType string) error {
			called = true
			config, format, err := image.DecodeConfig(bytes.NewReader(data))
			if err != nil || format != "png" || contentType != "image/png" || config.Width != 1200 || config.Height != 600 {
				t.Errorf("moderation did not receive final image pixels: %+v %s %v", config, format, err)
			}
			return failure.err
		}
		r := imageRequest(t, "image", 1, encoded.Bytes())
		w := httptest.NewRecorder()
		// No DB or cloud client: either would fail if reached before rejection.
		AjaxHandler(nil, &ajaxutil.Auth{UserID: 1}, w, r)
		checkPostImage = original
		if !called || w.Code != failure.status {
			t.Fatalf("moderation called=%v status=%d want=%d", called, w.Code, failure.status)
		}
	}
}

func TestImageBodyLimit(t *testing.T) {
	r := imageRequest(t, "image", 1, bytes.Repeat([]byte("x"), images.MaxUploadBytes+(2<<20)))
	w := httptest.NewRecorder()
	AjaxHandler(nil, nil, w, r)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized upload status=%d", w.Code)
	}
}
