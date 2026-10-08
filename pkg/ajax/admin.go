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

	if err := pocket.SetUserRole(db, userID, status); err != nil {
		if err == sql.ErrNoRows {
			return nil, http.StatusNotFound
		}
		if err == pocket.ErrUserIsAdmin {
			return ajax.AjaxErrorPayload{ErrorCode: "user-is-admin"}, http.StatusForbidden
		}
		logging.LogError(r, &auth, err)
		return nil, http.StatusInternalServerError
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

	parentID, err := pocket.DeletePost(db, postID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, http.StatusNotFound
		}
		logging.LogError(r, &auth, err)
		return nil, http.StatusInternalServerError
	}

	return map[string]any{
		"parentPostId": parentID,
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

func AjaxAdminDeleteUserTopics(db *sql.DB, auth ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {

	userID, err := types.AtoUint(r.FormValue("userId"))
	if err != nil {
		return nil, http.StatusBadRequest
	}
	topicIDs, err := types.AtoUintList(r.FormValue("topicIds"))
	if err != nil || len(topicIDs) == 0 || len(topicIDs) > pocket.MaxBulkTopicDelete {
		return nil, http.StatusBadRequest
	}

	deleted, err := pocket.DeleteUserTopics(db, userID, topicIDs)
	if err != nil {
		logging.LogError(r, &auth, err)
		return nil, http.StatusInternalServerError
	}

	return map[string]any{
		"deleted": deleted,
	}, http.StatusOK

}
