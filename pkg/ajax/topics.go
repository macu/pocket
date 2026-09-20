package ajax

import (
	"database/sql"
	"net/http"
	"strings"

	"pocket/pkg/pocket"
	"pocket/pkg/utils/ajax"
	"pocket/pkg/utils/logging"
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
