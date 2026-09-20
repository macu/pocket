package ajax

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"pocket/pkg/auth"
	"pocket/pkg/user"
	"pocket/pkg/utils/ajax"
	"pocket/pkg/utils/logging"
)

var ajaxHandlersAuthOptional = map[string]map[string]ajax.AjaxRouteAuthOptional{
	http.MethodGet: {
		"/ajax/load-login":  auth.AjaxLoadLogin,
		"/ajax/load-signup": auth.AjaxLoadSignup,

		"/ajax/dashboard": AjaxLoadDashboard,
	},
	http.MethodPost: {
		"/ajax/login":         auth.AjaxLogin,
		"/ajax/signup":        auth.AjaxSignup,
		"/ajax/signup-verify": auth.AjaxSignupVerify,
	},
}

var ajaxHandlersAuthRequired = map[string]map[string]ajax.AjaxRouteAuthRequired{
	http.MethodPost: {
		"/ajax/logout": auth.AjaxLogout,
		"/ajax/topic":  AjaxPostTopic,
	},
}

func AjaxHandler(db *sql.DB, auth *ajax.Auth, w http.ResponseWriter, r *http.Request) {
	// var rt = NewResponseTracker(w)

	var handle = func(handler func() (interface{}, int)) {

		// Verify access to admin routes
		if strings.HasPrefix(r.URL.Path, "/ajax/admin") {
			if auth == nil || !user.CheckRoleAdmin(auth.Role) {
				logging.LogError(r, auth, fmt.Errorf("forbidden admin access on %s", r.URL.Path))
				w.WriteHeader(http.StatusForbidden)
				return
			}
		}

		response, statusCode := handler()
		if response != nil {
			js, err := json.Marshal(response)
			if err != nil {
				logging.LogError(r, auth, fmt.Errorf("marshalling response: %w", err))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode) // WriteHeader is called after setting headers
			w.Write(js)
		} else {
			w.WriteHeader(statusCode)
		}

	}

	handlersAuthOptional, foundMethod := ajaxHandlersAuthOptional[r.Method]
	if foundMethod {
		handler, fouundPath := handlersAuthOptional[r.URL.Path]
		if fouundPath {
			handle(func() (interface{}, int) {
				return handler(db, auth, w, r)
			})
			return
		}
	}

	handlersAuthRequired, foundMethod := ajaxHandlersAuthRequired[r.Method]
	if foundMethod {
		handler, fouundPath := handlersAuthRequired[r.URL.Path]
		if fouundPath {
			if auth == nil {
				w.WriteHeader(http.StatusForbidden)
			} else {
				handle(func() (interface{}, int) {
					return handler(db, *auth, w, r)
				})
			}
			return
		}
	}

	logging.LogNotice(r, struct {
		Method string
		Path   string
	}{r.Method, r.URL.Path})

	w.WriteHeader(http.StatusNotFound)
}
