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
	ID        uint    `json:"id"`
	Name      string  `json:"name"`
	Sum       int     `json:"sum"`
	PostCount int     `json:"postCount"`
	UserVote  *string `json:"userVote"`
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

// escapeLikePattern escapes the LIKE/ILIKE wildcard characters in s so it can
// be safely embedded in a '%...%' search pattern.
func escapeLikePattern(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(s)
}

// SearchTopics finds up to MaxTopicSearchResults topics whose name contains
// query (case-insensitive), ranked by the number of posts they're on with a
// non-negative sum (i.e. not net-downvoted there), then by name.
func SearchTopics(conn *sql.DB, query string) ([]Topic, error) {

	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	rows, err := conn.Query(`
		SELECT t.id, t.name, COALESCE(SUM(pts.sum), 0) AS total_sum,
			COUNT(DISTINCT CASE WHEN pts.sum >= 0 THEN pts.post_id END) AS post_count
		FROM topic t
		LEFT JOIN post_topic_sum pts ON pts.topic_id = t.id
		WHERE t.name ILIKE '%' || $1 || '%' ESCAPE '\'
		GROUP BY t.id, t.name
		ORDER BY post_count DESC, t.name ASC
		LIMIT $2
	`, escapeLikePattern(query), MaxTopicSearchResults)
	if err != nil {
		return nil, fmt.Errorf("searching topics: %w", err)
	}
	defer rows.Close()

	topics := make([]Topic, 0)
	for rows.Next() {
		var topic Topic
		if err := rows.Scan(&topic.ID, &topic.Name, &topic.Sum, &topic.PostCount); err != nil {
			return nil, fmt.Errorf("scanning topic row: %w", err)
		}
		topics = append(topics, topic)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating topic rows: %w", err)
	}

	return topics, nil
}

// loadTopicsPage loads a page of topics ordered by total sum (net votes),
// along with each topic's post count (the number of posts where the topic
// doesn't have a negative individual sum, i.e. isn't net-downvoted there).
// If scopePostID is non-nil, results are restricted to topics tagged on the
// sub-posts of that post (rather than all posts). If selectedTopicIDs is
// non-empty, the selected topics are excluded, the results are restricted to
// topics that are co-present with the selected topics on the same posts
// (within scopePostID's sub-posts, if scoped), and each topic's sum/post
// count are limited to those same posts (i.e. posts matching the selected
// topics plus that topic), not the topic's totals across all posts.
func loadTopicsPage(conn *sql.DB, offset uint, selectedTopicIDs []uint, scopePostID *uint) ([]Topic, error) {

	var topics []Topic
	pageSize := MaxTopicPageSize

	var args []interface{}
	var query string

	var scopeArg string
	if scopePostID != nil {
		scopeArg = db.Arg(&args, *scopePostID)
	}

	if len(selectedTopicIDs) == 0 {
		topicJoin := "LEFT JOIN post_topic_sum pts ON pts.topic_id = t.id"
		scopeJoin := ""
		scopeWhere := ""
		if scopePostID != nil {
			// only topics actually tagged on a sub-post are of interest, so use an inner join
			topicJoin = "JOIN post_topic_sum pts ON pts.topic_id = t.id"
			scopeJoin = "JOIN post p ON p.id = pts.post_id"
			scopeWhere = "WHERE p.parent_post_id = " + scopeArg
		}
		query = `
			SELECT t.id, t.name, COALESCE(SUM(pts.sum), 0) AS total_sum,
				COUNT(DISTINCT CASE WHEN pts.sum >= 0 THEN pts.post_id END) AS post_count
			FROM topic t
			` + topicJoin + `
			` + scopeJoin + `
			` + scopeWhere + `
			GROUP BY t.id, t.name
			HAVING COUNT(DISTINCT CASE WHEN pts.sum >= 0 THEN pts.post_id END) > 0
			ORDER BY COALESCE(SUM(pts.sum), 0) DESC, t.name ASC
			LIMIT ` + db.Arg(&args, pageSize) + ` OFFSET ` + db.Arg(&args, offset)
	} else {
		selectedClause := db.In("topic_id", &args, selectedTopicIDs)
		excludeClause := db.In("t.id", &args, selectedTopicIDs)
		selectedCount := db.Arg(&args, len(selectedTopicIDs))

		selfScopeJoin := ""
		selfScopeWhere := ""
		if scopePostID != nil {
			selfScopeJoin = "JOIN post p_self ON p_self.id = pts_self.post_id"
			selfScopeWhere = "AND p_self.parent_post_id = " + scopeArg
		}

		// topic_totals is restricted to the posts that also match all of the
		// selected topics, so the sum/count reflect the selected topics plus
		// this candidate topic, not this topic's totals across all posts.
		query = `
			SELECT t.id, t.name, COALESCE(topic_totals.total_sum, 0), COALESCE(topic_totals.post_count, 0)
			FROM topic t
			JOIN (
				SELECT pts_self.topic_id, SUM(pts_self.sum) AS total_sum,
					COUNT(DISTINCT pts_self.post_id) AS post_count
				FROM post_topic_sum pts_self
				` + selfScopeJoin + `
				WHERE pts_self.sum >= 0
				` + selfScopeWhere + `
				AND pts_self.post_id IN (
					SELECT post_id FROM post_topic_sum
					WHERE ` + selectedClause + ` AND sum >= 0
					GROUP BY post_id
					HAVING COUNT(DISTINCT topic_id) = ` + selectedCount + `
				)
				GROUP BY pts_self.topic_id
			) topic_totals ON topic_totals.topic_id = t.id
			WHERE NOT (` + excludeClause + `)
			ORDER BY COALESCE(topic_totals.total_sum, 0) DESC, t.name ASC
			LIMIT ` + db.Arg(&args, pageSize) + ` OFFSET ` + db.Arg(&args, offset)
	}

	rows, err := conn.Query(query, args...)
	if err != nil {
		return topics, fmt.Errorf("loading topics: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var topic Topic
		if err := rows.Scan(&topic.ID, &topic.Name, &topic.Sum, &topic.PostCount); err != nil {
			return nil, fmt.Errorf("scanning topic row: %w", err)
		}
		topics = append(topics, topic)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating topic rows: %w", err)
	}

	return topics, nil
}

// LoadTopTopics loads a page of the site's top topics, ordered by total sum
// (net votes) across all posts. If selectedTopicIDs is non-empty, the
// selected topics are excluded and the results are restricted to topics that
// are co-present with the selected topics on the same posts (i.e. posts that
// have all of the selected topics present).
func LoadTopTopics(conn *sql.DB, auth *ajax.Auth, offset uint, selectedTopicIDs []uint) ([]Topic, error) {
	return loadTopicsPage(conn, offset, selectedTopicIDs, nil)
}

// LoadSpaceTopics loads a page of the top topics tagged on the sub-posts of
// parentPostID, ordered by total sum (net votes) across those sub-posts. If
// selectedTopicIDs is non-empty, the selected topics are excluded and the
// results are restricted to topics that are co-present with the selected
// topics on the same sub-posts (i.e. sub-posts that have all of the selected
// topics present).
func LoadSpaceTopics(conn *sql.DB, parentPostID uint, offset uint, selectedTopicIDs []uint) ([]Topic, error) {
	return loadTopicsPage(conn, offset, selectedTopicIDs, &parentPostID)
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
