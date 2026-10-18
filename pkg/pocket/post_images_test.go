package pocket

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"

	ajaxutil "pocket/pkg/utils/ajax"
)

func postImageTestDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("POCKET_ACCOUNT_TEST_DB") != "1" {
		t.Skip("set POCKET_ACCOUNT_TEST_DB=1 with DB_USER/DB_PASS/DB_NAME for isolated PostgreSQL tests")
	}
	u := &url.URL{Scheme: "postgres", Host: "localhost:5432", Path: os.Getenv("DB_NAME"),
		User: url.UserPassword(os.Getenv("DB_USER"), os.Getenv("DB_PASS")), RawQuery: "sslmode=disable"}
	admin, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	schema := "image_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.Exec("CREATE SCHEMA " + pq.QuoteIdentifier(schema)); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	query := u.Query()
	query.Set("search_path", schema+",public")
	u.RawQuery = query.Encode()
	conn, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn.Close()
		if _, err := admin.Exec("DROP SCHEMA " + pq.QuoteIdentifier(schema) + " CASCADE"); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	return conn
}

func initPostImageDB(t *testing.T, conn *sql.DB) {
	t.Helper()
	script, err := os.ReadFile("../../sql/init.pgsql")
	if err != nil {
		t.Fatal(err)
	}
	// Only create objects in the isolated schema; never run init's teardown or
	// create a shared extension as part of a test.
	ddl := string(script)
	ddl = ddl[strings.Index(ddl, "CREATE TYPE user_role_type"):]
	ddl = strings.ReplaceAll(ddl, "CREATE EXTENSION IF NOT EXISTS pg_trgm;", "")
	ddl = strings.ReplaceAll(ddl, "CREATE INDEX topic_name_trgm_idx ON topic USING GIN (name gin_trgm_ops);", "")
	if _, err := conn.Exec(ddl); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(`INSERT INTO user_account (email, display_name, auth_hash, created_at)
		VALUES ('owner@example.com', 'Owner', 'unused', now()), ('other@example.com', 'Other', 'unused', now())`); err != nil {
		t.Fatal(err)
	}
}

func TestPostImageContent(t *testing.T) {
	for _, test := range []struct {
		text, object string
		valid        bool
	}{
		{"", "", false}, {"   ", "", false}, {"Text", "", true}, {"", "posts/image.png", true},
		{"Text", "posts/image.png", true}, {strings.Repeat("x", MaxPostLength+1), "posts/image.png", false},
	} {
		if err := ValidatePostContent(test.text, test.object); (err == nil) != test.valid {
			t.Errorf("text length=%d object=%q: %v", len(test.text), test.object, err)
		}
	}
}

