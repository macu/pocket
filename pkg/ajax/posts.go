package ajax

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"pocket/pkg/pocket"
	"pocket/pkg/utils/ajax"
	"pocket/pkg/utils/logging"
)

func AjaxLoadPost(db *sql.DB, auth *ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {
	idValue := strings.TrimSpace(r.FormValue("id"))
	if idValue == "" {
		return nil, http.StatusBadRequest
	}

	id, err := strconv.ParseUint(idValue, 10, 64)
	if err != nil {
		return nil, http.StatusBadRequest
	}

	post, err := pocket.LoadPost(db, uint(id))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, http.StatusNotFound
		}
		logging.LogError(r, auth, err)
		return nil, http.StatusInternalServerError
	}

	var parentPost any
	if post.ParentPostID != nil {
		parent, err := pocket.LoadPost(db, *post.ParentPostID)
		if err != nil && err != sql.ErrNoRows {
			logging.LogError(r, auth, err)
			return nil, http.StatusInternalServerError
		}
		if err == nil {
			parentPost = parent
		}
	}

	return map[string]any{
		"post":       post,
		"parentPost": parentPost,
	}, http.StatusOK
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
		topicNames = pocket.ParseTopicNames(r.FormValue("topics"))
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

func AjaxAddPostTag(db *sql.DB, auth ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {
	postIDValue := strings.TrimSpace(r.FormValue("postId"))
	if postIDValue == "" {
		return nil, http.StatusBadRequest
	}

	postID, err := strconv.ParseUint(postIDValue, 10, 64)
	if err != nil {
		return nil, http.StatusBadRequest
	}

	tagName := pocket.NormalizeTopicName(r.FormValue("tag"))
	if err := pocket.ValidateTopicName(tagName); err != nil {
		return ajax.AjaxErrorPayload{ErrorCode: "invalid-topic-name"}, http.StatusBadRequest
	}

	topic, err := pocket.EnsureTopicOnPost(db, uint(postID), tagName, auth.UserID)
	if err != nil {
		logging.LogError(r, &auth, err)
		return nil, http.StatusInternalServerError
	}

	post, err := pocket.LoadPost(db, uint(postID))
	if err != nil {
		logging.LogError(r, &auth, err)
		return nil, http.StatusInternalServerError
	}

	return map[string]any{
		"topic": topic,
		"post":  post,
	}, http.StatusOK
}
