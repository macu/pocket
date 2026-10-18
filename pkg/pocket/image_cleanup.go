package pocket

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"pocket/pkg/images"
)

// TrackImageUpload makes abandoned uploads eligible for eventual cleanup.
func TrackImageUpload(ctx context.Context, conn *sql.DB, object string) error {
	_, err := conn.ExecContext(ctx, `INSERT INTO post_image_cleanup (object_name, not_before)
		VALUES ($1, now() + interval '1 hour')`, object)
	return err
}

// CleanupPostImages drains a bounded batch. Rows survive failures for retry.
// Database triggers cover removal through all deletion paths, including cascades.
func CleanupPostImages(ctx context.Context, conn *sql.DB) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for i := 0; i < 20; i++ {
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		var object string
		err = tx.QueryRowContext(ctx, `SELECT object_name FROM post_image_cleanup
			WHERE not_before <= now() ORDER BY not_before
			LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(&object)
		if err != nil {
			tx.Rollback()
			if err == sql.ErrNoRows {
				return nil
			}
			return err
		}
		var referenced bool
		err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM post WHERE image_object = $1)`, object).Scan(&referenced)
		if err == nil && !referenced {
			err = images.Delete(ctx, object)
		}
		if err == nil {
			_, err = tx.ExecContext(ctx, `DELETE FROM post_image_cleanup WHERE object_name = $1`, object)
		}
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("cleaning image %s: %w", object, err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
