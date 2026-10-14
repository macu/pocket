package auth

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"pocket/pkg/env"
	"pocket/pkg/user"
	"pocket/pkg/utils/ajax"
	dbutil "pocket/pkg/utils/db"
)

func AjaxRequestPasswordReset(db *sql.DB, auth *ajax.Auth, w http.ResponseWriter, r *http.Request) (interface{}, int) {
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	if err := validateAccountProfile("Reset", "", email); err != nil {
		return accountFailure(r, auth, err)
	}
	localToken := ""
	err := dbutil.InTransaction(db, func(tx *sql.Tx) error {
		var userID uint
		var role string
		err := tx.QueryRow(`SELECT id, user_role FROM user_account WHERE lower(email) = $1 FOR UPDATE`, email).
			Scan(&userID, &role)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return err
		}
		if !user.CheckRoleActive(role) {
			return nil
		}
		var latest sql.NullTime
		if err := tx.QueryRow(`SELECT MAX(created_at) FROM password_reset_request WHERE user_id = $1`, userID).
			Scan(&latest); err != nil {
			return err
		}
		if latest.Valid && time.Since(latest.Time) < accountEmailCooldown && !env.IsLocal() {
			return nil
		}
		token, err := accountToken()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM password_reset_request WHERE user_id = $1`, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO password_reset_request (user_id, sent_to_address, token, created_at)
			VALUES ($1, $2, $3, $4)`, userID, email, accountTokenHash(token), time.Now()); err != nil {
			return err
		}
		if env.IsLocal() {
			localToken = token
		} else {
			link, err := accountLink("/reset-password", token)
			if err != nil {
				return err
			}
			if err := sendAccountEmail(email, "Reset your Pocket password",
				"Reset your password using this link (expires in one hour):\n\n"+link+
					"\n\nIf you did not request this, you can ignore this email."); err != nil {
				return fmt.Errorf("sending password reset email: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return accountFailure(r, auth, fmt.Errorf("requesting password reset: %w", err))
	}
	return struct {
		Token string `json:"token,omitempty"`
	}{localToken}, http.StatusOK
}

func AjaxResetPassword(db *sql.DB, auth *ajax.Auth, w http.ResponseWriter, r *http.Request) (interface{}, int) {
	token := r.FormValue("token")
	password := r.FormValue("newPassword")
	if len(token) != 64 {
		return accountFailure(r, auth, invalidAccountInput("invalid-token"))
	}
	if !validAccountPassword(password) || password != r.FormValue("confirmPassword") {
		return accountFailure(r, auth, invalidAccountInput("invalid-password"))
	}
	var session string
	var expires time.Time
	err := dbutil.InTransaction(db, func(tx *sql.Tx) error {
		var userID uint
		if err := tx.QueryRow(`SELECT user_id FROM password_reset_request WHERE token = $1`, accountTokenHash(token)).
			Scan(&userID); err != nil {
			if err == sql.ErrNoRows {
				return invalidAccountInput("invalid-token")
			}
			return err
		}
		var email, role string
		if err := tx.QueryRow(`SELECT email, user_role FROM user_account WHERE id = $1 FOR UPDATE`, userID).
			Scan(&email, &role); err != nil {
			return err
		}
		var sentTo string
		var created time.Time
		if err := tx.QueryRow(`SELECT sent_to_address, created_at FROM password_reset_request
			WHERE user_id = $1 AND token = $2 FOR UPDATE`, userID, accountTokenHash(token)).
			Scan(&sentTo, &created); err != nil {
			if err == sql.ErrNoRows {
				return invalidAccountInput("invalid-token")
			}
			return err
		}
		if !user.CheckRoleActive(role) || email != sentTo {
			return invalidAccountInput("invalid-token")
		}
		if time.Since(created) >= accountTokenExpiry {
			return invalidAccountInput("token-expired")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), passwordBcryptCost)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE user_account SET auth_hash = $1 WHERE id = $2`, string(hash), userID); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM password_reset_request WHERE user_id = $1`, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM user_email_change_request WHERE user_id = $1`, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM user_session WHERE user_id = $1`, userID); err != nil {
			return err
		}
		session, expires, err = newSession(tx, userID)
		return err
	})
	if err != nil {
		return accountFailure(r, auth, fmt.Errorf("resetting password: %w", err))
	}
	createCookie(w, expires, session)
	return true, http.StatusOK
}
