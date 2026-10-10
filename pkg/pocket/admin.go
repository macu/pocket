package pocket

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"pocket/pkg/user"
	"pocket/pkg/utils/db"
)

const AdminUserPageSize = 25

// ErrUserIsAdmin is returned when an operation isn't permitted on admin users.
var ErrUserIsAdmin = fmt.Errorf("operation not permitted on admin users")

// AdminUser is a user as shown in the admin user table.
type AdminUser struct {
	ID          uint      `json:"id"`
	Handle      string    `json:"handle,omitempty"`
	DisplayName string    `json:"displayName"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"createdAt"`
	TopicVotes  int       `json:"topicVotes"`
	PostVotes   int       `json:"postVotes"`
	Posts       int       `json:"posts"`
}

// IsAssignableRole reports whether role may be assigned through the admin UI.
// Admin is deliberately excluded; it is only granted by direct database changes.
func IsAssignableRole(role string) bool {
	switch role {
	case string(user.RoleModerator), string(user.RoleUser),
		string(user.RoleInactive), string(user.RoleBanned):
		return true
	}
	return false
}

// SearchAdminUsers loads a page of users, newest first, whose name, handle or
// email contain query (if not empty) and whose role is role (if not empty),
// along with the total number of matching users.
func SearchAdminUsers(conn *sql.DB, query string, role string, offset uint) ([]AdminUser, int, error) {

	var args []interface{}
	where := "TRUE"

	query = strings.TrimSpace(query)
	if query != "" {
		pattern := db.Arg(&args, "%"+escapeLikePattern(query)+"%")
		emailMatch := ""
		if localPart, domain, hasDomain := strings.Cut(strings.ToLower(query), "@"); hasDomain {
			localPart, _, _ = strings.Cut(localPart, "+")
			baseEmail := db.Arg(&args, localPart+"@"+domain)
			emailMatch = " OR (split_part(split_part(lower(u.email), '@', 1), '+', 1) || '@' || " +
				"split_part(lower(u.email), '@', 2)) = " + baseEmail
		}
		where += " AND (u.display_name ILIKE " + pattern +
			" OR u.handle ILIKE " + pattern +
			" OR u.email ILIKE " + pattern + emailMatch + ")"
	}
	if role != "" {
		if !user.CheckRoleValid(role) {
			return nil, 0, fmt.Errorf("invalid role: %s", role)
		}
		where += " AND u.user_role = " + db.Arg(&args, role) + "::user_role_type"
	}

	var total int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM user_account u WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting users: %w", err)
	}

	rows, err := conn.Query(`
		SELECT u.id, u.handle, u.display_name, u.email, u.user_role::text, u.created_at,
			(SELECT COUNT(*) FROM post_topic_vote v WHERE v.user_id = u.id),
			(SELECT COUNT(*) FROM post_vote v WHERE v.user_id = u.id),
			(SELECT COUNT(*) FROM post p WHERE p.author = u.id)
		FROM user_account u
		WHERE `+where+`
		ORDER BY u.created_at DESC, u.id DESC
		LIMIT `+db.Arg(&args, AdminUserPageSize)+` OFFSET `+db.Arg(&args, offset), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("loading users: %w", err)
	}
	defer rows.Close()

	users := make([]AdminUser, 0)
	for rows.Next() {
		var u AdminUser
		var handle sql.NullString
		if err := rows.Scan(&u.ID, &handle, &u.DisplayName, &u.Email, &u.Role, &u.CreatedAt,
			&u.TopicVotes, &u.PostVotes, &u.Posts); err != nil {
			return nil, 0, fmt.Errorf("scanning user: %w", err)
		}
		if handle.Valid {
			u.Handle = handle.String
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("reading users: %w", err)
	}

	return users, total, nil
}

