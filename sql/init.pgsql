-- Clean up previous instance

DROP INDEX IF EXISTS post_text_fulltext_idx;
DROP INDEX IF EXISTS topic_name_trgm_idx;
-- DROP TABLE IF EXISTS topic_score CASCADE;
-- DROP TABLE IF EXISTS post_topic_score CASCADE;
-- DROP TABLE IF EXISTS post_topic_pin CASCADE;
DROP TABLE IF EXISTS post_vote_sum CASCADE;
DROP TABLE IF EXISTS post_vote CASCADE;
DROP TABLE IF EXISTS post_topic_sum CASCADE;
DROP TABLE IF EXISTS topic CASCADE;
DROP TABLE IF EXISTS post_topic_vote CASCADE;
DROP TABLE IF EXISTS post_topic CASCADE;
DROP TABLE IF EXISTS post_revision CASCADE;
DROP TABLE IF EXISTS post CASCADE;
DROP TABLE IF EXISTS post_image_cleanup;
DROP FUNCTION IF EXISTS queue_post_image_cleanup();
DROP TYPE IF EXISTS vote_type;

DROP INDEX IF EXISTS user_account_handle_idx;
DROP INDEX IF EXISTS user_account_email_idx;
DROP INDEX IF EXISTS password_reset_request_user_idx;
DROP TABLE IF EXISTS user_signup_request CASCADE;
DROP TABLE IF EXISTS user_email_change_request CASCADE;
DROP TABLE IF EXISTS password_reset_request CASCADE;
DROP TABLE IF EXISTS user_session CASCADE;
DROP TABLE IF EXISTS user_account CASCADE;
DROP TYPE IF EXISTS user_role_type;

DROP COLLATION IF EXISTS case_insensitive;

--------------------------------------------------
-- Create user management and session tables

CREATE TYPE user_role_type AS ENUM (
	'admin', -- can do anything
	'moderator', -- can delete and edit stuff
	'user', -- can create categories, posts, comments, and votes
	'inactive', -- can't do anything
	'banned' -- can't do anything
);

