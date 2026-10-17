package auth

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	emailpkg "pocket/pkg/email"
	"pocket/pkg/utils/ajax"
	dbutil "pocket/pkg/utils/db"
	"pocket/pkg/utils/logging"
	netutil "pocket/pkg/utils/net"
	"pocket/pkg/utils/random"
	"pocket/pkg/utils/types"

	"golang.org/x/crypto/bcrypt"
)

func emailUnavailableForSignup(db *sql.DB, email string) (bool, error) {
	localPart, domain, _ := strings.Cut(strings.ToLower(email), "@")
	localPart, _, _ = strings.Cut(localPart, "+")
	banMatchEmail := localPart + "@" + domain

	var unavailable bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM user_account
			WHERE email = $1 OR (
				user_role = 'banned'
				AND split_part(split_part(lower(email), '@', 1), '+', 1) || '@' ||
					split_part(lower(email), '@', 2) = $2
			)
		)
	`, email, banMatchEmail).Scan(&unavailable)
	return unavailable, err
}

func AjaxLoadSignup(db *sql.DB, auth *ajax.Auth, w http.ResponseWriter, r *http.Request) (interface{}, int) {

	if auth != nil {
		// Already authenticated
		return nil, http.StatusForbidden
	}

	var token = strings.TrimSpace(r.FormValue("token"))

	if token == "" {
		return nil, http.StatusBadRequest
	}

	var email string
	var createdAt time.Time
	err := db.QueryRow(
		`SELECT email, created_at
		FROM user_signup_request
		WHERE token = $1`,
		token,
	).Scan(&email, &createdAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return ajax.AjaxErrorPayload{
				ErrorCode: "invalid-token",
			}, http.StatusBadRequest
		}

		logging.LogError(r, nil, fmt.Errorf("error checking signup request: %v", err))
		return nil, http.StatusInternalServerError
	}

	if time.Since(createdAt) > signupTokenExpiry {
		return ajax.AjaxErrorPayload{
			ErrorCode: "token-expired",
		}, http.StatusBadRequest
	}

	return struct {
		Email string `json:"email"`
		Token string `json:"token"`
	}{
		Email: email,
		Token: token,
	}, http.StatusOK

}

func AjaxSignup(db *sql.DB, auth *ajax.Auth, w http.ResponseWriter, r *http.Request) (interface{}, int) {

	if auth != nil {
		// Already authenticated
		return nil, http.StatusForbidden
	}

	var email = strings.ToLower(strings.TrimSpace(r.FormValue("email")))

	if email == "" || !types.ValidateEmailAddress(email) {
		return ajax.AjaxErrorPayload{
			ErrorCode: "invalid-email",
		}, http.StatusBadRequest
	}

	// check if user exists
	userExists, err := emailUnavailableForSignup(db, email)

	if err != nil {
		logging.LogError(r, nil, fmt.Errorf("error checking if user exists: %v", err))
		return nil, http.StatusInternalServerError
	}

	if userExists {
		return ajax.AjaxErrorPayload{
			ErrorCode: "email-exists",
		}, http.StatusBadRequest
	}

	var token = random.RandomToken(signupRequestTokenLength)

	var requestID int64
	err = db.QueryRow(
		`INSERT INTO user_signup_request (email, token, created_at)
		VALUES ($1, $2, $3)
		RETURNING id`,
		email, token, time.Now(),
	).Scan(&requestID)

	if err != nil {
		logging.LogError(r, nil, fmt.Errorf("error inserting signup request: %v", err))
		return nil, http.StatusInternalServerError
	}

	logging.LogNotice(r, struct {
		Event        string
		EmailAddress string
		// IPAddress    string
	}{
		"SignupRequest",
		email,
		// getUserIP(r),
	})

	if emailpkg.Bypass() {
		// Return token so the frontend can skip the email step
		return struct {
			BypassEmail bool   `json:"bypassEmail"`
			Token       string `json:"token"`
		}{
			BypassEmail: true,
			Token:       token,
		}, http.StatusOK
	}

	verifyURL, err := netutil.BuildAbsoluteURL("/verify-signup", url.Values{"token": {token}})
	if err != nil {
		logging.LogError(r, nil, fmt.Errorf("building signup verification URL: %w", err))
		return nil, http.StatusInternalServerError
	}
	if err := emailpkg.Send(email, emailpkg.Message{
		Subject: "Verify your new Pocket account",
		Heading: "Welcome to Pocket",
		Paragraphs: []string{
			"Please verify your email address by clicking the link below to finish creating your account.",
			"This link expires in one hour.",
		},
		Action: &emailpkg.Action{Label: "Finish creating account", URL: verifyURL},
		Note:   "If you did not sign up for Pocket, you can ignore this email.",
	}); err != nil {
		logging.LogError(r, nil, fmt.Errorf("sending signup verification email: %w", err))
		return nil, http.StatusInternalServerError
	}

	return true, http.StatusOK

}

func AjaxSignupVerify(db *sql.DB, auth *ajax.Auth, w http.ResponseWriter, r *http.Request) (interface{}, int) {

	if auth != nil {
		// Already authenticated
		return nil, http.StatusForbidden
	}

	var token = strings.TrimSpace(r.FormValue("token"))
	var password = r.FormValue("password")
	var handle = strings.TrimSpace(r.FormValue("handle"))
	var displayName = types.NormalizeSpaces(r.FormValue("displayName"))
	var message = strings.TrimSpace(r.FormValue("message"))

	// enforce client-side validation
	if token == "" ||
		len(strings.TrimSpace(password)) < PasswordMinLength || len(message) > 200 {
		return nil, http.StatusBadRequest
	}

	if !validDisplayName(r.FormValue("displayName")) {
		return ajax.AjaxErrorPayload{
			ErrorCode: "invalid-name",
		}, http.StatusBadRequest
	}

	// load request
	var requestID int64
	var email string
	var createdAt time.Time
	err := db.QueryRow(
		`SELECT id, email, created_at
		FROM user_signup_request
		WHERE token = $1`,
		token,
	).Scan(&requestID, &email, &createdAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return ajax.AjaxErrorPayload{
				ErrorCode: "invalid-token",
			}, http.StatusBadRequest
		}

		logging.LogError(r, nil, fmt.Errorf("error checking signup request: %v", err))
		return nil, http.StatusInternalServerError
	}

	if time.Since(createdAt) > signupTokenExpiry {
		return ajax.AjaxErrorPayload{
			ErrorCode: "token-expired",
		}, http.StatusBadRequest
	}

	if !validHandle(handle) {
		return ajax.AjaxErrorPayload{
			ErrorCode: "invalid-handle",
		}, http.StatusBadRequest
	}

	// verify handle is unused
	var handleExists bool
	err = db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM user_account WHERE handle = $1)",
		handle,
	).Scan(&handleExists)

	if err != nil {
		logging.LogError(r, nil, fmt.Errorf("error checking if handle exists: %v", err))
		return nil, http.StatusInternalServerError
	}

	if handleExists {
		return ajax.AjaxErrorPayload{
			ErrorCode: "handle-exists",
		}, http.StatusBadRequest
	}

	// verify email doesn't yet exist
	userExists, err := emailUnavailableForSignup(db, email)

	if err != nil {
		logging.LogError(r, nil, fmt.Errorf("error checking if user exists: %v", err))
		return nil, http.StatusInternalServerError
	}

	if userExists {
		return ajax.AjaxErrorPayload{
			ErrorCode: "email-exists",
		}, http.StatusBadRequest
	}

	if err := moderateAccountProfile(r.Context(), handle, displayName); err != nil {
		return accountFailure(r, nil, err)
	}

	// hash password
	authHash, err := bcrypt.GenerateFromPassword([]byte(password), passwordBcryptCost)
	if err != nil {
		logging.LogError(r, nil, fmt.Errorf("hashing password: %w", err))
		return nil, http.StatusInternalServerError
	}

	var userID *uint

	err = dbutil.InTransaction(db, func(tx *sql.Tx) error {

		// create user account
		err = tx.QueryRow(
			`INSERT INTO user_account (email, handle, display_name, auth_hash, created_at)
				VALUES ($1, $2, $3, $4, $5)
				RETURNING id`,
			email, handle, displayName, authHash, time.Now(),
		).Scan(&userID)

		if err != nil {
			return fmt.Errorf("error creating user account: %v", err)
		}

		// delete signup request
		_, err = tx.Exec(
			"DELETE FROM user_signup_request WHERE id = $1",
			requestID,
		)

		if err != nil {
			return fmt.Errorf("error deleting signup request: %v", err)
		}

		return nil

	})

	if err != nil {
		logging.LogError(r, nil, fmt.Errorf("error creating user: %v", err))
		return nil, http.StatusInternalServerError
	}

	logging.LogNotice(r, struct {
		Event        string
		UserID       uint
		EmailAddress string
		Handle       string
		DisplayName  string
	}{
		"SignupVerify",
		*userID,
		email,
		handle,
		displayName,
	})

	if err := notifyAdminsOfSignup(db, r, email, handle, displayName, message); err != nil {
		logging.LogError(r, nil, fmt.Errorf("notifying admins of signup for user %d: %w", *userID, err))
	}

	// authenticate immediately
	authUser(w, db, *userID)

	return userID, http.StatusOK

}

func notifyAdminsOfSignup(db *sql.DB, r *http.Request, newUserEmail, handle, displayName, message string) error {
	rows, err := db.Query(`SELECT email FROM user_account WHERE user_role = 'admin' ORDER BY id`)
	if err != nil {
		return fmt.Errorf("loading admin email addresses: %w", err)
	}
	defer rows.Close()

	var adminEmails []string
	for rows.Next() {
		var adminEmail string
		if err := rows.Scan(&adminEmail); err != nil {
			return fmt.Errorf("scanning admin email address: %w", err)
		}
		adminEmails = append(adminEmails, adminEmail)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reading admin email addresses: %w", err)
	}

	msg := emailpkg.Message{
		Subject:    "New Pocket account signup",
		Heading:    "New account created",
		Paragraphs: []string{"A new user just signed up for Pocket."},
		Details: []emailpkg.Detail{
			{Label: "Name", Value: displayName},
			{Label: "Handle", Value: handle},
			{Label: "Email", Value: newUserEmail},
		},
	}
	if message != "" {
		msg.QuoteLabel = "Message from the new user"
		msg.Quote = message
	}
	for _, adminEmail := range adminEmails {
		if err := emailpkg.Send(adminEmail, msg); err != nil {
			logging.LogError(r, nil, fmt.Errorf("sending signup notification to admin %s: %w", adminEmail, err))
		}
	}
	return nil
}
