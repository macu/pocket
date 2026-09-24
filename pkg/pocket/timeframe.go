package pocket

import (
	"fmt"
	"time"

	"pocket/pkg/utils/db"
)

// ParseTimeframe converts a timeframe value ("24h", "7d", "30d", "all", or
// "") into the cutoff time before which votes/posts should be excluded from
// topic/post filtering. A nil return value means no cutoff (all time), which
// is the default when raw is empty.
func ParseTimeframe(raw string) (*time.Time, error) {
	var duration time.Duration
	switch raw {
	case "", "all":
		return nil, nil
	case "24h":
		duration = 24 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	case "30d":
		duration = 30 * 24 * time.Hour
	default:
		return nil, fmt.Errorf("invalid timeframe: %s", raw)
	}
	cutoff := time.Now().Add(-duration)
	return &cutoff, nil
}

// filteredPostTopicSumTable returns a SQL expression usable in place of the
// post_topic_sum table, yielding the same (post_id, topic_id, sum) columns.
// If cutoff is non-nil, the sum is recomputed from votes cast at or after
// cutoff, and rows are limited to posts created at or after cutoff too, so
// that timeframe-scoped topic/post filtering reflects only recent activity.
// If cutoff is nil, the cached post_topic_sum table is used as-is.
func filteredPostTopicSumTable(args *[]interface{}, cutoff *time.Time) string {
	if cutoff == nil {
		return "post_topic_sum"
	}
	cutoffArg := db.Arg(args, *cutoff)
	return `(
		SELECT pts.post_id, pts.topic_id,
			COUNT(*) FILTER (WHERE v.vote_type = 'upvote') - COUNT(*) FILTER (WHERE v.vote_type = 'downvote') AS sum
		FROM post_topic_sum pts
		JOIN post p ON p.id = pts.post_id
		LEFT JOIN post_topic_vote v
			ON v.post_id = pts.post_id AND v.topic_id = pts.topic_id AND v.created_at >= ` + cutoffArg + `
		WHERE p.created_at >= ` + cutoffArg + `
		GROUP BY pts.post_id, pts.topic_id
	)`
}
