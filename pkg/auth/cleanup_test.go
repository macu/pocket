package auth

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestCleanupExpiredRecordsPostgres(t *testing.T) {
	db := accountTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	for i, age := range []time.Duration{2 * time.Hour, time.Hour, 30 * time.Minute} {
		userID := i + 1
		if _, err := db.Exec(`INSERT INTO user_account (email, display_name, auth_hash)
			VALUES ($1, 'User', 'unused')`, fmt.Sprintf("user%d@example.com", userID)); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO user_signup_request (email, token, created_at)
			VALUES ($1, $2, $3)`, fmt.Sprintf("signup%d@example.com", userID),
			fmt.Sprintf("signup%d", userID), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO user_email_change_request
			(user_id, email, original_email, token, created_at) VALUES ($1, $2, $3, $4, $5)`,
			userID, fmt.Sprintf("new%d@example.com", userID), fmt.Sprintf("user%d@example.com", userID),
			fmt.Sprintf("email%d", userID), now.Add(-age)); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO password_reset_request
			(user_id, sent_to_address, token, created_at) VALUES ($1, $2, $3, $4)`,
			userID, fmt.Sprintf("user%d@example.com", userID), fmt.Sprintf("reset%d", userID),
			now.Add(-age)); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO user_session (token, user_id, expires) VALUES ($1, $2, $3)`,
			fmt.Sprintf("session%d", userID), userID, now.Add(time.Hour-age)); err != nil {
			t.Fatal(err)
		}
	}
	for run := 0; run < 2; run++ {
		if err := cleanupExpiredRecords(context.Background(), db, now); err != nil {
			t.Fatal(err)
		}
		for _, check := range []struct {
			table string
			count int
		}{
			{"user_signup_request", 2},
			{"user_email_change_request", 1},
			{"password_reset_request", 1},
			{"user_session", 1},
			{"user_account", 3},
		} {
			var count int
			if err := db.QueryRow("SELECT count(*) FROM " + check.table).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != check.count {
				t.Errorf("run %d: %s has %d records, want %d", run, check.table, count, check.count)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := CleanupExpiredRecords(ctx, db); !errors.Is(err, context.Canceled) {
		t.Fatalf("cleanup with a canceled context: got %v, want context.Canceled", err)
	}
	if _, err := db.Exec("DROP TABLE user_email_change_request"); err != nil {
		t.Fatal(err)
	}
	if err := CleanupExpiredRecords(context.Background(), db); err == nil {
		t.Fatal("cleanup must report database failures")
	}
}
