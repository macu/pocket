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

func LoadTopPosts(db *sql.DB, auth *ajax.Auth, offset uint) ([]Post, error) {

	// offset is uint, so it cannot be negative

	var userID *uint
	if auth != nil {
		userID = &auth.UserID
	}

	rows, err := db.Query(`
		SELECT p.id, p.parent_post_id, p.author, u.display_name, p.post_text, p.created_at,
			COALESCE(SUM(CASE WHEN pts.sum IS NULL THEN 0 ELSE pts.sum END), 0) AS total_topic_score
		FROM post p
		LEFT JOIN user_account u ON u.id = p.author
		LEFT JOIN post_topic_sum pts ON pts.post_id = p.id
		GROUP BY p.id, p.parent_post_id, p.author, u.display_name, p.post_text, p.created_at
		ORDER BY total_topic_score DESC, p.created_at DESC
		LIMIT $1 OFFSET $2
	`, MaxPostPageSize, offset)
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
		post.Topics, err = LoadPostTopics(db, post.ID, userID, 0)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}

	return posts, nil
}

func LoadTopSubPosts(db *sql.DB, auth *ajax.Auth, parentPostID uint, offset uint) ([]Post, error) {

	var userID *uint
	if auth != nil {
		userID = &auth.UserID
	}

	rows, err := db.Query(`
		SELECT p.id, p.parent_post_id, p.author, u.display_name, p.post_text, p.created_at,
			COALESCE(SUM(CASE WHEN pts.sum IS NULL THEN 0 ELSE pts.sum END), 0) AS total_topic_score
		FROM post p
		LEFT JOIN user_account u ON u.id = p.author
		LEFT JOIN post_topic_sum pts ON pts.post_id = p.id
		WHERE p.parent_post_id = $1
		GROUP BY p.id, p.parent_post_id, p.author, u.display_name, p.post_text, p.created_at
		ORDER BY total_topic_score DESC, p.created_at DESC
		LIMIT $2 OFFSET $3
	`, parentPostID, MaxPostPageSize, offset)
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
		post.Topics, err = LoadPostTopics(db, post.ID, userID, 0)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}

	return posts, nil
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

func LoadPostTotalTopicScore(conn db.DBConn, postID uint) (int, error) {
	var total int
	err := conn.QueryRow(`
		SELECT COALESCE(SUM(sum), 0) FROM post_topic_sum WHERE post_id = $1
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
