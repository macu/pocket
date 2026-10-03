package pocket

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"pocket/pkg/utils/ajax"
	"pocket/pkg/utils/db"
)

type Post struct {
	ID                uint      `json:"id"`
	ParentPostID      *uint     `json:"parentPostId,omitempty"`
	AuthorID          uint      `json:"authorId"`
	AuthorDisplayName string    `json:"authorDisplayName,omitempty"`
	AuthorHandle      string    `json:"authorHandle,omitempty"`
	PostText          string    `json:"postText"`
	CreatedAt         time.Time `json:"createdAt"`
	Topics            []Topic   `json:"topics,omitempty"`
	SubPostCount      int       `json:"subPostCount"`
	Sum               int       `json:"sum"`
	UserVote          *string   `json:"userVote"`
}

// PostCursor identifies the last post returned by a recent-mode page.
type PostCursor struct {
	CreatedAt time.Time
	ID        uint
}

func NormalizePostText(text string) string {
	return strings.TrimSpace(text)
}

func ValidatePostText(text string) error {
	text = NormalizePostText(text)
	if text == "" {
		return fmt.Errorf("post text cannot be empty")
	}
	if len(text) > MaxPostLength {
		return fmt.Errorf("post text exceeds maximum length of %d characters", MaxPostLength)
	}
	return nil
}

// LoadTopPosts loads a page of the site's top posts, ordered by their own net
// vote sum. If selectedTopicIDs is non-empty, results are restricted to posts
// that have all of the selected topics present. If cutoff is non-nil, only
// posts created at or after cutoff are considered, and vote sums only reflect
// votes cast at or after cutoff. If mostRecent is true, posts are ordered by
// creation time and ID, and cursor excludes posts at or before the last post.
func LoadTopPosts(conn *sql.DB, auth *ajax.Auth, offset uint, selectedTopicIDs []uint, cutoff *time.Time, mostRecent bool, cursor *PostCursor) ([]Post, error) {

	// offset is uint, so it cannot be negative

	var userID *uint
	if auth != nil {
		userID = &auth.UserID
	}

	var args []interface{}
	var query string

	var userIDParam any
	if userID != nil {
		userIDParam = *userID
	}
	userArg := db.Arg(&args, userIDParam) + "::INTEGER"

	var postWhere string
	if cutoff != nil {
		postWhere = "WHERE p.created_at >= " + db.Arg(&args, *cutoff)
	}
	if cursor != nil && mostRecent {
		cursorWhere := "(p.created_at, p.id) < (" + db.Arg(&args, cursor.CreatedAt) + ", " + db.Arg(&args, cursor.ID) + ")"
		if postWhere == "" {
			postWhere = "WHERE " + cursorWhere
		} else {
			postWhere += " AND " + cursorWhere
		}
	}
	if mostRecent && cursor != nil {
		offset = 0
	}
	orderBy := "vote_sum DESC, p.created_at DESC, p.id DESC"
	if mostRecent {
		orderBy = "p.created_at DESC, p.id DESC"
	}
	selectedOrderBy := orderBy

	if len(selectedTopicIDs) == 0 {
		query = `
			SELECT p.id, p.parent_post_id, p.author, u.display_name, u.handle, p.post_text, p.created_at,
				COALESCE(pvs.sum, 0) AS vote_sum, uv.vote_type,
				(SELECT COUNT(*) FROM post c WHERE c.parent_post_id = p.id) AS sub_post_count
			FROM post p
			LEFT JOIN user_account u ON u.id = p.author
			LEFT JOIN ` + filteredPostVoteSumTable(&args, cutoff) + ` pvs ON pvs.post_id = p.id
			LEFT JOIN post_vote uv ON uv.post_id = p.id AND uv.user_id = ` + userArg + `
			` + postWhere + `
			ORDER BY ` + orderBy + `
			LIMIT ` + db.Arg(&args, MaxPostPageSize) + ` OFFSET ` + db.Arg(&args, offset)
	} else {
		selectedClause := db.In("topic_id", &args, selectedTopicIDs)
		selectedCount := db.Arg(&args, len(selectedTopicIDs))
		query = `
			SELECT p.id, p.parent_post_id, p.author, u.display_name, u.handle, p.post_text, p.created_at,
				COALESCE(pvs.sum, 0) AS vote_sum, uv.vote_type,
				(SELECT COUNT(*) FROM post c WHERE c.parent_post_id = p.id) AS sub_post_count
			FROM post p
			LEFT JOIN user_account u ON u.id = p.author
			LEFT JOIN ` + filteredPostVoteSumTable(&args, cutoff) + ` pvs ON pvs.post_id = p.id
			LEFT JOIN post_vote uv ON uv.post_id = p.id AND uv.user_id = ` + userArg + `
			JOIN (
				SELECT post_id
				FROM ` + filteredPostTopicSumTable(&args, cutoff) + `
				WHERE ` + selectedClause + ` AND sum >= 0
				GROUP BY post_id
				HAVING COUNT(DISTINCT topic_id) = ` + selectedCount + `
			) matching ON matching.post_id = p.id
			` + postWhere + `
			ORDER BY ` + selectedOrderBy + `
			LIMIT ` + db.Arg(&args, MaxPostPageSize) + ` OFFSET ` + db.Arg(&args, offset)
	}

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := make([]Post, 0)
	for rows.Next() {
		var post Post
		var parentID sql.NullInt64
		var displayName sql.NullString
		var handle sql.NullString
		var userVote sql.NullString
		if err := rows.Scan(&post.ID, &parentID, &post.AuthorID, &displayName, &handle, &post.PostText, &post.CreatedAt, &post.Sum, &userVote, &post.SubPostCount); err != nil {
			continue
		}
		if userVote.Valid {
			v := userVote.String
			post.UserVote = &v
		}
		if parentID.Valid {
			v := uint(parentID.Int64)
			post.ParentPostID = &v
		}
		if displayName.Valid {
			post.AuthorDisplayName = displayName.String
		}
		if handle.Valid {
			post.AuthorHandle = handle.String
		}
		post.Topics, err = LoadPostTopics(conn, post.ID, userID, 0)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}

	return posts, nil
}