func TestPostImagesAndRevisionsPostgres(t *testing.T) {
	conn := postImageTestDB(t)
	initPostImageDB(t, conn)
	t.Setenv("POST_IMAGE_BUCKET", "test-post-images")
	post, err := CreatePostWithImage(conn, nil, 1, "", []string{"Pictures"}, "posts/first.png")
	if err != nil {
		t.Fatal(err)
	}
	if post.PostText != "" || !strings.HasSuffix(post.ImageURL, "/posts/first.png") || len(post.Topics) != 1 {
		t.Fatalf("image-only creation returned incomplete post: %+v", post)
	}
	id := post.ID
	if _, err := UpdatePostContent(conn, id, 2, "", nil); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("non-owner edited image post: %v", err)
	}
	empty := ""
	if _, err := UpdatePostContent(conn, id, 1, "", &empty); !errors.Is(err, ErrEmptyPostContent) {
		t.Fatalf("empty post accepted: %v", err)
	}
	replacement := "posts/second.webp"
	if _, err := UpdatePostContent(conn, id, 1, "", &replacement); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdatePostContent(conn, id, 1, "New text", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdatePostContent(conn, id, 1, "New text", &empty); err != nil {
		t.Fatal(err)
	}
	added := "posts/third.jpg"
	if _, err := UpdatePostContent(conn, id, 1, "New text", &added); err != nil {
		t.Fatal(err)
	}
	revisions, total, err := LoadPostRevisions(conn, id, 0)
	if err != nil || total != 5 {
		t.Fatalf("revisions total=%d err=%v", total, err)
	}
	for i, want := range []string{"added", "deleted", "", "updated", "added"} {
		if revisions[i].ImageChange != want {
			t.Errorf("revision %d: imageChange=%q want %q", i, revisions[i].ImageChange, want)
		}
	}
	payload, err := json.Marshal(revisions)
	if err != nil || strings.Contains(string(payload), "posts/") || strings.Contains(string(payload), "imageUrl") {
		t.Fatalf("revisions retained image references: %s %v", payload, err)
	}
	if _, err := UpdatePostContent(conn, id, 1, "New text", nil); err != nil {
		t.Fatal(err)
	}
	_, total, err = LoadPostRevisions(conn, id, 0)
	if err != nil || total != 5 {
		t.Fatalf("no-op edit added a revision: total=%d err=%v", total, err)
	}
	auth := &ajaxutil.Auth{UserID: 1}
	topics := []uint{post.Topics[0].ID}
	for _, selected := range [][]uint{nil, topics} {
		posts, err := LoadTopPosts(conn, auth, 0, selected, nil, false, nil)
		if err != nil || len(posts) != 1 || posts[0].ImageObject != added || posts[0].ImageURL == "" {
			t.Fatalf("dashboard image missing: %+v err=%v", posts, err)
		}
		posts, err = LoadUserPosts(conn, auth, 1, 0, selected, nil, false, nil)
		if err != nil || len(posts) != 1 || posts[0].ImageObject != added {
			t.Fatalf("user post image missing: %+v err=%v", posts, err)
		}
	}
	child, err := CreatePostWithImage(conn, &id, 2, "", nil, "posts/child.png")
	if err != nil {
		t.Fatal(err)
	}
	children, err := LoadTopSubPosts(conn, auth, id, 0, nil, nil, false, nil)
	if err != nil || len(children) != 1 || children[0].ID != child.ID || children[0].ImageURL == "" {
		t.Fatalf("subpost image missing: %+v err=%v", children, err)
	}
	if _, err := DeleteOwnPost(conn, id, 1); err != nil {
		t.Fatal(err)
	}
	var queued int
	if err := conn.QueryRow(`SELECT count(*) FROM post_image_cleanup`).Scan(&queued); err != nil || queued != 4 {
		t.Fatalf("removed/replaced/cascaded images not queued: %d %v", queued, err)
	}
}

func TestPostImageMigrationPostgres(t *testing.T) {
	conn := postImageTestDB(t)
	if _, err := conn.Exec(`CREATE TABLE post (id SERIAL PRIMARY KEY, post_text TEXT NOT NULL);
		CREATE TABLE post_revision (id SERIAL PRIMARY KEY, post_id INTEGER, post_text TEXT, replaced_at TIMESTAMPTZ);
		INSERT INTO post (post_text) VALUES ('Existing post');`); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../sql/migrations/20261018_post_images.pgsql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(string(migration)); err != nil {
		t.Fatal(err)
	}
	var object string
	if err := conn.QueryRow(`SELECT image_object FROM post WHERE id=1`).Scan(&object); err != nil || object != "" {
		t.Fatalf("existing post not preserved: %q %v", object, err)
	}
	if _, err := conn.Exec(`INSERT INTO post (post_text) VALUES ('')`); err == nil {
		t.Fatal("database allowed completely empty post")
	}
}

func TestImageUploadTrackingPostgres(t *testing.T) {
	conn := postImageTestDB(t)
	initPostImageDB(t, conn)
	if err := TrackImageUpload(context.Background(), conn, "posts/pending.png"); err != nil {
		t.Fatal(err)
	}
	var future bool
	if err := conn.QueryRow(`SELECT not_before > now() FROM post_image_cleanup`).Scan(&future); err != nil || !future {
		t.Fatalf("new upload must not be deleted while saving: %v %v", future, err)
	}
	if _, err := conn.Exec(`INSERT INTO post (author, post_text, image_object, created_at)
		VALUES (1, '', 'posts/active.png', now());
		INSERT INTO post_image_cleanup VALUES ('posts/active.png', now())`); err != nil {
		t.Fatal(err)
	}
	// No bucket credentials are needed: a live image must never be deleted.
	t.Setenv("POST_IMAGE_BUCKET", "")
	if err := CleanupPostImages(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := conn.QueryRow(`SELECT count(*) FROM post_image_cleanup`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("cleanup removed pending upload or retained live entry: %d %v", count, err)
	}
	if _, err := conn.Exec(`UPDATE post_image_cleanup SET not_before = now()`); err != nil {
		t.Fatal(err)
	}
	if err := CleanupPostImages(context.Background(), conn); err == nil {
		t.Fatal("missing bucket should fail cleanup")
	}
	if err := conn.QueryRow(`SELECT count(*) FROM post_image_cleanup`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("failed deletion was not retained for retry: %d %v", count, err)
	}
}
