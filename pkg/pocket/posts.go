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
	PostText          string    `json:"postText"`
	CreatedAt         time.Time `json:"createdAt"`
	Topics            []Topic   `json:"topics,omitempty"`
	TotalTopicScore   int       `json:"totalTopicScore"`
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

// LoadTopPosts loads a page of the site's top posts, ordered by total topic
// score (sum of net votes across topics with a positive individual sum;
// downvoted topics are excluded so they can't drag a post's score down). If
// selectedTopicIDs is non-empty, results are restricted to posts that have
// all of the selected topics present, ordered by the same positive-sum-only
// total across just those topics.
func LoadTopPosts(conn *sql.DB, auth *ajax.Auth, offset uint, selectedTopicIDs []uint) ([]Post, error) {

	// offset is uint, so it cannot be negative

	var userID *uint
	if auth != nil {
		userID = &auth.UserID
	}

	var args []interface{}
	var query string

	if len(selectedTopicIDs) == 0 {
		query = `
			SELECT p.id, p.parent_post_id, p.author, u.display_name, p.post_text, p.created_at,
				COALESCE(SUM(CASE WHEN pts.sum > 0 THEN pts.sum ELSE 0 END), 0) AS total_topic_score
			FROM post p
			LEFT JOIN user_account u ON u.id = p.author
			LEFT JOIN post_topic_sum pts ON pts.post_id = p.id
			GROUP BY p.id, p.parent_post_id, p.author, u.display_name, p.post_text, p.created_at
			ORDER BY total_topic_score DESC, p.created_at DESC
			LIMIT ` + db.Arg(&args, MaxPostPageSize) + ` OFFSET ` + db.Arg(&args, offset)
	} else {
		selectedClause := db.In("topic_id", &args, selectedTopicIDs)
		selectedCount := db.Arg(&args, len(selectedTopicIDs))
		query = `
			SELECT p.id, p.parent_post_id, p.author, u.display_name, p.post_text, p.created_at,
				topic_scores.selected_sum AS total_topic_score
			FROM post p
			LEFT JOIN user_account u ON u.id = p.author
			JOIN (
				SELECT post_id, SUM(CASE WHEN sum > 0 THEN sum ELSE 0 END) AS selected_sum
				FROM post_topic_sum
				WHERE ` + selectedClause + ` AND sum >= 0
				GROUP BY post_id
				HAVING COUNT(DISTINCT topic_id) = ` + selectedCount + `
			) topic_scores ON topic_scores.post_id = p.id
			ORDER BY topic_scores.selected_sum DESC, p.created_at DESC
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
		if err := rows.Scan(&post.ID, &parentID, &post.AuthorID, &displayName, &post.PostText, &post.CreatedAt, &post.TotalTopicScore); err != nil {
			continue
		}
		if parentID.Valid {
			v := uint(parentID.Int64)
			post.ParentPostID = &v
		}
		if displayName.Valid {
			post.AuthorDisplayName = displayName.String
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
// by total topic score (sum of net votes across topics with a positive
// individual sum; downvoted topics are excluded so they can't drag a post's
// score down). If selectedTopicIDs is non-empty, results are restricted to
// sub-posts that have all of the selected topics present, ordered by the
// same positive-sum-only total across just those topics.
func LoadTopSubPosts(conn *sql.DB, auth *ajax.Auth, parentPostID uint, offset uint, selectedTopicIDs []uint) ([]Post, error) {

	var userID *uint
	if auth != nil {
		userID = &auth.UserID
	}

	var args []interface{}
	var query string

	parentArg := db.Arg(&args, parentPostID)

	if len(selectedTopicIDs) == 0 {
		query = `
			SELECT p.id, p.parent_post_id, p.author, u.display_name, p.post_text, p.created_at,
				COALESCE(SUM(CASE WHEN pts.sum > 0 THEN pts.sum ELSE 0 END), 0) AS total_topic_score
			FROM post p
			LEFT JOIN user_account u ON u.id = p.author
			LEFT JOIN post_topic_sum pts ON pts.post_id = p.id
			WHERE p.parent_post_id = ` + parentArg + `
			GROUP BY p.id, p.parent_post_id, p.author, u.display_name, p.post_text, p.created_at
			ORDER BY total_topic_score DESC, p.created_at DESC
			LIMIT ` + db.Arg(&args, MaxPostPageSize) + ` OFFSET ` + db.Arg(&args, offset)
	} else {
		selectedClause := db.In("topic_id", &args, selectedTopicIDs)
		selectedCount := db.Arg(&args, len(selectedTopicIDs))
		query = `
			SELECT p.id, p.parent_post_id, p.author, u.display_name, p.post_text, p.created_at,
				topic_scores.selected_sum AS total_topic_score
			FROM post p
			LEFT JOIN user_account u ON u.id = p.author
			JOIN (
				SELECT post_id, SUM(CASE WHEN sum > 0 THEN sum ELSE 0 END) AS selected_sum
				FROM post_topic_sum
				WHERE ` + selectedClause + ` AND sum >= 0
				GROUP BY post_id
				HAVING COUNT(DISTINCT topic_id) = ` + selectedCount + `
			) topic_scores ON topic_scores.post_id = p.id
			WHERE p.parent_post_id = ` + parentArg + `
			ORDER BY topic_scores.selected_sum DESC, p.created_at DESC
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
		if err := rows.Scan(&post.ID, &parentID, &post.AuthorID, &displayName, &post.PostText, &post.CreatedAt, &post.TotalTopicScore); err != nil {
			continue
		}
		if parentID.Valid {
			v := uint(parentID.Int64)
			post.ParentPostID = &v
		}
		if displayName.Valid {
			post.AuthorDisplayName = displayName.String
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
// posts match), optionally restricted to the sub-posts of scopePostID.
func countTopPosts(conn *sql.DB, selectedTopicIDs []uint, scopePostID *uint) (int, error) {

	var args []interface{}
	var query string

	var scopeWhere string
	if scopePostID != nil {
		scopeWhere = "WHERE p.parent_post_id = " + db.Arg(&args, *scopePostID)
	}

	if len(selectedTopicIDs) == 0 {
		query = `SELECT COUNT(*) FROM post p ` + scopeWhere
	} else {
		selectedClause := db.In("topic_id", &args, selectedTopicIDs)
		selectedCount := db.Arg(&args, len(selectedTopicIDs))
		query = `
			SELECT COUNT(*) FROM post p
			JOIN (
				SELECT post_id FROM post_topic_sum
				WHERE ` + selectedClause + ` AND sum >= 0
				GROUP BY post_id
				HAVING COUNT(DISTINCT topic_id) = ` + selectedCount + `
			) matching ON matching.post_id = p.id
			` + scopeWhere
	}

	var total int
	if err := conn.QueryRow(query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("counting posts: %w", err)
	}
	return total, nil
}

// CountTopPosts counts the total number of the site's posts matching
// selectedTopicIDs (see LoadTopPosts).
func CountTopPosts(conn *sql.DB, selectedTopicIDs []uint) (int, error) {
	return countTopPosts(conn, selectedTopicIDs, nil)
}

// CountTopSubPosts counts the total number of sub-posts of parentPostID
// matching selectedTopicIDs (see LoadTopSubPosts).
func CountTopSubPosts(conn *sql.DB, parentPostID uint, selectedTopicIDs []uint) (int, error) {
	return countTopPosts(conn, selectedTopicIDs, &parentPostID)
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

// LoadPostTotalTopicScore sums the post's topics that have a positive
// individual sum, so a downvoted topic can't drag the post's score down.
func LoadPostTotalTopicScore(conn db.DBConn, postID uint) (int, error) {
	var total int
	err := conn.QueryRow(`
		SELECT COALESCE(SUM(CASE WHEN sum > 0 THEN sum ELSE 0 END), 0) FROM post_topic_sum WHERE post_id = $1
	`, postID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("loading total topic score for post %d: %w", postID, err)
	}
	return total, nil
}

func LoadPost(db *sql.DB, postID uint, userID *uint) (*Post, error) {
	var post Post
	var parentID sql.NullInt64
	var displayName sql.NullString

	err := db.QueryRow(`
		SELECT p.id, p.parent_post_id, p.author, u.display_name, p.post_text, p.created_at
		FROM post p
		LEFT JOIN user_account u ON u.id = p.author
		WHERE p.id = $1
	`, postID).Scan(&post.ID, &parentID, &post.AuthorID, &displayName, &post.PostText, &post.CreatedAt)
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

	post.Topics, err = LoadPostTopics(db, post.ID, userID, 0)
	if err != nil {
		return nil, fmt.Errorf("loading post topics for post %d: %w", post.ID, err)
	}
	post.TotalTopicScore, err = LoadPostTotalTopicScore(db, post.ID)
	if err != nil {
		return nil, fmt.Errorf("loading total topic score for post %d: %w", post.ID, err)
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

	return topic, nil
}
