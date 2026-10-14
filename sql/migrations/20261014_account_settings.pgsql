BEGIN;

CREATE TABLE user_email_change_request (
	user_id INTEGER PRIMARY KEY REFERENCES user_account (id) ON DELETE CASCADE,
	email VARCHAR(50) NOT NULL,
	original_email VARCHAR(50) NOT NULL,
	token VARCHAR(64) UNIQUE NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE password_reset_request ALTER COLUMN token TYPE VARCHAR(64);
DELETE FROM password_reset_request WHERE token IS NULL OR created_at IS NULL;
ALTER TABLE password_reset_request ALTER COLUMN token SET NOT NULL;
ALTER TABLE password_reset_request ALTER COLUMN created_at SET NOT NULL;
CREATE INDEX password_reset_request_user_idx ON password_reset_request (user_id);

COMMIT;
