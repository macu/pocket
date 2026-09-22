package pocket

const MaxTopicPageSize = 20
const MaxTopicLength = 50
const MaxTopicSearchResults = 10

const MaxPostPageSize = 20
const MaxPostLength = 1024

const (
	VoteTypeUpvote   = "upvote"
	VoteTypeDownvote = "downvote"
)

func IsValidVote(voteType string) bool {
	return voteType == VoteTypeUpvote || voteType == VoteTypeDownvote
}
