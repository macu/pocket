package main

import (
	"database/sql"
	_ "embed"
	"html/template"
	"net/http"

	"pocket/pkg/auth"
	"pocket/pkg/env"
	"pocket/pkg/pocket"
	"pocket/pkg/utils/ajax"
)

var indexTemplate = template.Must(template.ParseFiles("html/index.html"))

func indexHandler(db *sql.DB, user *ajax.Auth, w http.ResponseWriter, r *http.Request) {
	indexTemplate.Execute(w, struct {
		Local             bool
		VersionStamp      string
		PasswordMinLength uint
		HandlePattern     string
		MaxPostLength     int
		MaxTopicPageSize  int
		MaxTopicLength    int
		MaxPostPageSize   int
	}{
		env.IsLocal(),
		env.GetCacheControlVersionStamp(),
		auth.PasswordMinLength,
		auth.UserHandlePattern,
		pocket.MaxPostLength,
		pocket.MaxTopicPageSize,
		pocket.MaxTopicLength,
		pocket.MaxPostPageSize,
	})
}
