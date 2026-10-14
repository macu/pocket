package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"

	emailpkg "pocket/pkg/email"
	"pocket/pkg/env"
	"pocket/pkg/user"
	"pocket/pkg/utils/ajax"
	dbutil "pocket/pkg/utils/db"
	"pocket/pkg/utils/logging"
	"pocket/pkg/utils/types"
)

const accountTokenExpiry = time.Hour
const accountEmailCooldown = time.Minute

var sendAccountEmail = emailpkg.Send
var handlePattern = regexp.MustCompile(UserHandlePattern)

type accountError struct {
	code   string
	status int
}

func (e *accountError) Error() string { return e.code }

func invalidAccountInput(code string) error {
	return &accountError{code, http.StatusBadRequest}
}

func accountFailure(r *http.Request, auth *ajax.Auth, err error) (interface{}, int) {
	var inputError *accountError
	if errors.As(err, &inputError) {
		return ajax.AjaxErrorPayload{ErrorCode: inputError.code}, inputError.status
	}
	var pgError *pq.Error
	if errors.As(err, &pgError) && pgError.Code == "23505" {
		if strings.Contains(pgError.Constraint, "handle") {
			return ajax.AjaxErrorPayload{ErrorCode: "handle-exists"}, http.StatusConflict
		}
		if strings.Contains(pgError.Constraint, "email") {
			return ajax.AjaxErrorPayload{ErrorCode: "email-exists"}, http.StatusConflict
		}
	}
	logging.LogError(r, auth, err)
	return nil, http.StatusInternalServerError
}

func accountToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating account token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func accountTokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func accountLink(path, token string) (string, error) {
	base := strings.TrimSpace(env.GetPublicURL())
	if base == "" && env.IsAppEngine() && env.GetDomain() != "" {
		base = "https://" + env.GetDomain()
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "https" && !(env.IsLocal() && u.Scheme == "http")) ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("PUBLIC_URL must be a valid absolute application URL")
	}
	return u.ResolveReference(&url.URL{Path: path, RawQuery: url.Values{"token": {token}}.Encode()}).String(), nil
}

func validAccountPassword(password string) bool {
	return utf8.ValidString(password) && utf8.RuneCountInString(password) >= PasswordMinLength &&
		len(strings.TrimSpace(password)) > 0 && len(password) <= 72
}

func validateAccountProfile(name, handle, email string) error {
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > userDisplayNameMaxLength ||
		types.HasUnprintableChar(name) {
		return invalidAccountInput("invalid-name")
	}
	if handle != "" && (len(handle) > userHandleMaxLength || !handlePattern.MatchString(handle)) {
		return invalidAccountInput("invalid-handle")
	}
	if len(email) > 50 || !types.ValidateEmailAddress(email) {
		return invalidAccountInput("invalid-email")
	}
	return nil
}

func availableAccountEmail(db dbutil.DBConn, email string, userID uint) error {
	local, domain, _ := strings.Cut(email, "@")
	local, _, _ = strings.Cut(local, "+")
	var exists bool
	if err := db.QueryRow(`SELECT EXISTS(
		SELECT 1 FROM user_account WHERE id <> $2 AND
		(lower(email) = $1 OR (user_role = 'banned' AND
		split_part(split_part(lower(email), '@', 1), '+', 1) || '@' ||
		split_part(lower(email), '@', 2) = $3)))`, email, userID, local+"@"+domain).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return invalidAccountInput("email-exists")
	}
	return nil
}

func AjaxLoadAccount(db *sql.DB, auth ajax.Auth, w http.ResponseWriter, r *http.Request) (interface{}, int) {
	var profile struct {
		DisplayName  string  `json:"displayName"`
		Handle       string  `json:"handle"`
		Email        string  `json:"email"`
		PendingEmail *string `json:"pendingEmail"`
	}
	err := db.QueryRow(`SELECT u.display_name, COALESCE(u.handle, ''), u.email,
		(SELECT email FROM user_email_change_request WHERE user_id = u.id AND created_at > $2)
		FROM user_account u WHERE u.id = $1`, auth.UserID, time.Now().Add(-accountTokenExpiry)).
		Scan(&profile.DisplayName, &profile.Handle, &profile.Email, &profile.PendingEmail)
	if err != nil {
		return accountFailure(r, &auth, fmt.Errorf("loading account settings: %w", err))
	}
	return profile, http.StatusOK
}

