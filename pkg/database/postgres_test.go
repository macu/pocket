package database

import (
	"net/url"
	"testing"

	"github.com/lib/pq"
)

func TestPostgresConnectionURLCloudSQLSocket(t *testing.T) {
	const (
		host     = "/cloudsql/project:region:instance"
		user     = "pocket user"
		password = "p@ss/word"
		dbName   = "pocket"
	)

	assertPostgresURL(t, PostgresConnectionURL(host, user, password, dbName), host, user, password, dbName)
}

func TestPostgresConnectionURLLocalDockerHost(t *testing.T) {
	const (
		host     = "db"
		user     = "user"
		password = "pass"
		dbName   = "pocket"
	)

	assertPostgresURL(t, PostgresConnectionURL(host, user, password, dbName), host, user, password, dbName)
}

func assertPostgresURL(t *testing.T, connectionURL, host, user, password, dbName string) {
	t.Helper()

	parsed, err := url.Parse(connectionURL)
	if err != nil {
		t.Fatalf("parse connection URL: %v", err)
	}
	if _, err := pq.ParseURL(connectionURL); err != nil {
		t.Fatalf("parse URL with PostgreSQL driver: %v", err)
	}
	if parsed.Scheme != "postgres" {
		t.Errorf("scheme = %q, want postgres", parsed.Scheme)
	}
	if got := parsed.User.Username(); got != user {
		t.Errorf("username = %q, want %q", got, user)
	}
	gotPassword, ok := parsed.User.Password()
	if !ok || gotPassword != password {
		t.Errorf("password = %q, want %q", gotPassword, password)
	}
	if parsed.Path != "/"+dbName {
		t.Errorf("path = %q, want /%s", parsed.Path, dbName)
	}
	if got := parsed.Query().Get("host"); got != host {
		t.Errorf("host = %q, want %q", got, host)
	}
	if got := parsed.Query().Get("sslmode"); got != "disable" {
		t.Errorf("sslmode = %q, want disable", got)
	}
}