// SetUserRole changes a user's role. Admins can't be changed, and admin can't
// be assigned. Returns the user's details and previous role, ErrUserIsAdmin
// for admin targets, and sql.ErrNoRows if the user doesn't exist.
func SetUserRole(conn *sql.DB, userID uint, role string) (AdminUser, string, error) {

	if !IsAssignableRole(role) {
		return AdminUser{}, "", fmt.Errorf("role %q cannot be assigned", role)
	}

	var user AdminUser
	var previousRole string
	err := conn.QueryRow(`
		WITH target AS (
			SELECT id, user_role FROM user_account
			WHERE id = $2 AND user_role <> 'admin'
			FOR UPDATE
		)
		UPDATE user_account u SET user_role = $1::user_role_type
		FROM target
		WHERE u.id = target.id
		RETURNING u.id, u.email, u.display_name, u.user_role::text, target.user_role::text
	`, role, userID).Scan(&user.ID, &user.Email, &user.DisplayName, &user.Role, &previousRole)
	if err != nil {
		if err == sql.ErrNoRows {
			var isAdmin bool
			err = conn.QueryRow(`SELECT user_role = 'admin' FROM user_account WHERE id = $1`, userID).Scan(&isAdmin)
			if err != nil {
				return AdminUser{}, "", err
			}
			if isAdmin {
				return AdminUser{}, "", ErrUserIsAdmin
			}
			return AdminUser{}, "", sql.ErrNoRows
		}
		return AdminUser{}, "", fmt.Errorf("updating role of user %d: %w", userID, err)
	}
	return user, previousRole, nil
}

// DeleteUserContent deletes everything a user has created or cast: their
// posts (along with those posts' sub-posts, votes and revisions), their post
// and topic votes (adjusting the cached vote sums), and topics they added to
// others' posts that no one else has voted for. Topics they created are kept
// since others may use them; see DeleteTopic. The account itself is kept. Returns ErrUserIsAdmin for
// admin users and sql.ErrNoRows if the user doesn't exist.
func DeleteUserContent(conn *sql.DB, userID uint) error {

	return db.InTransaction(conn, func(tx *sql.Tx) error {

		var role string
		if err := tx.QueryRow(`SELECT user_role::text FROM user_account WHERE id = $1 FOR UPDATE`, userID).Scan(&role); err != nil {
			return err
		}
		if user.CheckRoleAdmin(role) {
			return ErrUserIsAdmin
		}

		// Cached sums are recomputed from the other users' votes before this user's votes are deleted.
		if _, err := tx.Exec(`
			UPDATE post_vote_sum s SET
				upvotes = (SELECT COUNT(*) FROM post_vote v WHERE v.post_id = s.post_id AND v.user_id <> $1 AND v.vote_type = 'upvote'),
				downvotes = (SELECT COUNT(*) FROM post_vote v WHERE v.post_id = s.post_id AND v.user_id <> $1 AND v.vote_type = 'downvote'),
				sum = (SELECT COUNT(*) FILTER (WHERE v.vote_type = 'upvote') - COUNT(*) FILTER (WHERE v.vote_type = 'downvote')
					FROM post_vote v WHERE v.post_id = s.post_id AND v.user_id <> $1)
			WHERE EXISTS (SELECT 1 FROM post_vote v WHERE v.post_id = s.post_id AND v.user_id = $1)
		`, userID); err != nil {
			return fmt.Errorf("updating post vote sums: %w", err)
		}

		// Topics that were only kept on a post by this user's vote are removed from the post.
		if _, err := tx.Exec(`
			DELETE FROM post_topic_sum s
			WHERE EXISTS (SELECT 1 FROM post_topic_vote v WHERE v.post_id = s.post_id AND v.topic_id = s.topic_id AND v.user_id = $1)
				AND NOT EXISTS (SELECT 1 FROM post_topic_vote v WHERE v.post_id = s.post_id AND v.topic_id = s.topic_id AND v.user_id <> $1)
		`, userID); err != nil {
			return fmt.Errorf("removing topics from posts: %w", err)
		}

		if _, err := tx.Exec(`
			UPDATE post_topic_sum s SET
				upvotes = (SELECT COUNT(*) FROM post_topic_vote v WHERE v.post_id = s.post_id AND v.topic_id = s.topic_id AND v.user_id <> $1 AND v.vote_type = 'upvote'),
				downvotes = (SELECT COUNT(*) FROM post_topic_vote v WHERE v.post_id = s.post_id AND v.topic_id = s.topic_id AND v.user_id <> $1 AND v.vote_type = 'downvote'),
				sum = (SELECT COUNT(*) FILTER (WHERE v.vote_type = 'upvote') - COUNT(*) FILTER (WHERE v.vote_type = 'downvote')
					FROM post_topic_vote v WHERE v.post_id = s.post_id AND v.topic_id = s.topic_id AND v.user_id <> $1)
			WHERE EXISTS (SELECT 1 FROM post_topic_vote v WHERE v.post_id = s.post_id AND v.topic_id = s.topic_id AND v.user_id = $1)
		`, userID); err != nil {
			return fmt.Errorf("updating topic sums: %w", err)
		}

		if _, err := tx.Exec(`DELETE FROM post_vote WHERE user_id = $1`, userID); err != nil {
			return fmt.Errorf("deleting post votes: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM post_topic_vote WHERE user_id = $1`, userID); err != nil {
			return fmt.Errorf("deleting topic votes: %w", err)
		}

		// Cascades to sub-posts, votes, sums and revisions.
		if _, err := tx.Exec(`DELETE FROM post WHERE author = $1`, userID); err != nil {
			return fmt.Errorf("deleting posts: %w", err)
		}

		return nil
	})
}

type DeletedPost struct {
	ParentPostID *uint
	AuthorEmail  string
	PostText     string
}

// DeletePost deletes a post along with its sub-posts, votes and revisions.
// Returns details of the deleted post and sql.ErrNoRows if it doesn't exist.
func DeletePost(conn *sql.DB, postID uint) (*DeletedPost, error) {
	var parentID sql.NullInt64
	var deletedPost DeletedPost
	err := conn.QueryRow(`
		DELETE FROM post p
		USING user_account u
		WHERE p.id = $1 AND u.id = p.author
		RETURNING p.parent_post_id, p.post_text, u.email
	`, postID).Scan(&parentID, &deletedPost.PostText, &deletedPost.AuthorEmail)
	if err != nil {
		return nil, err
	}
	if parentID.Valid {
		v := uint(parentID.Int64)
		deletedPost.ParentPostID = &v
	}
	return &deletedPost, nil
}

// GetAdminUser loads a single user as shown to admins. Returns sql.ErrNoRows
// if the user doesn't exist.
func GetAdminUser(conn *sql.DB, userID uint) (*AdminUser, error) {
	var u AdminUser
	var handle sql.NullString
	err := conn.QueryRow(`
		SELECT u.id, u.handle, u.display_name, u.email, u.user_role::text, u.created_at,
			(SELECT COUNT(*) FROM post_topic_vote v WHERE v.user_id = u.id),
			(SELECT COUNT(*) FROM post_vote v WHERE v.user_id = u.id),
			(SELECT COUNT(*) FROM post p WHERE p.author = u.id)
		FROM user_account u
		WHERE u.id = $1
	`, userID).Scan(&u.ID, &handle, &u.DisplayName, &u.Email, &u.Role, &u.CreatedAt,
		&u.TopicVotes, &u.PostVotes, &u.Posts)
	if err != nil {
		return nil, err
	}
	u.Handle = handle.String
	return &u, nil
}

// AdminTopic is a topic as shown in the admin user's topic table.
type AdminTopic struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	PostCount int       `json:"postCount"`
	Banned    bool      `json:"banned"`
}

