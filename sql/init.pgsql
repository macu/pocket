-- Clean up previous instance

DROP INDEX IF EXISTS post_text_fulltext_idx;
DROP TABLE IF EXISTS topic CASCADE;
DROP TABLE IF EXISTS post_topic CASCADE;
DROP TABLE IF EXISTS post CASCADE;
DROP TYPE IF EXISTS vote_type;

DROP INDEX IF EXISTS user_account_handle_idx;
DROP INDEX IF EXISTS user_account_email_idx;
DROP TABLE IF EXISTS user_signup_request CASCADE;
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
CREATE UNIQUE INDEX user_account_email_idx ON user_account (email);

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
	token VARCHAR(15) UNIQUE,
	created_at TIMESTAMPTZ
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

CREATE TABLE topic (
	id SERIAL PRIMARY KEY,
	topic_label VARCHAR(50) COLLATE case_insensitive NOT NULL UNIQUE,
	author INTEGER NOT NULL REFERENCES user_account (id),
	created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE post (
	id SERIAL PRIMARY KEY,
	parent_post_id INTEGER REFERENCES post (id) ON DELETE CASCADE, -- null for topics
	author INTEGER NOT NULL REFERENCES user_account (id) ON DELETE CASCADE,
	post_text TEXT COLLATE case_insensitive NOT NULL, -- written story, joke, etc.
	created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX post_text_fulltext_idx ON post USING GIN (to_tsvector('simple', post_text));

CREATE TYPE vote_type AS ENUM (
	'upvote',
	'downvote'
);

CREATE TABLE post_topic (
	post_id INTEGER NOT NULL REFERENCES post (id) ON DELETE CASCADE,
	author INTEGER NOT NULL REFERENCES user_account (id) ON DELETE CASCADE, -- pin author's
	topic_id INTEGER NOT NULL REFERENCES topic (id) ON DELETE CASCADE,
	vote_type vote_type NOT NULL,
	created_at TIMESTAMPTZ NOT NULL
);
