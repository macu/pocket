package ajax

import (
	"database/sql"
	"net/http"
	"strings"

	"pocket/pkg/pocket"
	userpkg "pocket/pkg/user"
	"pocket/pkg/utils/ajax"
	"pocket/pkg/utils/logging"
)

// AjaxLoadUser loads a user (by handle or numeric ID) along with their top
// topics and posts for the given timeframe.
func AjaxLoadUser(db *sql.DB, auth *ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {

	identifier := strings.TrimSpace(r.FormValue("user"))
	if identifier == "" {
		return nil, http.StatusBadRequest
	}

	cutoff, err := pocket.ParseTimeframe(r.FormValue("timeframe"))
	if err != nil {
		return nil, http.StatusBadRequest
	}
	mostRecent := pocket.IsMostRecentTimeframe(r.FormValue("timeframe"))

	user, err := pocket.LoadUserByIdentifier(db, identifier)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, http.StatusNotFound
		}
		logging.LogError(r, auth, err)
		return nil, http.StatusInternalServerError
	}

	if auth == nil || !userpkg.CheckRoleAdmin(auth.Role) {
		user.Role = ""
	}

	topTopics, err := pocket.LoadUserTopics(db, user.ID, 0, nil, cutoff)
	if err != nil {
		logging.LogError(r, auth, err)
		return nil, http.StatusInternalServerError
	}

	topPosts, err := pocket.LoadUserPosts(db, auth, user.ID, 0, nil, cutoff, mostRecent, nil)
	if err != nil {
		logging.LogError(r, auth, err)
		return nil, http.StatusInternalServerError
	}

	totalPosts, err := pocket.CountUserPosts(db, user.ID, nil, cutoff)
	if err != nil {
		logging.LogError(r, auth, err)
		return nil, http.StatusInternalServerError
	}

	return map[string]any{
		"user":       user,
		"topTopics":  topTopics,
		"topPosts":   topPosts,
		"totalPosts": totalPosts,
	}, http.StatusOK

}
