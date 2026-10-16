package database

import "net/url"

// PostgresConnectionURL builds a PostgreSQL URL for local and Cloud SQL socket connections.
func PostgresConnectionURL(host, user, password, databaseName string) string {
	query := url.Values{}
	query.Set("host", host)
	query.Set("sslmode", "disable")

	return (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Path:     "/" + databaseName,
		RawQuery: query.Encode(),
	}).String()
}
