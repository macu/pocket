package ajax

import (
	"errors"
	"net/http"

	"pocket/pkg/moderation"
	"pocket/pkg/utils/ajax"
	"pocket/pkg/utils/logging"
)

const moderationRejectedErrorCode = "moderation-rejected"

// moderationFailure converts a moderation error into an AJAX response.
// ok is false when the request should be aborted with the returned payload and status.
func moderationFailure(r *http.Request, auth *ajax.Auth, err error) (payload any, status int, ok bool) {
	if err == nil {
		return nil, 0, true
	}
	logging.LogError(r, auth, err)
	if errors.Is(err, moderation.ErrRejected) {
		return ajax.AjaxErrorPayload{
			ErrorCode: moderationRejectedErrorCode,
		}, http.StatusUnprocessableEntity, false
	}
	return nil, http.StatusInternalServerError, false
}
