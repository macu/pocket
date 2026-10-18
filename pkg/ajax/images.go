package ajax

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"time"

	"pocket/pkg/images"
	"pocket/pkg/moderation"
	"pocket/pkg/pocket"
	"pocket/pkg/utils/ajax"
	"pocket/pkg/utils/logging"
)

var checkPostImage = moderation.CheckImage

// readPostImage never trusts a filename or the client-provided MIME type.
func readPostImage(r *http.Request) (*images.Image, error) {
	if r.MultipartForm == nil {
		return nil, nil
	}
	files := r.MultipartForm.File["image"]
	for field, uploads := range r.MultipartForm.File {
		if field != "image" && len(uploads) > 0 {
			return nil, images.ErrInvalid
		}
	}
	if len(files) == 0 {
		return nil, nil
	}
	if len(files) != 1 {
		return nil, images.ErrInvalid
	}
	file, err := files[0].Open()
	if err != nil {
		return nil, fmt.Errorf("opening image: %w", err)
	}
	defer file.Close()
	return images.Process(file)
}

func uploadPostImage(r *http.Request, db *sql.DB, img *images.Image) (string, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	object := images.NewObjectName(img)
	if err := pocket.TrackImageUpload(ctx, db, object); err != nil {
		return "", fmt.Errorf("tracking image upload: %w", err)
	}
	if err := images.Upload(ctx, object, img); err != nil {
		return "", err
	}
	return object, nil
}

func postImageFailure(r *http.Request, auth *ajax.Auth, err error) (any, int) {
	if errors.Is(err, images.ErrInvalid) || errors.Is(err, multipart.ErrMessageTooLarge) {
		return ajax.AjaxErrorPayload{ErrorCode: "invalid-post-image"}, http.StatusBadRequest
	}
	if payload, status, ok := moderationFailure(r, auth, err); !ok {
		return payload, status
	}
	logging.LogError(r, auth, err)
	return nil, http.StatusInternalServerError
}

func moderatePostImage(r *http.Request, img *images.Image) error {
	if img == nil {
		return nil
	}
	return checkPostImage(r.Context(), img.Data, img.ContentType)
}