// LoadUserCreatedTopics loads a page of topics created by the user, newest first,
// along with the total number of topics they created.
func LoadUserCreatedTopics(conn *sql.DB, userID uint, offset uint) ([]AdminTopic, int, error) {

	var total int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM topic WHERE created_by = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting topics: %w", err)
	}

	rows, err := conn.Query(`
		SELECT t.id, t.name, t.created_at,
			(SELECT COUNT(*) FROM post_topic_sum s WHERE s.topic_id = t.id),
			t.banned
		FROM topic t
		WHERE t.created_by = $1
		ORDER BY t.created_at DESC, t.id DESC
		LIMIT $2 OFFSET $3`, userID, AdminUserPageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("loading topics: %w", err)
	}
	defer rows.Close()

	topics := make([]AdminTopic, 0)
	for rows.Next() {
		var t AdminTopic
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.PostCount, &t.Banned); err != nil {
			return nil, 0, fmt.Errorf("scanning topic: %w", err)
		}
		topics = append(topics, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("reading topics: %w", err)
	}

	return topics, total, nil
}

const MaxBulkTopicStatus = 100

// AdminSiteTopic is a topic as shown in the admin site-wide topic table.
type AdminSiteTopic struct {
	AdminTopic
	CreatedBy       uint   `json:"createdBy"`
	CreatedByName   string `json:"createdByName"`
	CreatedByHandle string `json:"createdByHandle,omitempty"`
}

