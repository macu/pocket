package pocket

import (
	"database/sql"
	"fmt"
	"regexp"
	"time"
)

// User is the public profile of a user account.
type User struct {
	ID          uint      `json:"id"`
	Handle      string    `json:"handle,omitempty"`
	DisplayName string    `json:"displayName"`
	CreatedAt   time.Time `json:"createdAt"`
}

var numericIdentifierPattern = regexp.MustCompile(`^[0-9]+$`)

// LoadUserByIdentifier loads a user by handle, or by numeric ID if identifier
// is all digits (handles always begin with a letter). Returns sql.ErrNoRows
// if there is no such user.
func LoadUserByIdentifier(conn *sql.DB, identifier string) (*User, error) {

	column := "handle"
	var arg any = identifier
	if numericIdentifierPattern.MatchString(identifier) {
		column = "id"
		var id uint64
		if _, err := fmt.Sscan(identifier, &id); err != nil || id > 2147483647 {
			return nil, sql.ErrNoRows
		}
		arg = id
	}

	var user User
	var handle sql.NullString
	err := conn.QueryRow(`
		SELECT id, handle, display_name, created_at FROM user_account WHERE `+column+` = $1
	`, arg).Scan(&user.ID, &handle, &user.DisplayName, &user.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("loading user %q: %w", identifier, err)
	}
	if handle.Valid {
		user.Handle = handle.String
	}
	return &user, nil
}
