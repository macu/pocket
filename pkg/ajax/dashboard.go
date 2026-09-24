package ajax

import (
	"database/sql"
	"net/http"

	"pocket/pkg/pocket"
	"pocket/pkg/utils/ajax"
	"pocket/pkg/utils/logging"
)

func AjaxLoadDashboard(db *sql.DB, auth *ajax.Auth,
	w http.ResponseWriter, r *http.Request,
) (any, int) {

	cutoff, err := pocket.ParseTimeframe(r.FormValue("timeframe"))
	if err != nil {
		return nil, http.StatusBadRequest
	}

	topTopics, err := pocket.LoadTopTopics(db, auth, 0, nil, cutoff)
	if err != nil {
		logging.LogError(r, auth, err)
		return nil, http.StatusInternalServerError
	}

	topPosts, err := pocket.LoadTopPosts(db, auth, 0, nil, cutoff)
	if err != nil {
		logging.LogError(r, auth, err)
		return nil, http.StatusInternalServerError
	}

	totalPosts, err := pocket.CountTopPosts(db, nil, cutoff)
	if err != nil {
		logging.LogError(r, auth, err)
		return nil, http.StatusInternalServerError
	}

	return map[string]any{
		"topTopics":  topTopics,
		"topPosts":   topPosts,
		"totalPosts": totalPosts,
	}, 200

}
