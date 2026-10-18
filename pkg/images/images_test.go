package images

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"github.com/HugoSmits86/nativewebp"
)

func encodedImage(t *testing.T, format string, width, height int) []byte {
	t.Helper()
	pixels := image.NewNRGBA(image.Rect(0, 0, width, height))
	pixels.SetNRGBA(0, 0, color.NRGBA{R: 100, A: 120})
	var output bytes.Buffer
	var err error
	switch format {
	case "jpeg":
		err = jpeg.Encode(&output, pixels, nil)
	case "png":
		err = png.Encode(&output, pixels)
	case "webp":
		err = nativewebp.Encode(&output, pixels, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestProcessFormatsAndDimensions(t *testing.T) {
	for _, format := range []string{"jpeg", "png", "webp"} {
		for _, size := range []struct{ w, h, wantW, wantH int }{
			{100, 100, 100, 100}, {2400, 1200, 1200, 600}, {800, 1600, 600, 1200},
		} {
			data := encodedImage(t, format, size.w, size.h)
			result, err := Process(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("%s %dx%d: %v", format, size.w, size.h, err)
			}
			config, gotFormat, err := image.DecodeConfig(bytes.NewReader(result.Data))
			if err != nil || gotFormat != format || config.Width != size.wantW || config.Height != size.wantH {
				t.Fatalf("format=%s size=%+v: config=%+v format=%s err=%v", format, size, config, gotFormat, err)
			}
			if result.ContentType != "image/"+format {
				t.Fatalf("unexpected content type %s", result.ContentType)
			}
		}
	}
}

func TestProcessRejectsInvalidImages(t *testing.T) {
	for _, data := range [][]byte{
		[]byte("<svg></svg>"), []byte("not an image"),
		encodedImage(t, "png", 99, 100),
		encodedImage(t, "jpeg", 100, 99),
		encodedImage(t, "png", 5000, 4001),
		[]byte(strings.Repeat("x", MaxUploadBytes+1)),
	} {
		if _, err := Process(bytes.NewReader(data)); err != ErrInvalid {
			t.Errorf("invalid image accepted: %v", err)
		}
	}
}

func TestProcessPreservesPNGTransparency(t *testing.T) {
	result, err := Process(bytes.NewReader(encodedImage(t, "png", 100, 100)))
	if err != nil {
		t.Fatal(err)
	}
	pixels, _, err := image.Decode(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, alpha := pixels.At(0, 0).RGBA()
	if alpha != 120*257 {
		t.Fatalf("PNG alpha changed: %d", alpha)
	}
}

func TestUniqueNamesAndPublicURL(t *testing.T) {
	t.Setenv("POST_IMAGE_BUCKET", "test-post-images")
	img := &Image{Extension: "png"}
	first, second := NewObjectName(img), NewObjectName(img)
	if first == second || !strings.HasPrefix(first, "posts/") || !strings.HasSuffix(first, ".png") {
		t.Fatalf("invalid names %s %s", first, second)
	}
	if got := PublicURL(first); got != "https://storage.googleapis.com/test-post-images/"+first {
		t.Fatal(got)
	}
	if PublicURL("") != "" {
		t.Fatal("missing images must not have a URL")
	}
}
