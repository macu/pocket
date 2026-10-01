package pocket

const MaxTopicPageSize = 20
const MaxTopicLength = 50
const MaxTopicSearchResults = 10
const MaxTopicSelectionCount = 20

const MaxPostPageSize = 20
const MaxPostLength = 1024
const MaxNewPostTopics = 20

const (
	VoteTypeUpvote   = "upvote"
	VoteTypeDownvote = "downvote"
)

func IsValidVote(voteType string) bool {
	return voteType == VoteTypeUpvote || voteType == VoteTypeDownvote
}

// LimitTopicSelection caps a slice of topic IDs (e.g. from a search/filter
// request) to MaxTopicSelectionCount.
func LimitTopicSelection(topicIDs []uint) []uint {
	if len(topicIDs) > MaxTopicSelectionCount {
		return topicIDs[:MaxTopicSelectionCount]
	}
	return topicIDs
}
