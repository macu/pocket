BEGIN;

DROP INDEX IF EXISTS user_account_email_idx;
CREATE UNIQUE INDEX user_account_email_idx ON user_account (lower(email));

COMMIT;