CREATE TABLE user_account (
	id SERIAL PRIMARY KEY,
	email VARCHAR(50) UNIQUE NOT NULL,
	user_role user_role_type NOT NULL DEFAULT 'user',
	handle VARCHAR(25) UNIQUE, -- optional handle
	display_name VARCHAR(50) NOT NULL, -- required
	auth_hash VARCHAR(60) NOT NULL,
	user_settings JSON,
	created_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX user_account_handle_idx ON user_account (handle);
CREATE UNIQUE INDEX user_account_email_idx ON user_account (lower(email));

CREATE TABLE user_session (
	token VARCHAR(30) PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES user_account (id) ON DELETE CASCADE,
	expires TIMESTAMPTZ NOT NULL
);

CREATE TABLE user_signup_request (
	id SERIAL PRIMARY KEY,
	email VARCHAR(50) NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	token VARCHAR(15) UNIQUE NOT NULL
);

CREATE TABLE password_reset_request (
	id SERIAL PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES user_account (id) ON DELETE CASCADE,
	sent_to_address VARCHAR(50) NOT NULL,
	token VARCHAR(64) UNIQUE NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX password_reset_request_user_idx ON password_reset_request (user_id);

CREATE TABLE user_email_change_request (
	user_id INTEGER PRIMARY KEY REFERENCES user_account (id) ON DELETE CASCADE,
	email VARCHAR(50) NOT NULL,
	original_email VARCHAR(50) NOT NULL,
	token VARCHAR(64) UNIQUE NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
);

--------------------------------------------------
-- Create metadata objects

CREATE COLLATION case_insensitive (
	provider = icu, -- "International Components for Unicode"
	-- und stands for undefined (ICU root collation - language agnostic)
	-- colStrength=primary ignores case and accents
	-- colNumeric=yes sorts strings with numeric parts by numeric value
	-- colAlternate=shifted would recognize equality of equivalent punctuation sequences
	locale = 'und@colStrength=primary;colNumeric=yes',
	deterministic = false
);

--------------------------------------------------

CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE topic (
	id SERIAL PRIMARY KEY,
	name VARCHAR(50) NOT NULL,
	created_by INTEGER NOT NULL REFERENCES user_account (id),
	created_at TIMESTAMPTZ NOT NULL,
	banned BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX topic_name_trgm_idx ON topic USING GIN (name gin_trgm_ops);

CREATE TABLE post (
	id SERIAL PRIMARY KEY,
	parent_post_id INTEGER REFERENCES post (id) ON DELETE CASCADE, -- null for topics
	author INTEGER NOT NULL REFERENCES user_account (id) ON DELETE CASCADE,
	post_text TEXT COLLATE case_insensitive NOT NULL, -- written story, joke, etc.
	image_object TEXT NOT NULL DEFAULT '',
	image_change TEXT NOT NULL DEFAULT '' CHECK (image_change IN ('', 'added', 'updated', 'deleted')),
	created_at TIMESTAMPTZ NOT NULL,
	CONSTRAINT post_content_required CHECK (btrim(post_text) <> '' OR image_object <> '')
);

CREATE INDEX post_text_fulltext_idx ON post USING GIN (to_tsvector('simple', post_text));

-- previous contents of a post, recorded each time it is edited
CREATE TABLE post_revision (
	id SERIAL PRIMARY KEY,
	post_id INTEGER NOT NULL REFERENCES post (id) ON DELETE CASCADE,
	post_text TEXT NOT NULL, -- content before the edit
	image_change TEXT NOT NULL DEFAULT '' CHECK (image_change IN ('', 'added', 'updated', 'deleted')),
	replaced_at TIMESTAMPTZ NOT NULL -- when this content was replaced
);

CREATE INDEX post_revision_post_idx ON post_revision (post_id, replaced_at);

CREATE TABLE post_image_cleanup (
	object_name TEXT PRIMARY KEY,
	not_before TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE FUNCTION queue_post_image_cleanup() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
	IF OLD.image_object <> '' THEN
		INSERT INTO post_image_cleanup (object_name) VALUES (OLD.image_object)
			ON CONFLICT (object_name) DO UPDATE SET not_before = now();
	END IF;
	RETURN OLD;
END;
$$;

CREATE TRIGGER post_image_deleted AFTER DELETE ON post
	FOR EACH ROW EXECUTE FUNCTION queue_post_image_cleanup();
CREATE TRIGGER post_image_replaced AFTER UPDATE OF image_object ON post
	FOR EACH ROW WHEN (OLD.image_object IS DISTINCT FROM NEW.image_object)
	EXECUTE FUNCTION queue_post_image_cleanup();

CREATE TYPE vote_type AS ENUM (
	'upvote',
	'downvote'
);

CREATE TABLE post_vote (
	post_id INTEGER NOT NULL REFERENCES post (id) ON DELETE CASCADE,
	user_id INTEGER NOT NULL REFERENCES user_account (id) ON DELETE CASCADE,
	vote_type vote_type NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (post_id, user_id)
);

CREATE TABLE post_vote_sum (
	post_id INTEGER PRIMARY KEY REFERENCES post (id) ON DELETE CASCADE,
	upvotes INTEGER NOT NULL,
	downvotes INTEGER NOT NULL,
	sum INTEGER NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE post_topic_vote (
	post_id INTEGER NOT NULL REFERENCES post (id) ON DELETE CASCADE,
	user_id INTEGER NOT NULL REFERENCES user_account (id) ON DELETE CASCADE,
	topic_id INTEGER NOT NULL REFERENCES topic (id) ON DELETE CASCADE,
	vote_type vote_type NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE post_topic_sum (
	post_id INTEGER NOT NULL REFERENCES post (id) ON DELETE CASCADE,
	topic_id INTEGER NOT NULL REFERENCES topic (id) ON DELETE CASCADE,
	upvotes INTEGER NOT NULL,
	downvotes INTEGER NOT NULL,
	sum INTEGER NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (post_id, topic_id)
);

-- CREATE TABLE post_topic_pin (
-- 	post_id INTEGER NOT NULL REFERENCES post (id) ON DELETE CASCADE,
-- 	topic_id INTEGER NOT NULL REFERENCES topic (id) ON DELETE CASCADE,
-- 	user_id INTEGER NOT NULL REFERENCES user_account (id) ON DELETE CASCADE,
-- 	created_at TIMESTAMPTZ NOT NULL,
-- 	PRIMARY KEY (post_id, topic_id, user_id)
-- );

-- CREATE TABLE topic_score (
-- 	topic_id INTEGER NOT NULL REFERENCES topic (id) ON DELETE CASCADE,
-- 	upvotes INTEGER NOT NULL,
-- 	downvotes INTEGER NOT NULL,
-- 	sum INTEGER NOT NULL,
-- 	created_at TIMESTAMPTZ NOT NULL,
-- 	PRIMARY KEY (topic_id)
-- );
