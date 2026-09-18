package main

import (
	"database/sql"
	_ "embed"
	"html/template"
	"net/http"

	"pocket/pkg/auth"
	"pocket/pkg/env"
	"pocket/pkg/utils/ajax"
)

var indexTemplate = template.Must(template.ParseFiles("html/index.html"))

func indexHandler(db *sql.DB, user *ajax.Auth, w http.ResponseWriter, r *http.Request) {
	indexTemplate.Execute(w, struct {
		Local             bool
		VersionStamp      string
		PasswordMinLength uint
		HandlePattern     string
	}{
		env.IsLocal(),
		env.GetCacheControlVersionStamp(),
		auth.PasswordMinLength,
		auth.UserHandlePattern,
	})
}