func AjaxSaveAccount(db *sql.DB, auth ajax.Auth, w http.ResponseWriter, r *http.Request) (interface{}, int) {
	if types.HasUnprintableChar(r.FormValue("displayName")) {
		return accountFailure(r, &auth, invalidAccountInput("invalid-name"))
	}
	name := strings.TrimSpace(r.FormValue("displayName"))
	handle := strings.TrimSpace(r.FormValue("handle"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	if err := validateAccountProfile(name, handle, email); err != nil {
		return accountFailure(r, &auth, err)
	}
	changePassword := r.FormValue("changePassword") == "true"
	password := r.FormValue("newPassword")
	if changePassword && (!validAccountPassword(password) || password != r.FormValue("confirmPassword")) {
		return accountFailure(r, &auth, invalidAccountInput("invalid-password"))
	}
	var newHash []byte
	var session string
	var expires time.Time
	emailPending := false
	localToken := ""
	err := dbutil.InTransaction(db, func(tx *sql.Tx) error {
		var currentEmail, authHash string
		if err := tx.QueryRow(`SELECT email, auth_hash FROM user_account WHERE id = $1 FOR UPDATE`, auth.UserID).
			Scan(&currentEmail, &authHash); err != nil {
			return err
		}
		if changePassword {
			oldPassword := r.FormValue("oldPassword")
			if len(oldPassword) > 72 || bcrypt.CompareHashAndPassword([]byte(authHash), []byte(oldPassword)) != nil {
				return invalidAccountInput("incorrect-password")
			}
			var err error
			newHash, err = bcrypt.GenerateFromPassword([]byte(password), passwordBcryptCost)
			if err != nil {
				return err
			}
		}
		if err := availableAccountEmail(tx, email, auth.UserID); err != nil {
			return err
		}
		var handleExists bool
		if handle != "" {
			if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM user_account WHERE handle = $1 AND id <> $2)`, handle, auth.UserID).
				Scan(&handleExists); err != nil {
				return err
			}
			if handleExists {
				return invalidAccountInput("handle-exists")
			}
		}
		if _, err := tx.Exec(`UPDATE user_account SET display_name = $1, handle = NULLIF($2, '') WHERE id = $3`,
			name, handle, auth.UserID); err != nil {
			return err
		}
		if changePassword {
			if _, err := tx.Exec(`UPDATE user_account SET auth_hash = $1 WHERE id = $2`, string(newHash), auth.UserID); err != nil {
				return err
			}
			if _, err := tx.Exec(`DELETE FROM password_reset_request WHERE user_id = $1`, auth.UserID); err != nil {
				return err
			}
			if _, err := tx.Exec(`DELETE FROM user_session WHERE user_id = $1`, auth.UserID); err != nil {
				return err
			}
			var err error
			session, expires, err = newSession(tx, auth.UserID)
			if err != nil {
				return err
			}
		}
		if email != currentEmail {
			var pendingEmail string
			var created time.Time
			err := tx.QueryRow(`SELECT email, created_at FROM user_email_change_request WHERE user_id = $1`, auth.UserID).
				Scan(&pendingEmail, &created)
			if err != nil && err != sql.ErrNoRows {
				return err
			}
			if err == nil && pendingEmail == email && time.Since(created) < accountTokenExpiry && !env.IsLocal() {
				emailPending = true
				return nil
			}
			if err == nil && time.Since(created) < accountEmailCooldown && !env.IsLocal() {
				return &accountError{"email-rate-limit", http.StatusTooManyRequests}
			}
			token, err := accountToken()
			if err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO user_email_change_request (user_id, email, original_email, token, created_at)
				VALUES ($1, $2, $3, $4, $5) ON CONFLICT (user_id) DO UPDATE SET email = EXCLUDED.email,
				original_email = EXCLUDED.original_email, token = EXCLUDED.token, created_at = EXCLUDED.created_at`,
				auth.UserID, email, currentEmail, accountTokenHash(token), time.Now()); err != nil {
				return err
			}
			if env.IsLocal() {
				localToken = token
			} else {
				link, err := accountLink("/confirm-email", token)
				if err != nil {
					return err
				}
				if err := sendAccountEmail(email, "Confirm your Pocket email change",
					"Confirm your new email address using this link (expires in one hour):\n\n"+link); err != nil {
					return fmt.Errorf("sending email change confirmation: %w", err)
				}
			}
			emailPending = true
		} else {
			_, err := tx.Exec(`DELETE FROM user_email_change_request WHERE user_id = $1`, auth.UserID)
			return err
		}
		return nil
	})
	if err != nil {
		return accountFailure(r, &auth, fmt.Errorf("saving account settings: %w", err))
	}
	if session != "" {
		createCookie(w, expires, session)
	}
	return struct {
		EmailPending bool   `json:"emailPending"`
		Token        string `json:"token,omitempty"`
	}{emailPending, localToken}, http.StatusOK
}

func AjaxConfirmEmail(db *sql.DB, auth *ajax.Auth, w http.ResponseWriter, r *http.Request) (interface{}, int) {
	token := r.FormValue("token")
	if len(token) != 64 {
		return accountFailure(r, auth, invalidAccountInput("invalid-token"))
	}
	err := dbutil.InTransaction(db, func(tx *sql.Tx) error {
		var userID uint
		if err := tx.QueryRow(`SELECT user_id FROM user_email_change_request WHERE token = $1`, accountTokenHash(token)).
			Scan(&userID); err != nil {
			if err == sql.ErrNoRows {
				return invalidAccountInput("invalid-token")
			}
			return err
		}
		var currentEmail, role string
		if err := tx.QueryRow(`SELECT email, user_role FROM user_account WHERE id = $1 FOR UPDATE`, userID).
			Scan(&currentEmail, &role); err != nil {
			return err
		}
		var email, originalEmail string
		var created time.Time
		if err := tx.QueryRow(`SELECT email, original_email, created_at FROM user_email_change_request
			WHERE user_id = $1 AND token = $2 FOR UPDATE`, userID, accountTokenHash(token)).
			Scan(&email, &originalEmail, &created); err != nil {
			if err == sql.ErrNoRows {
				return invalidAccountInput("invalid-token")
			}
			return err
		}
		if !user.CheckRoleActive(role) || originalEmail != currentEmail {
			return invalidAccountInput("invalid-token")
		}
		if time.Since(created) >= accountTokenExpiry {
			return invalidAccountInput("token-expired")
		}
		if err := availableAccountEmail(tx, email, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE user_account SET email = $1 WHERE id = $2`, email, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM password_reset_request WHERE user_id = $1`, userID); err != nil {
			return err
		}
		_, err := tx.Exec(`DELETE FROM user_email_change_request WHERE user_id = $1`, userID)
		return err
	})
	if err != nil {
		return accountFailure(r, auth, fmt.Errorf("confirming account email: %w", err))
	}
	return true, http.StatusOK
}
