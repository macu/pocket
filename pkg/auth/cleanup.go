package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func CleanupExpiredRecords(ctx context.Context, db *sql.DB) error {
	return cleanupExpiredRecords(ctx, db, time.Now())
}

func cleanupExpiredRecords(ctx context.Context, db *sql.DB, now time.Time) error {
	for _, cleanup := range []struct {
		query  string
		expiry time.Duration
	}{
		{"DELETE FROM user_signup_request WHERE created_at < $1", signupTokenExpiry},
		{"DELETE FROM user_email_change_request WHERE created_at <= $1", accountTokenExpiry},
		{"DELETE FROM password_reset_request WHERE created_at <= $1", accountTokenExpiry},
	} {
		if _, err := db.ExecContext(ctx, cleanup.query, now.Add(-cleanup.expiry)); err != nil {
			return fmt.Errorf("cleaning expired auth records (%s): %w", cleanup.query, err)
		}
	}
	return deleteExpiredSessions(ctx, db, now)
}
