package pocket

import (
	"database/sql"
	"fmt"
	"pocket/pkg/utils/ajax"
	"strconv"
	"strings"
)

type Topic struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Sum  int    `json:"sum"`
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

func CheckTopicExists(db *sql.DB, name string) (bool, error) {
	var exists bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM topic WHERE name = $1)`, name).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func LoadTopTopics(db *sql.DB, auth *ajax.Auth, offset uint) ([]Topic, error) {

	var topics []Topic

	pageSize := MAX_TOPIC_PAGE_SIZE

	rows, err := db.Query(`SELECT id, name,
		COALESCE(post_topic_sum.sum, 0)
		FROM topic
		LEFT JOIN post_topic_sum ON topic.id = post_topic_sum.topic_id
		ORDER BY id DESC
		LIMIT $1 OFFSET $2`,
		pageSize,
		offset,
	)
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

func CreateTopic(db *sql.DB, name string, createdBy uint) (*Topic, error) {
	var topic = &Topic{}

	// Normalize the topic name
	name = NormalizeTopicName(name)

	// Validate
	if err := ValidateTopicName(name); err != nil {
		return nil, err
	}

	// Check if exists
	if exists, err := CheckTopicExists(db, name); err != nil {
		return nil, err
	} else if exists {
		return nil, fmt.Errorf("topic with name %s already exists", name)
	}

	err := db.QueryRow(`INSERT INTO topic (name, created_by, created_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		RETURNING id, name`,
		name, createdBy,
	).Scan(&topic.ID, &topic.Name)
	if err != nil {
		return nil, err
	}

	return topic, nil
}
