package pocket

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"pocket/pkg/utils/ajax"
	"pocket/pkg/utils/db"
)

type Topic struct {
	ID       uint    `json:"id"`
	Name     string  `json:"name"`
	Sum      int     `json:"sum"`
	UserVote *string `json:"userVote"`
}

func NormalizeTopicName(name string) string {
	return strings.Join(strings.Fields(name), " ")
}

func ValidateTopicName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("topic name cannot be empty")
	}
	// check for non-printable characters
	for _, r := range name {
		if !strconv.IsPrint(r) {
			return fmt.Errorf("topic name contains non-printable characters")
		}
	}
	return nil
}

func ParseTopicNames(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := NormalizeTopicName(part)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

// ParseTopicNamesJSON parses a JSON-encoded array of topic name strings.
func ParseTopicNamesJSON(raw string) []string {
	if raw == "" {
		return nil
	}
	var parts []string
	if err := json.Unmarshal([]byte(raw), &parts); err != nil {
		return nil
	}
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := NormalizeTopicName(part)
		if item != "" {
			items = append(items, item)
		}
	}
	return items
}

func CheckTopicExists(conn db.DBConn, name string) (bool, error) {
	var exists bool
	err := conn.QueryRow(`SELECT EXISTS(SELECT 1 FROM topic WHERE name = $1)`, name).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func LoadTopTopics(db *sql.DB, auth *ajax.Auth, offset uint) ([]Topic, error) {

	var topics []Topic
	pageSize := MaxTopicPageSize

	rows, err := db.Query(`
		SELECT t.id, t.name, COALESCE(SUM(pts.sum), 0) AS total_sum
		FROM topic t
		LEFT JOIN post_topic_sum pts ON pts.topic_id = t.id
		GROUP BY t.id, t.name
		ORDER BY total_sum DESC, t.name ASC
		LIMIT $1 OFFSET $2
	`, pageSize, offset)
	if err != nil {
		return topics, fmt.Errorf("loading top topics: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var topic Topic
		if err := rows.Scan(&topic.ID, &topic.Name, &topic.Sum); err != nil {
			return nil, fmt.Errorf("scanning topic row: %w", err)
		}
		topics = append(topics, topic)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating topic rows: %w", err)
	}

	return topics, nil
}

func SetPostTopicVote(conn *sql.DB, postID uint, topicID uint, userID uint, voteType string) (*Topic, error) {

	if !IsValidVote(voteType) {
		return nil, fmt.Errorf("invalid vote type: %s", voteType)
	}

	var topic Topic
	var userVote *string

	err := db.InTransaction(conn, func(tx *sql.Tx) error {

		var sumExists bool
		if err := tx.QueryRow(`
			SELECT EXISTS(SELECT 1 FROM post_topic_sum WHERE post_id = $1 AND topic_id = $2)
		`, postID, topicID).Scan(&sumExists); err != nil {
			return fmt.Errorf("checking topic on post: %w", err)
		}
		if !sumExists {
			return fmt.Errorf("topic %d is not associated with post %d", topicID, postID)
		}

		var existingVote sql.NullString
		err := tx.QueryRow(`
			SELECT vote_type FROM post_topic_vote WHERE post_id = $1 AND user_id = $2 AND topic_id = $3
		`, postID, userID, topicID).Scan(&existingVote)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("loading existing vote: %w", err)
		}

		if existingVote.Valid && existingVote.String == voteType {
			if _, err := tx.Exec(`
				DELETE FROM post_topic_vote WHERE post_id = $1 AND user_id = $2 AND topic_id = $3
			`, postID, userID, topicID); err != nil {
				return fmt.Errorf("deleting vote: %w", err)
			}
			userVote = nil
		} else {
			if existingVote.Valid {
				if _, err := tx.Exec(`
					UPDATE post_topic_vote SET vote_type = $4, created_at = CURRENT_TIMESTAMP
					WHERE post_id = $1 AND user_id = $2 AND topic_id = $3
				`, postID, userID, topicID, voteType); err != nil {
					return fmt.Errorf("updating vote: %w", err)
				}
			} else {
				if _, err := tx.Exec(`
					INSERT INTO post_topic_vote (post_id, user_id, topic_id, vote_type, created_at)
					VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
				`, postID, userID, topicID, voteType); err != nil {
					return fmt.Errorf("inserting vote: %w", err)
				}
			}
			v := voteType
			userVote = &v
		}

		if _, err := tx.Exec(`
			UPDATE post_topic_sum SET
				upvotes = (SELECT COUNT(*) FROM post_topic_vote WHERE post_id = $1 AND topic_id = $2 AND vote_type = 'upvote'),
				downvotes = (SELECT COUNT(*) FROM post_topic_vote WHERE post_id = $1 AND topic_id = $2 AND vote_type = 'downvote'),
				sum = (
					SELECT COUNT(*) FILTER (WHERE vote_type = 'upvote') - COUNT(*) FILTER (WHERE vote_type = 'downvote')
					FROM post_topic_vote WHERE post_id = $1 AND topic_id = $2
				)
			WHERE post_id = $1 AND topic_id = $2
		`, postID, topicID); err != nil {
			return fmt.Errorf("updating topic sum: %w", err)
		}

		if err := tx.QueryRow(`
			SELECT t.id, t.name, pts.sum FROM post_topic_sum pts
			JOIN topic t ON t.id = pts.topic_id
			WHERE pts.post_id = $1 AND pts.topic_id = $2
		`, postID, topicID).Scan(&topic.ID, &topic.Name, &topic.Sum); err != nil {
			return fmt.Errorf("loading updated topic: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	topic.UserVote = userVote
	return &topic, nil

}

func RemovePostTopicVote(conn *sql.DB, postID uint, topicID uint, userID uint) (*Topic, error) {

	var topic Topic

	err := db.InTransaction(conn, func(tx *sql.Tx) error {

		var sumExists bool
		if err := tx.QueryRow(`
			SELECT EXISTS(SELECT 1 FROM post_topic_sum WHERE post_id = $1 AND topic_id = $2)
		`, postID, topicID).Scan(&sumExists); err != nil {
			return fmt.Errorf("checking topic on post: %w", err)
		}
		if !sumExists {
			return fmt.Errorf("topic %d is not associated with post %d", topicID, postID)
		}

		if _, err := tx.Exec(`
			DELETE FROM post_topic_vote WHERE post_id = $1 AND user_id = $2 AND topic_id = $3
		`, postID, userID, topicID); err != nil {
			return fmt.Errorf("deleting vote: %w", err)
		}

		if _, err := tx.Exec(`
			UPDATE post_topic_sum SET
				upvotes = (SELECT COUNT(*) FROM post_topic_vote WHERE post_id = $1 AND topic_id = $2 AND vote_type = 'upvote'),
				downvotes = (SELECT COUNT(*) FROM post_topic_vote WHERE post_id = $1 AND topic_id = $2 AND vote_type = 'downvote'),
				sum = (
					SELECT COUNT(*) FILTER (WHERE vote_type = 'upvote') - COUNT(*) FILTER (WHERE vote_type = 'downvote')
					FROM post_topic_vote WHERE post_id = $1 AND topic_id = $2
				)
			WHERE post_id = $1 AND topic_id = $2
		`, postID, topicID); err != nil {
			return fmt.Errorf("updating topic sum: %w", err)
		}

		if err := tx.QueryRow(`
			SELECT t.id, t.name, pts.sum FROM post_topic_sum pts
			JOIN topic t ON t.id = pts.topic_id
			WHERE pts.post_id = $1 AND pts.topic_id = $2
		`, postID, topicID).Scan(&topic.ID, &topic.Name, &topic.Sum); err != nil {
			return fmt.Errorf("loading updated topic: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &topic, nil

}

func CreateTopic(conn db.DBConn, name string, createdBy uint) (*Topic, error) {
	var topic = &Topic{}

	// Normalize the topic name
	name = NormalizeTopicName(name)

	// Validate
	if err := ValidateTopicName(name); err != nil {
		return nil, err
	}

	// Check if exists
	if exists, err := CheckTopicExists(conn, name); err != nil {
		return nil, err
	} else if exists {
		return nil, fmt.Errorf("topic with name %s already exists", name)
	}

	err := conn.QueryRow(`INSERT INTO topic (name, created_by, created_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		RETURNING id, name`,
		name, createdBy,
	).Scan(&topic.ID, &topic.Name)
	if err != nil {
		return nil, err
	}

	return topic, nil
}
