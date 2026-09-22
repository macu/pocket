package ajax

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"pocket/pkg/pocket"
	"pocket/pkg/utils/ajax"
	"pocket/pkg/utils/logging"
	"pocket/pkg/utils/types"
)

func AjaxLoadPost(db *sql.DB, auth *ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {

	id, err := types.AtoUint(r.FormValue("id"))
	if err != nil {
		return nil, http.StatusBadRequest
	}

	loadContent := types.AtoBool(r.FormValue("loadContent"))

	var userID *uint
	if auth != nil {
		userID = &auth.UserID
	}

	post, err := pocket.LoadPost(db, uint(id), userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, http.StatusNotFound
		}
		logging.LogError(r, auth, err)
		return nil, http.StatusInternalServerError
	}

	var parentPost any
	if post.ParentPostID != nil {
		parent, err := pocket.LoadPost(db, *post.ParentPostID, userID)
		if err != nil && err != sql.ErrNoRows {
			logging.LogError(r, auth, err)
			return nil, http.StatusInternalServerError
		}
		if err == nil {
			parentPost = parent
		}
	}

	payload := map[string]any{
		"post":       post,
		"parentPost": parentPost,
	}

	if loadContent {
		payload["topTopics"] = post.Topics
		topSubPosts, err := pocket.LoadTopSubPosts(db, auth, id, 0)
		if err != nil {
			logging.LogError(r, auth, err)
			return nil, http.StatusInternalServerError
		}
		payload["topSubPosts"] = topSubPosts
	}

	return payload, http.StatusOK

}

func AjaxCreatePost(db *sql.DB, auth ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {

	text := strings.TrimSpace(r.FormValue("text"))
	if err := pocket.ValidatePostText(text); err != nil {
		return ajax.AjaxErrorPayload{ErrorCode: "invalid-post-text"}, http.StatusBadRequest
	}

	var parentPostID *uint
	if parentIDValue := strings.TrimSpace(r.FormValue("parentId")); parentIDValue != "" {
		parsed, err := strconv.ParseUint(parentIDValue, 10, 64)
		if err != nil {
			return nil, http.StatusBadRequest
		}
		actual := uint(parsed)
		parentPostID = &actual
	}

	topicNames := pocket.ParseTopicNames(r.FormValue("tags"))
	if len(topicNames) == 0 {
		topicNames = pocket.ParseTopicNamesJSON(r.FormValue("topics"))
	}

	post, err := pocket.CreatePost(db, parentPostID, auth.UserID, text, topicNames)
	if err != nil {
		logging.LogError(r, &auth, err)
		return nil, http.StatusInternalServerError
	}

	return map[string]any{
		"post": post,
	}, http.StatusOK

}

func AjaxAddPostTopic(db *sql.DB, auth ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {

	postID, err := types.AtoUint(r.FormValue("postId"))
	if err != nil {
		return nil, http.StatusBadRequest
	}

	topicNames := pocket.ParseTopicNamesJSON(r.FormValue("topics"))
	if len(topicNames) == 0 {
		return ajax.AjaxErrorPayload{ErrorCode: "invalid-topic-name"}, http.StatusBadRequest
	}

	existingNames := make(map[string]bool, len(topicNames))
	addedTopics := make([]pocket.Topic, 0, len(topicNames))
	for _, rawName := range topicNames {
		topicName := pocket.NormalizeTopicName(rawName)
		if err := pocket.ValidateTopicName(topicName); err != nil {
			continue
		}
		// skip duplicate names within this submission; EnsureTopicOnPost is idempotent for topics already on the post
		if existingNames[strings.ToLower(topicName)] {
			continue
		}
		topic, err := pocket.EnsureTopicOnPost(db, uint(postID), topicName, auth.UserID)
		if err != nil {
			logging.LogError(r, &auth, err)
			return nil, http.StatusInternalServerError
		}
		existingNames[strings.ToLower(topicName)] = true
		addedTopics = append(addedTopics, topic)
	}

	post, err := pocket.LoadPost(db, uint(postID), &auth.UserID)
	if err != nil {
		logging.LogError(r, &auth, err)
		return nil, http.StatusInternalServerError
	}

	return map[string]any{
		"topics": addedTopics,
		"post":   post,
	}, http.StatusOK

}

func AjaxVotePostTopic(db *sql.DB, auth ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {

	postID, err := types.AtoUint(r.FormValue("postId"))
	if err != nil {
		return nil, http.StatusBadRequest
	}

	topicID, err := types.AtoUint(r.FormValue("topicId"))
	if err != nil {
		return nil, http.StatusBadRequest
	}

	voteType := strings.TrimSpace(r.FormValue("voteType"))
	if voteType == "" {
		// Remove vote
		topic, err := pocket.RemovePostTopicVote(db, postID, topicID, auth.UserID)
		if err != nil {
			logging.LogError(r, &auth, err)
			return nil, http.StatusInternalServerError
		}
		totalTopicScore, err := pocket.LoadPostTotalTopicScore(db, postID)
		if err != nil {
			logging.LogError(r, &auth, err)
			return nil, http.StatusInternalServerError
		}
		return map[string]any{
			"topic":           topic,
			"totalTopicScore": totalTopicScore,
		}, http.StatusOK
	}

	if !pocket.IsValidVote(voteType) {
		return ajax.AjaxErrorPayload{ErrorCode: "invalid-vote-type"}, http.StatusBadRequest
	}

	topic, err := pocket.SetPostTopicVote(db, postID, topicID, auth.UserID, voteType)
	if err != nil {
		logging.LogError(r, &auth, err)
		return nil, http.StatusInternalServerError
	}

	totalTopicScore, err := pocket.LoadPostTotalTopicScore(db, postID)
	if err != nil {
		logging.LogError(r, &auth, err)
		return nil, http.StatusInternalServerError
	}

	return map[string]any{
		"topic":           topic,
		"totalTopicScore": totalTopicScore,
	}, http.StatusOK

}

// AjaxLoadPostsPage loads a page of posts for one of several contexts:
//   - context=dashboard: the site's top posts, paged by offset
//   - context=subposts: the sub-posts of a given postId, paged by offset
func AjaxLoadPostsPage(db *sql.DB, auth *ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {

	offset, err := types.AtoUint(r.FormValue("offset"))
	if err != nil {
		return nil, http.StatusBadRequest
	}

	var posts []pocket.Post

	switch r.FormValue("context") {

	case "dashboard":
		posts, err = pocket.LoadTopPosts(db, auth, offset)
		if err != nil {
			logging.LogError(r, auth, err)
			return nil, http.StatusInternalServerError
		}

	case "subposts":
		postID, err := types.AtoUint(r.FormValue("postId"))
		if err != nil {
			return nil, http.StatusBadRequest
		}
		posts, err = pocket.LoadTopSubPosts(db, auth, postID, offset)
		if err != nil {
			logging.LogError(r, auth, err)
			return nil, http.StatusInternalServerError
		}

	default:
		return nil, http.StatusBadRequest
	}

	return map[string]any{
		"posts":   posts,
		"hasMore": len(posts) == pocket.MaxPostPageSize,
	}, http.StatusOK

}
