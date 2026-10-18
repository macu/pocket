package images

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"pocket/pkg/moderation"
)

func TestPublicCloudUpload(t *testing.T) {
	if os.Getenv("POCKET_IMAGE_CLOUD_TEST") != "1" {
		t.Skip("set POCKET_IMAGE_CLOUD_TEST=1 with POST_IMAGE_BUCKET, OPENAI_SECRET_KEY and ADC")
	}
	img, err := Process(bytes.NewReader(encodedImage(t, "png", 100, 100)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := moderation.CheckImage(ctx, img.Data, img.ContentType); err != nil {
		t.Fatal(err)
	}
	object := NewObjectName(img)
	if err := Upload(ctx, object, img); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := Delete(ctx, object); err != nil {
			t.Error(err)
		}
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, PublicURL(object), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("public image not accessible: status=%d type=%s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	saved, err := io.ReadAll(resp.Body)
	if err != nil || !bytes.Equal(saved, img.Data) {
		t.Fatalf("public image bytes did not match moderated bytes: %v", err)
	}
	if err := Upload(ctx, object, img); err == nil {
		t.Fatal("immutable image object was overwritten")
	}
}
