package images

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/HugoSmits86/nativewebp"
	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/storage/v1"
)

const MaxUploadBytes = 10 << 20
const MinDimension = 100
const MaxDimension = 1200
const MaxPixels = 20_000_000

var ErrInvalid = errors.New("image must be JPEG, PNG, or static WebP, at least 100x100 pixels, at most 20 megapixels and 10 MiB")

type Image struct {
	Data        []byte
	ContentType string
	Extension   string
}

// Process decodes and re-encodes the pixels, excluding embedded metadata.
func Process(reader io.Reader) (*Image, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxUploadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading image: %w", err)
	}
	if len(data) > MaxUploadBytes {
		return nil, ErrInvalid
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png" && format != "webp") {
		return nil, ErrInvalid
	}
	if config.Width < MinDimension || config.Height < MinDimension ||
		int64(config.Width)*int64(config.Height) > MaxPixels {
		return nil, ErrInvalid
	}
	// Reject animations rather than moderating and saving only their first frame.
	if format == "webp" && len(data) >= 21 && string(data[12:16]) == "VP8X" && data[20]&2 != 0 {
		return nil, ErrInvalid
	}
	if format == "png" && animatedPNG(data) {
		return nil, ErrInvalid
	}
	pixels, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, ErrInvalid
	}
	width, height := pixels.Bounds().Dx(), pixels.Bounds().Dy()
	if width > MaxDimension || height > MaxDimension {
		if width >= height {
			height = max(1, height*MaxDimension/width)
			width = MaxDimension
		} else {
			width = max(1, width*MaxDimension/height)
			height = MaxDimension
		}
		resized := image.NewNRGBA(image.Rect(0, 0, width, height))
		draw.CatmullRom.Scale(resized, resized.Bounds(), pixels, pixels.Bounds(), draw.Src, nil)
		pixels = resized
	}
	var output bytes.Buffer
	result := &Image{Extension: format, ContentType: "image/" + format}
	switch format {
	case "jpeg":
		result.Extension = "jpg"
		err = jpeg.Encode(&output, pixels, &jpeg.Options{Quality: 85})
	case "png":
		err = png.Encode(&output, pixels)
	case "webp":
		err = nativewebp.Encode(&output, pixels, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("encoding image: %w", err)
	}
	result.Data = output.Bytes()
	return result, nil
}

func animatedPNG(data []byte) bool {
	for offset := 8; offset+12 <= len(data); {
		length := int64(data[offset])<<24 | int64(data[offset+1])<<16 |
			int64(data[offset+2])<<8 | int64(data[offset+3])
		if string(data[offset+4:offset+8]) == "acTL" {
			return true
		}
		next := int64(offset) + length + 12
		if next > int64(len(data)) {
			break
		}
		offset = int(next)
	}
	return false
}

func NewObjectName(img *Image) string {
	return "posts/" + uuid.NewString() + "." + img.Extension
}

func PublicURL(object string) string {
	if object == "" {
		return ""
	}
	return "https://storage.googleapis.com/" + url.PathEscape(os.Getenv("POST_IMAGE_BUCKET")) + "/" + object
}

func bucket() (string, error) {
	name := os.Getenv("POST_IMAGE_BUCKET")
	if name == "" || strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("POST_IMAGE_BUCKET is missing or invalid")
	}
	return name, nil
}

func Upload(ctx context.Context, object string, img *Image) error {
	name, err := bucket()
	if err != nil {
		return err
	}
	service, err := storage.NewService(ctx)
	if err != nil {
		return fmt.Errorf("creating Cloud Storage client: %w", err)
	}
	_, err = service.Objects.Insert(name, &storage.Object{
		Name: object, ContentType: img.ContentType, CacheControl: "public, max-age=3600",
	}).Media(bytes.NewReader(img.Data), googleapi.ContentType(img.ContentType)).
		IfGenerationMatch(0).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("uploading post image: %w", err)
	}
	return nil
}

func Delete(ctx context.Context, object string) error {
	name, err := bucket()
	if err != nil {
		return err
	}
	service, err := storage.NewService(ctx)
	if err != nil {
		return fmt.Errorf("creating Cloud Storage client: %w", err)
	}
	err = service.Objects.Delete(name, object).Context(ctx).Do()
	var apiError *googleapi.Error
	if errors.As(err, &apiError) && apiError.Code == http.StatusNotFound {
		return nil
	}
	return err
}
