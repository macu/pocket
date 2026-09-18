package auth

import (
	"database/sql"
	"net/http"

	"pocket/pkg/utils/ajax"
)

type AuthOptionalHandler func(
	db *sql.DB,
	auth *ajax.Auth,
	w http.ResponseWriter,
	r *http.Request,
)

type User struct {
	ID     uint   `json:"id"`
	Handle string `json:"handle"`
}