// LoadTopSubPosts loads a page of the top sub-posts of parentPostID, ordered
// by their own net vote sum. If selectedTopicIDs is non-empty, results are
// restricted to sub-posts that have all of the selected topics present. If
// cutoff is non-nil, only sub-posts created at or after cutoff are
// considered, and vote sums only reflect votes cast at or after cutoff. If mostRecent is
// true, sub-posts are ordered by creation time and ID, and cursor excludes
// posts at or before the last post.
func LoadTopSubPosts(conn *sql.DB, auth *ajax.Auth, parentPostID uint, offset uint, selectedTopicIDs []uint, cutoff *time.Time, mostRecent bool, cursor *PostCursor) ([]Post, error) {

	var userID *uint
	if auth != nil {
		userID = &auth.UserID
	}

	var args []interface{}
	var query string

	var userIDParam any
	if userID != nil {
		userIDParam = *userID
	}
	userArg := db.Arg(&args, userIDParam) + "::INTEGER"

	parentArg := db.Arg(&args, parentPostID)

	cutoffClause := ""
	if cutoff != nil {
		cutoffClause = " AND p.created_at >= " + db.Arg(&args, *cutoff)
	}
	if cursor != nil && mostRecent {
		cutoffClause += " AND (p.created_at, p.id) < (" + db.Arg(&args, cursor.CreatedAt) + ", " + db.Arg(&args, cursor.ID) + ")"
		offset = 0
	}
	orderBy := "vote_sum DESC, p.created_at DESC, p.id DESC"
	if mostRecent {
		orderBy = "p.created_at DESC, p.id DESC"
	}
	selectedOrderBy := orderBy

	if len(selectedTopicIDs) == 0 {
		query = `
			SELECT p.id, p.parent_post_id, p.author, u.display_name, u.handle, p.post_text, p.created_at,
				COALESCE(pvs.sum, 0) AS vote_sum, uv.vote_type,
				(SELECT COUNT(*) FROM post c WHERE c.parent_post_id = p.id) AS sub_post_count
			FROM post p
			LEFT JOIN user_account u ON u.id = p.author
			LEFT JOIN ` + filteredPostVoteSumTable(&args, cutoff) + ` pvs ON pvs.post_id = p.id
			LEFT JOIN post_vote uv ON uv.post_id = p.id AND uv.user_id = ` + userArg + `
			WHERE p.parent_post_id = ` + parentArg + cutoffClause + `
			ORDER BY ` + orderBy + `
			LIMIT ` + db.Arg(&args, MaxPostPageSize) + ` OFFSET ` + db.Arg(&args, offset)
	} else {
		selectedClause := db.In("topic_id", &args, selectedTopicIDs)
		selectedCount := db.Arg(&args, len(selectedTopicIDs))
		query = `
			SELECT p.id, p.parent_post_id, p.author, u.display_name, u.handle, p.post_text, p.created_at,
				COALESCE(pvs.sum, 0) AS vote_sum, uv.vote_type,
				(SELECT COUNT(*) FROM post c WHERE c.parent_post_id = p.id) AS sub_post_count
			FROM post p
			LEFT JOIN user_account u ON u.id = p.author
			LEFT JOIN ` + filteredPostVoteSumTable(&args, cutoff) + ` pvs ON pvs.post_id = p.id
			LEFT JOIN post_vote uv ON uv.post_id = p.id AND uv.user_id = ` + userArg + `
			JOIN (
				SELECT post_id
				FROM ` + filteredPostTopicSumTable(&args, cutoff) + `
				WHERE ` + selectedClause + ` AND sum >= 0
				GROUP BY post_id
				HAVING COUNT(DISTINCT topic_id) = ` + selectedCount + `
			) matching ON matching.post_id = p.id
			WHERE p.parent_post_id = ` + parentArg + cutoffClause + `
			ORDER BY ` + selectedOrderBy + `
			LIMIT ` + db.Arg(&args, MaxPostPageSize) + ` OFFSET ` + db.Arg(&args, offset)
	}

	rows, err := conn.Query(query, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	posts := make([]Post, 0)
	for rows.Next() {
		var post Post
		var parentID sql.NullInt64
		var displayName sql.NullString
		var handle sql.NullString
		var userVote sql.NullString
		if err := rows.Scan(&post.ID, &parentID, &post.AuthorID, &displayName, &handle, &post.PostText, &post.CreatedAt, &post.Sum, &userVote, &post.SubPostCount); err != nil {
			continue
		}
		if userVote.Valid {
			v := userVote.String
			post.UserVote = &v
		}
		if parentID.Valid {
			v := uint(parentID.Int64)
			post.ParentPostID = &v
		}
		if displayName.Valid {
			post.AuthorDisplayName = displayName.String
		}
		if handle.Valid {
			post.AuthorHandle = handle.String
		}
		post.Topics, err = LoadPostTopics(conn, post.ID, userID, 0)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}

	return posts, nil
}

// countTopPosts counts the total number of posts matching selectedTopicIDs
// (all of them must be present on a post for it to match; if empty, all
// posts match), optionally restricted to the sub-posts of scopePostID. If
// cutoff is non-nil, only posts created at or after cutoff are counted, and
// matching is based on votes cast at or after cutoff.
func countTopPosts(conn *sql.DB, selectedTopicIDs []uint, scopePostID *uint, cutoff *time.Time) (int, error) {

	var args []interface{}
	var query string

	var scopeClause string
	if scopePostID != nil {
		scopeClause = " AND p.parent_post_id = " + db.Arg(&args, *scopePostID)
	}
	var cutoffClause string
	if cutoff != nil {
		cutoffClause = " AND p.created_at >= " + db.Arg(&args, *cutoff)
	}
	whereClause := ""
	if scopeClause != "" || cutoffClause != "" {
		whereClause = "WHERE TRUE" + scopeClause + cutoffClause
	}

	if len(selectedTopicIDs) == 0 {
		query = `SELECT COUNT(*) FROM post p ` + whereClause
	} else {
		selectedClause := db.In("topic_id", &args, selectedTopicIDs)
		selectedCount := db.Arg(&args, len(selectedTopicIDs))
		query = `
			SELECT COUNT(*) FROM post p
			JOIN (
				SELECT post_id FROM ` + filteredPostTopicSumTable(&args, cutoff) + `
				WHERE ` + selectedClause + ` AND sum >= 0
				GROUP BY post_id
				HAVING COUNT(DISTINCT topic_id) = ` + selectedCount + `
			) matching ON matching.post_id = p.id
			` + whereClause
	}

	var total int
	if err := conn.QueryRow(query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("counting posts: %w", err)
	}
	return total, nil
}

// CountTopPosts counts the total number of the site's posts matching
// selectedTopicIDs (see LoadTopPosts).
func CountTopPosts(conn *sql.DB, selectedTopicIDs []uint, cutoff *time.Time) (int, error) {
	return countTopPosts(conn, selectedTopicIDs, nil, cutoff)
}

// CountTopSubPosts counts the total number of sub-posts of parentPostID
// matching selectedTopicIDs (see LoadTopSubPosts).
func CountTopSubPosts(conn *sql.DB, parentPostID uint, selectedTopicIDs []uint, cutoff *time.Time) (int, error) {
	return countTopPosts(conn, selectedTopicIDs, &parentPostID, cutoff)
}

func LoadPostTopics(db *sql.DB, postID uint, userID *uint, offset uint) ([]Topic, error) {
	var userIDParam any
	if userID != nil {
		userIDParam = *userID
	}

	rows, err := db.Query(`
		SELECT t.id, t.name, COALESCE(pts.sum, 0), ptv.vote_type
		FROM post_topic_sum pts
		JOIN topic t ON t.id = pts.topic_id
		LEFT JOIN post_topic_vote ptv
			ON ptv.post_id = pts.post_id AND ptv.topic_id = pts.topic_id AND ptv.user_id = $2
		WHERE pts.post_id = $1
		ORDER BY pts.sum DESC, t.name ASC
		LIMIT $3 OFFSET $4
	`, postID, userIDParam, MaxTopicPageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	topics := make([]Topic, 0)
	for rows.Next() {
		var topic Topic
		var sum int
		var voteType sql.NullString
		if err := rows.Scan(&topic.ID, &topic.Name, &sum, &voteType); err != nil {
			continue
		}
		topic.Sum = sum
		if voteType.Valid {
			v := voteType.String
			topic.UserVote = &v
		}
		topics = append(topics, topic)
	}
	return topics, nil
}

func LoadPost(db *sql.DB, postID uint, userID *uint) (*Post, error) {
	var post Post
	var parentID sql.NullInt64
	var displayName sql.NullString
	var handle sql.NullString

	err := db.QueryRow(`
		SELECT p.id, p.parent_post_id, p.author, u.display_name, u.handle, p.post_text, p.created_at,
			(SELECT COUNT(*) FROM post c WHERE c.parent_post_id = p.id)
		FROM post p
		LEFT JOIN user_account u ON u.id = p.author
		WHERE p.id = $1
	`, postID).Scan(&post.ID, &parentID, &post.AuthorID, &displayName, &handle, &post.PostText, &post.CreatedAt, &post.SubPostCount)
	if err != nil {
		return nil, fmt.Errorf("loading post %d: %w", postID, err)
	}

	if parentID.Valid {
		v := uint(parentID.Int64)
		post.ParentPostID = &v
	}
	if displayName.Valid {
		post.AuthorDisplayName = displayName.String
	}
	if handle.Valid {
		post.AuthorHandle = handle.String
	}

	post.Topics, err = LoadPostTopics(db, post.ID, userID, 0)
	if err != nil {
		return nil, fmt.Errorf("loading post topics for post %d: %w", post.ID, err)
	}
	post.Sum, post.UserVote, err = LoadPostVote(db, post.ID, userID)
	if err != nil {
		return nil, err
	}
	return &post, nil
}

func CreatePost(conn *sql.DB, parentPostID *uint, authorID uint, text string, topicNames []string) (*Post, error) {

	text = NormalizePostText(text)

	if err := ValidatePostText(text); err != nil {
		return nil, fmt.Errorf("validating post text: %w", err)
	}

	var postID uint

	err := db.InTransaction(conn, func(tx *sql.Tx) error {

		err := tx.QueryRow(`
			INSERT INTO post (parent_post_id, author, post_text, created_at)
			VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
			RETURNING id
		`, parentPostID, authorID, text).Scan(&postID)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(`
			INSERT INTO post_vote (post_id, user_id, vote_type, created_at)
			VALUES ($1, $2, 'upvote', CURRENT_TIMESTAMP)
		`, postID, authorID); err != nil {
			return err
		}

		if _, err := tx.Exec(`
			INSERT INTO post_vote_sum (post_id, upvotes, downvotes, sum, created_at)
			VALUES ($1, 1, 0, 1, CURRENT_TIMESTAMP)
		`, postID); err != nil {
			return err
		}

		for _, rawName := range topicNames {
			topicName := NormalizeTopicName(rawName)
			if topicName == "" {
				continue
			}
			if err := ValidateTopicName(topicName); err != nil {
				continue
			}
			if _, err := EnsureTopicOnPost(tx, postID, topicName, authorID); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("creating post: %w", err)
	}

	return LoadPost(conn, postID, &authorID)
}

// UpdatePostText updates the text of the post with postID, but only if
// authorID matches the post's author. Returns sql.ErrNoRows if the post
// doesn't exist or isn't owned by authorID.
func UpdatePostText(conn *sql.DB, postID uint, authorID uint, text string) (*Post, error) {

	text = NormalizePostText(text)

	if err := ValidatePostText(text); err != nil {
		return nil, fmt.Errorf("validating post text: %w", err)
	}

	result, err := conn.Exec(`
		UPDATE post SET post_text = $1 WHERE id = $2 AND author = $3
	`, text, postID, authorID)
	if err != nil {
		return nil, fmt.Errorf("updating post %d: %w", postID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("checking rows affected for post %d: %w", postID, err)
	}
	if rowsAffected == 0 {
		return nil, sql.ErrNoRows
	}

	return LoadPost(conn, postID, &authorID)
}

func EnsureTopicOnPost(conn db.DBConn, postID uint, topicName string, createdBy uint) (Topic, error) {

	topicName = NormalizeTopicName(topicName)

	if err := ValidateTopicName(topicName); err != nil {
		return Topic{}, err
	}

	topicExists, err := CheckTopicExists(conn, topicName)
	if err != nil {
		return Topic{}, fmt.Errorf("checking if topic exists: %w", err)
	}
	var topic Topic
	if topicExists {
		err = conn.QueryRow(`SELECT id, name FROM topic WHERE name = $1`, topicName).Scan(&topic.ID, &topic.Name)
		if err != nil {
			return Topic{}, fmt.Errorf("querying topic by name: %w", err)
		}
	} else {
		createdTopic, err := CreateTopic(conn, topicName, createdBy)
		if err != nil {
			return Topic{}, err
		}
		topic = *createdTopic
	}

	var topicRowExists bool
	err = conn.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM post_topic_sum WHERE post_id = $1 AND topic_id = $2
		)
	`, postID, topic.ID).Scan(&topicRowExists)
	if err != nil {
		return Topic{}, fmt.Errorf("checking if topic row exists: %w", err)
	}
	if !topicRowExists {
		_, err = conn.Exec(`
			INSERT INTO post_topic_sum (post_id, topic_id, upvotes, downvotes, sum, created_at)
			VALUES ($1, $2, 0, 0, 0, CURRENT_TIMESTAMP)
		`, postID, topic.ID)
		if err != nil {
			return Topic{}, fmt.Errorf("inserting topic row: %w", err)
		}
	}

	// Auto-upvote the topic on behalf of the user adding it, whether it's
	// newly added to the post or already there.
	if err := ensureUserUpvote(conn, postID, topic.ID, createdBy); err != nil {
		return Topic{}, err
	}

	err = conn.QueryRow(`
		SELECT pts.sum, ptv.vote_type FROM post_topic_sum pts
		LEFT JOIN post_topic_vote ptv
			ON ptv.post_id = pts.post_id AND ptv.topic_id = pts.topic_id AND ptv.user_id = $3
		WHERE pts.post_id = $1 AND pts.topic_id = $2
	`, postID, topic.ID, createdBy).Scan(&topic.Sum, &topic.UserVote)
	if err != nil {
		return Topic{}, fmt.Errorf("loading topic sum after upvote: %w", err)
	}

	return topic, nil
}

// ensureUserUpvote makes sure userID has an upvote recorded against topicID
// on postID, inserting one and updating the topic's vote sum if they don't
// already have any vote there.
func ensureUserUpvote(conn db.DBConn, postID uint, topicID uint, userID uint) error {
	var voteExists bool
	if err := conn.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM post_topic_vote WHERE post_id = $1 AND topic_id = $2 AND user_id = $3
		)
	`, postID, topicID, userID).Scan(&voteExists); err != nil {
		return fmt.Errorf("checking existing vote: %w", err)
	}
	if voteExists {
		return nil
	}

	if _, err := conn.Exec(`
		INSERT INTO post_topic_vote (post_id, user_id, topic_id, vote_type, created_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
	`, postID, userID, topicID, VoteTypeUpvote); err != nil {
		return fmt.Errorf("auto-upvoting topic: %w", err)
	}
	if _, err := conn.Exec(`
		UPDATE post_topic_sum SET
			upvotes = upvotes + 1,
			sum = sum + 1
		WHERE post_id = $1 AND topic_id = $2
	`, postID, topicID); err != nil {
		return fmt.Errorf("updating topic sum after auto-upvote: %w", err)
	}
	return nil
}
