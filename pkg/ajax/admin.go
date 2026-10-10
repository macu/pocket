package ajax

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"pocket/pkg/email"
	"pocket/pkg/pocket"
	"pocket/pkg/utils/ajax"
	"pocket/pkg/utils/logging"
	"pocket/pkg/utils/types"
)

// All handlers in this file are served under /ajax/admin, which AjaxHandler
// restricts to admins.

func AjaxAdminLoadUsers(db *sql.DB, auth ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {

	offset, err := types.AtoUint(r.FormValue("offset"))
	if err != nil {
		return nil, http.StatusBadRequest
	}

	users, total, err := pocket.SearchAdminUsers(db, r.FormValue("query"), strings.TrimSpace(r.FormValue("status")), offset)
	if err != nil {
		logging.LogError(r, &auth, err)
		return nil, http.StatusInternalServerError
	}

	return map[string]any{
		"users":    users,
		"total":    total,
		"pageSize": pocket.AdminUserPageSize,
	}, http.StatusOK

}

func AjaxAdminSetUserStatus(db *sql.DB, auth ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {

	userID, err := types.AtoUint(r.FormValue("userId"))
	if err != nil {
		return nil, http.StatusBadRequest
	}

	status := r.FormValue("status")
	if !pocket.IsAssignableRole(status) {
		return ajax.AjaxErrorPayload{ErrorCode: "invalid-status"}, http.StatusBadRequest
	}

	updatedUser, previousStatus, err := pocket.SetUserRole(db, userID, status)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, http.StatusNotFound
		}
		if err == pocket.ErrUserIsAdmin {
			return ajax.AjaxErrorPayload{ErrorCode: "user-is-admin"}, http.StatusForbidden
		}
		logging.LogError(r, &auth, err)
		return nil, http.StatusInternalServerError
	}
	if previousStatus != status {
		body := fmt.Sprintf("Your Pocket account status has changed from %s to %s.", previousStatus, status)
		if err := email.Send(updatedUser.Email, "Your Pocket account status changed", body); err != nil {
			logging.LogError(r, &auth, fmt.Errorf("sending account status email to user %d: %w", userID, err))
		}
	}

	return map[string]any{
		"status": status,
	}, http.StatusOK

}

func AjaxAdminDeleteUserContent(db *sql.DB, auth ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {

	userID, err := types.AtoUint(r.FormValue("userId"))
	if err != nil {
		return nil, http.StatusBadRequest
	}

	if err := pocket.DeleteUserContent(db, userID); err != nil {
		if err == sql.ErrNoRows {
			return nil, http.StatusNotFound
		}
		if err == pocket.ErrUserIsAdmin {
			return ajax.AjaxErrorPayload{ErrorCode: "user-is-admin"}, http.StatusForbidden
		}
		logging.LogError(r, &auth, err)
		return nil, http.StatusInternalServerError
	}

	return map[string]any{}, http.StatusOK

}

func AjaxAdminDeletePost(db *sql.DB, auth ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {

	postID, err := types.AtoUint(r.FormValue("id"))
	if err != nil {
		return nil, http.StatusBadRequest
	}

	deletedPost, err := pocket.DeletePost(db, postID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, http.StatusNotFound
		}
		logging.LogError(r, &auth, err)
		return nil, http.StatusInternalServerError
	}
	body := fmt.Sprintf("An administrator deleted your post from Pocket.\n\nPost:\n%s", deletedPost.PostText)
	if err := email.Send(deletedPost.AuthorEmail, "Your Pocket post was deleted", body); err != nil {
		logging.LogError(r, &auth, fmt.Errorf("sending post deletion email for post %d: %w", postID, err))
	}

	return map[string]any{
		"parentPostId": deletedPost.ParentPostID,
	}, http.StatusOK

}

func AjaxAdminLoadUser(db *sql.DB, auth ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {

	userID, err := types.AtoUint(r.FormValue("userId"))
	if err != nil {
		return nil, http.StatusBadRequest
	}

	u, err := pocket.GetAdminUser(db, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, http.StatusNotFound
		}
		logging.LogError(r, &auth, err)
		return nil, http.StatusInternalServerError
	}

	return map[string]any{
		"user": u,
	}, http.StatusOK

}

func AjaxAdminLoadUserTopics(db *sql.DB, auth ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {

	userID, err := types.AtoUint(r.FormValue("userId"))
	if err != nil {
		return nil, http.StatusBadRequest
	}
	offset, err := types.AtoUint(r.FormValue("offset"))
	if err != nil {
		return nil, http.StatusBadRequest
	}

	topics, total, err := pocket.LoadUserCreatedTopics(db, userID, offset)
	if err != nil {
		logging.LogError(r, &auth, err)
		return nil, http.StatusInternalServerError
	}

	return map[string]any{
		"topics":   topics,
		"total":    total,
		"pageSize": pocket.AdminUserPageSize,
	}, http.StatusOK

}

func AjaxAdminLoadTopics(db *sql.DB, auth ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {

	offset, err := types.AtoUint(r.FormValue("offset"))
	if err != nil {
		return nil, http.StatusBadRequest
	}

	topics, total, err := pocket.SearchAdminTopics(db, r.FormValue("query"), offset)
	if err != nil {
		logging.LogError(r, &auth, err)
		return nil, http.StatusInternalServerError
	}

	return map[string]any{
		"topics":   topics,
		"total":    total,
		"pageSize": pocket.AdminUserPageSize,
	}, http.StatusOK

}

func AjaxAdminSetTopicsBanned(db *sql.DB, auth ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {

	topicIDs, err := types.AtoUintList(r.FormValue("topicIds"))
	if err != nil || len(topicIDs) == 0 || len(topicIDs) > pocket.MaxBulkTopicStatus {
		return nil, http.StatusBadRequest
	}

	var banned bool
	switch r.FormValue("banned") {
	case "true":
		banned = true
	case "false":
	default:
		return nil, http.StatusBadRequest
	}

	updated, err := pocket.SetTopicsBanned(db, topicIDs, banned)
	if err != nil {
		logging.LogError(r, &auth, err)
		return nil, http.StatusInternalServerError
	}

	return map[string]any{
		"updated": updated,
	}, http.StatusOK

}

func AjaxAdminLoadUserPosts(db *sql.DB, auth ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {

	userID, err := types.AtoUint(r.FormValue("userId"))
	if err != nil {
		return nil, http.StatusBadRequest
	}
	offset, err := types.AtoUint(r.FormValue("offset"))
	if err != nil {
		return nil, http.StatusBadRequest
	}

	posts, total, err := pocket.LoadUserAuthoredPosts(db, userID, offset)
	if err != nil {
		logging.LogError(r, &auth, err)
		return nil, http.StatusInternalServerError
	}

	return map[string]any{
		"posts":    posts,
		"total":    total,
		"pageSize": pocket.AdminUserPageSize,
	}, http.StatusOK

}