// SearchAdminTopics loads a page of all topics, newest first, whose name
// contains query (if not empty), along with the total number of matches.
func SearchAdminTopics(conn *sql.DB, query string, offset uint) ([]AdminSiteTopic, int, error) {

	var args []interface{}
	where := "TRUE"
	query = strings.TrimSpace(query)
	if query != "" {
		where += " AND t.name ILIKE " + db.Arg(&args, "%"+escapeLikePattern(query)+"%")
	}

	var total int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM topic t WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting topics: %w", err)
	}

	rows, err := conn.Query(`
		SELECT t.id, t.name, t.created_at,
			(SELECT COUNT(*) FROM post_topic_sum s WHERE s.topic_id = t.id),
			t.banned, t.created_by, u.display_name, u.handle
		FROM topic t
		JOIN user_account u ON u.id = t.created_by
		WHERE `+where+`
		ORDER BY t.created_at DESC, t.id DESC
		LIMIT `+db.Arg(&args, AdminUserPageSize)+` OFFSET `+db.Arg(&args, offset), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("loading topics: %w", err)
	}
	defer rows.Close()

	topics := make([]AdminSiteTopic, 0)
	for rows.Next() {
		var t AdminSiteTopic
		var handle sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.PostCount,
			&t.Banned, &t.CreatedBy, &t.CreatedByName, &handle); err != nil {
			return nil, 0, fmt.Errorf("scanning topic: %w", err)
		}
		t.CreatedByHandle = handle.String
		topics = append(topics, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("reading topics: %w", err)
	}

	return topics, total, nil
}

// SetTopicsBanned changes the banned status of the given topics.
func SetTopicsBanned(conn *sql.DB, topicIDs []uint, banned bool) (int64, error) {
	if len(topicIDs) == 0 {
		return 0, nil
	}
	args := []interface{}{banned}
	placeholders := make([]string, 0, len(topicIDs))
	for _, id := range topicIDs {
		placeholders = append(placeholders, db.Arg(&args, id))
	}
	result, err := conn.Exec(`UPDATE topic SET banned = $1 WHERE id IN (`+
		strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return 0, fmt.Errorf("updating topic banned status: %w", err)
	}
	return result.RowsAffected()
}

// AdminPost is a post as shown in the admin user's post table.
type AdminPost struct {
	ID           uint      `json:"id"`
	ParentPostID *uint     `json:"parentPostId"`
	PostText     string    `json:"postText"`
	CreatedAt    time.Time `json:"createdAt"`
	Upvotes      int       `json:"upvotes"`
	Downvotes    int       `json:"downvotes"`
	Sum          int       `json:"sum"`
	SubPosts     int       `json:"subPosts"`
}

const adminPostTextPreviewLength = 200

// LoadUserAuthoredPosts loads a page of posts written by the user, newest
// first (with text truncated to a preview), along with their total number.
func LoadUserAuthoredPosts(conn *sql.DB, userID uint, offset uint) ([]AdminPost, int, error) {

	var total int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM post WHERE author = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting posts: %w", err)
	}

	rows, err := conn.Query(`
		SELECT p.id, p.parent_post_id, LEFT(p.post_text, $4), p.created_at,
			COALESCE(s.upvotes, 0), COALESCE(s.downvotes, 0), COALESCE(s.sum, 0),
			(SELECT COUNT(*) FROM post c WHERE c.parent_post_id = p.id)
		FROM post p
		LEFT JOIN post_vote_sum s ON s.post_id = p.id
		WHERE p.author = $1
		ORDER BY p.created_at DESC, p.id DESC
		LIMIT $2 OFFSET $3`, userID, AdminUserPageSize, offset, adminPostTextPreviewLength)
	if err != nil {
		return nil, 0, fmt.Errorf("loading posts: %w", err)
	}
	defer rows.Close()

	posts := make([]AdminPost, 0)
	for rows.Next() {
		var p AdminPost
		var parentID sql.NullInt64
		if err := rows.Scan(&p.ID, &parentID, &p.PostText, &p.CreatedAt,
			&p.Upvotes, &p.Downvotes, &p.Sum, &p.SubPosts); err != nil {
			return nil, 0, fmt.Errorf("scanning post: %w", err)
		}
		if parentID.Valid {
			v := uint(parentID.Int64)
			p.ParentPostID = &v
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("reading posts: %w", err)
	}

	return posts, total, nil
}
