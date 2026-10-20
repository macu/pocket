package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"

	"pocket/pkg/ajax"
	"pocket/pkg/auth"
	"pocket/pkg/database"
	"pocket/pkg/env"
	"pocket/pkg/pocket"
	"pocket/pkg/utils/logging"
	"pocket/pkg/utils/secrets"
	"pocket/pkg/utils/types"
)

func main() {

	if types.AtoBool(env.GetMaintenanceMode()) {
		// Site down for databae upgrades
		maintenanceMode()
		os.Exit(0)

	}

	var port = env.GetPort()
	var dbHost = env.GetDbHost()
	var dbUser = env.GetDbUser()
	var dbPass = env.GetDbPass()
	var dbName = env.GetDbName()
	var err error

	if env.IsAppEngine() {

		// Database password loaded from secret manager
		secretName := env.GetDbPassSecret()
		dbPass, err = secrets.LoadSecret(secretName)
		if err != nil {
			logging.LogErrorFatal(err)
		}

		for _, secret := range []struct {
			name        string
			environment string
		}{
			{name: env.GetOpenAISecretKeySecret(), environment: "OPENAI_SECRET_KEY"},
			{name: env.GetMailjetApiKeySecret(), environment: "MAILJET_API_KEY"},
			{name: env.GetMailjetSecretSecret(), environment: "MAILJET_SECRET"},
		} {
			if secret.name == "" {
				continue
			}
			value, loadErr := secrets.LoadSecret(secret.name)
			if loadErr != nil {
				logging.LogErrorFatal(loadErr)
			}
			if setErr := os.Setenv(secret.environment, value); setErr != nil {
				logging.LogErrorFatal(setErr)
			}
		}

	}

	// Connect to postgres, using DB_HOST, DB_USER, DB_PASS, DB_NAME
	dbURL := database.PostgresConnectionURL(dbHost, dbUser, dbPass, dbName)
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}

	r := mux.NewRouter()

	r.HandleFunc("/cron/auth-cleanup", func(w http.ResponseWriter, r *http.Request) {
		if !env.IsAppEngine() || r.Header.Get("X-Appengine-Cron") != "true" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		if err := auth.CleanupExpiredRecords(r.Context(), db); err != nil {
			logging.LogError(r, nil, err)
			http.Error(w, "Auth cleanup failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}).Methods(http.MethodGet)

	r.HandleFunc("/cron/post-images", func(w http.ResponseWriter, r *http.Request) {
		if !env.IsAppEngine() || r.Header.Get("X-Appengine-Cron") != "true" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		if err := pocket.CleanupPostImages(r.Context(), db); err != nil {
			logging.LogError(r, nil, err)
			http.Error(w, "Image cleanup failed", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}).Methods(http.MethodGet)

	if env.IsAppEngine() {
		// Set up AppEngine cron handlers
		// r.HandleFunc("/cron/cleanup", makeCronHandler(db, cleanupHandler))
	} else {
		// Set up static resource routes
		// (These static directories are configured by app.yaml for App Engine)
		r.PathPrefix("/css/").Handler(http.StripPrefix("/css/", http.FileServer(http.Dir("css"))))
		r.PathPrefix("/js/").Handler(http.StripPrefix("/js/", http.FileServer(http.Dir("js"))))
		r.PathPrefix("/img/").Handler(http.StripPrefix("/img/", http.FileServer(http.Dir("img"))))
	}

	// set up authenticated routes
	authenticate := auth.MakeAuthenticator(db)

	r.PathPrefix("/ajax/").HandlerFunc(authenticate(ajax.AjaxHandler))

	// All other paths go through index handler
	r.PathPrefix("/").HandlerFunc(authenticate(indexHandler))

	s := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		// Write link to /test
		fmt.Fprintf(w, "Welcome to Pocket!")
	})

	if err = s.ListenAndServe(); err != http.ErrServerClosed {
		logging.LogErrorFatal(err)
	}

	// Flush pending logs
	logging.CloseLoggingClients()

}
