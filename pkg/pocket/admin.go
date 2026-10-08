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
		where += " AND (u.display_name ILIKE " + pattern +
			" OR u.handle ILIKE " + pattern +
			" OR u.email ILIKE " + pattern + ")"
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
// be assigned. Returns ErrUserIsAdmin for admin targets and sql.ErrNoRows if
// the user doesn't exist.
func SetUserRole(conn *sql.DB, userID uint, role string) error {

	if !IsAssignableRole(role) {
		return fmt.Errorf("role %q cannot be assigned", role)
	}

	result, err := conn.Exec(`
		UPDATE user_account SET user_role = $1::user_role_type
		WHERE id = $2 AND user_role <> 'admin'
	`, role, userID)
	if err != nil {
		return fmt.Errorf("updating role of user %d: %w", userID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected for user %d: %w", userID, err)
	}
	if affected > 0 {
		return nil
	}

	var isAdmin bool
	err = conn.QueryRow(`SELECT user_role = 'admin' FROM user_account WHERE id = $1`, userID).Scan(&isAdmin)
	if err != nil {
		return err // includes sql.ErrNoRows
	}
	return ErrUserIsAdmin
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

// DeletePost deletes a post along with its sub-posts, votes and revisions.
// Returns the deleted post's parent ID (if any) and sql.ErrNoRows if the post
// doesn't exist.
func DeletePost(conn *sql.DB, postID uint) (*uint, error) {
	var parentID sql.NullInt64
	err := conn.QueryRow(`DELETE FROM post WHERE id = $1 RETURNING parent_post_id`, postID).Scan(&parentID)
	if err != nil {
		return nil, err
	}
	if parentID.Valid {
		v := uint(parentID.Int64)
		return &v, nil
	}
	return nil, nil
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
			(SELECT COUNT(*) FROM post_topic_sum s WHERE s.topic_id = t.id)
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
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.PostCount); err != nil {
			return nil, 0, fmt.Errorf("scanning topic: %w", err)
		}
		topics = append(topics, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("reading topics: %w", err)
	}

	return topics, total, nil
}

// MaxBulkTopicDelete is the most topics that can be deleted in one request.
const MaxBulkTopicDelete = 100

// DeleteUserTopics deletes the given topics, along with their votes and tags
// on posts, but only those created by the user. Returns the number deleted.
func DeleteUserTopics(conn *sql.DB, userID uint, topicIDs []uint) (int64, error) {
	if len(topicIDs) == 0 {
		return 0, nil
	}
	args := []interface{}{userID}
	placeholders := make([]string, 0, len(topicIDs))
	for _, id := range topicIDs {
		placeholders = append(placeholders, db.Arg(&args, id))
	}
	result, err := conn.Exec(`DELETE FROM topic WHERE created_by = $1 AND id IN (`+
		strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return 0, fmt.Errorf("deleting topics of user %d: %w", userID, err)
	}
	return result.RowsAffected()
}
