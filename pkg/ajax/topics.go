package ajax

import (
	"database/sql"
	"net/http"
	"strings"

	"pocket/pkg/pocket"
	"pocket/pkg/utils/ajax"
	"pocket/pkg/utils/logging"
	"pocket/pkg/utils/types"
)

func AjaxPostTopic(db *sql.DB, auth ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {

	name := strings.TrimSpace(r.FormValue("name"))
	if err := pocket.ValidateTopicName(name); err != nil {
		return ajax.AjaxErrorPayload{
			ErrorCode: "invalid-topic-name",
		}, http.StatusBadRequest
	}

	normalizedName := pocket.NormalizeTopicName(name)

	exists, err := pocket.CheckTopicExists(db, normalizedName)
	if err != nil {
		return nil, http.StatusInternalServerError
	}
	if exists {
		return map[string]any{
			"exists": true,
			"topic":  nil,
		}, http.StatusOK
	}

	topic, err := pocket.CreateTopic(db, normalizedName, auth.UserID)
	if err != nil {
		logging.LogError(r, &auth, err)
		return nil, http.StatusInternalServerError
	}

	return map[string]any{
		"exists": false,
		"topic":  topic,
	}, http.StatusOK

}

// AjaxLoadTopicsPage loads a page of topics for one of several contexts:
//   - context=dashboard: the site's top topics, paged by offset
//   - context=post: the topics attached to a given postId, paged by offset
//   - context=space: the top topics tagged on the sub-posts of a given postId, paged by offset
func AjaxLoadTopicsPage(db *sql.DB, auth *ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {

	offset, err := types.AtoUint(r.FormValue("offset"))
	if err != nil {
		return nil, http.StatusBadRequest
	}

	var topics []pocket.Topic

	switch r.FormValue("context") {

	case "dashboard":
		selectedTopicIDs, err := types.AtoUintList(r.FormValue("topicIds"))
		if err != nil {
			return nil, http.StatusBadRequest
		}
		topics, err = pocket.LoadTopTopics(db, auth, offset, selectedTopicIDs)
		if err != nil {
			logging.LogError(r, auth, err)
			return nil, http.StatusInternalServerError
		}

	case "post":
		postID, err := types.AtoUint(r.FormValue("postId"))
		if err != nil {
			return nil, http.StatusBadRequest
		}
		var userID *uint
		if auth != nil {
			userID = &auth.UserID
		}
		topics, err = pocket.LoadPostTopics(db, postID, userID, offset)
		if err != nil {
			logging.LogError(r, auth, err)
			return nil, http.StatusInternalServerError
		}

	case "space":
		postID, err := types.AtoUint(r.FormValue("postId"))
		if err != nil {
			return nil, http.StatusBadRequest
		}
		selectedTopicIDs, err := types.AtoUintList(r.FormValue("topicIds"))
		if err != nil {
			return nil, http.StatusBadRequest
		}
		topics, err = pocket.LoadSpaceTopics(db, postID, offset, selectedTopicIDs)
		if err != nil {
			logging.LogError(r, auth, err)
			return nil, http.StatusInternalServerError
		}

	default:
		return nil, http.StatusBadRequest
	}

	return map[string]any{
		"topics":  topics,
		"hasMore": len(topics) == pocket.MaxTopicPageSize,
	}, http.StatusOK

}
