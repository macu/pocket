package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"

	emailpkg "pocket/pkg/email"
	"pocket/pkg/moderation"
	"pocket/pkg/utils/ajax"
)

func TestAccountValidation(t *testing.T) {
	for _, name := range []string{"", " \n ", "Name\tHere", "Name\u200b", strings.Repeat("a", 51)} {
		if validateAccountProfile(name, "", "user@example.com") == nil {
			t.Errorf("invalid name accepted: %q", name)
		}
	}
	if err := validateAccountProfile("Jos\u00e9", "valid_handle", "user@example.com"); err != nil {
		t.Fatal(err)
	}
	for _, handle := range []string{"1handle", "has space", strings.Repeat("a", 26)} {
		if validateAccountProfile("Name", handle, "user@example.com") == nil {
			t.Errorf("invalid handle accepted: %q", handle)
		}
	}
	for _, password := range []string{"short", "        ", strings.Repeat("a", 73)} {
		if validAccountPassword(password) {
			t.Errorf("invalid password accepted")
		}
	}
	if !validAccountPassword(strings.Repeat("a", 72)) || !validAccountPassword("password") {
		t.Fatal("valid password rejected")
	}
}

func TestAccountTokenAndLink(t *testing.T) {
	t.Setenv("APP_ENGINE", "1")
	t.Setenv("DOMAIN", "pocket.example")
	t.Setenv("PUBLIC_URL", "")
	first, err := accountToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := accountToken()
	if err != nil {
		t.Fatal(err)
	}
	if first == second || len(first) != 64 || len(accountTokenHash(first)) != 64 || first == accountTokenHash(first) {
		t.Fatal("tokens must be unique and stored as hashes")
	}
	link, err := accountLink("/reset-password", first)
	if err != nil || link != "https://pocket.example/reset-password?token="+first {
		t.Fatalf("unexpected link: %s %v", link, err)
	}
	t.Setenv("DOMAIN", "http://pocket.example")
	if _, err := accountLink("/reset-password", first); err == nil {
		t.Fatal("production links must use HTTPS")
	}
}

func accountTestDB(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("POCKET_ACCOUNT_TEST_DB") != "1" {
		t.Skip("set POCKET_ACCOUNT_TEST_DB=1 and DB_USER/DB_PASS/DB_NAME for isolated PostgreSQL integration tests")
	}
	u := &url.URL{Scheme: "postgres", Host: "localhost:5432", Path: os.Getenv("DB_NAME"),
		User: url.UserPassword(os.Getenv("DB_USER"), os.Getenv("DB_PASS")), RawQuery: "sslmode=disable"}
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal("opening test database failed")
	}
	db.SetMaxOpenConns(1)
	token, err := accountToken()
	if err != nil {
		t.Fatal(err)
	}
	schema := pq.QuoteIdentifier("account_test_" + token[:16])
	if _, err := db.Exec("CREATE SCHEMA " + schema); err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Error(err)
		}
		db.Close()
	})
	if _, err := db.Exec("SET search_path TO " + schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TABLE user_account (id SERIAL PRIMARY KEY, email VARCHAR(50) UNIQUE NOT NULL,
			handle VARCHAR(25) UNIQUE, display_name VARCHAR(50) NOT NULL, auth_hash VARCHAR(60) NOT NULL,
			user_role TEXT NOT NULL DEFAULT 'user', created_at TIMESTAMPTZ NOT NULL DEFAULT now());
		CREATE TABLE user_signup_request (id SERIAL PRIMARY KEY, email VARCHAR(50) NOT NULL,
			token VARCHAR(15) UNIQUE NOT NULL, created_at TIMESTAMPTZ NOT NULL);
		CREATE TABLE user_session (token VARCHAR(30) PRIMARY KEY, user_id INTEGER REFERENCES user_account(id), expires TIMESTAMPTZ NOT NULL);
		CREATE TABLE password_reset_request (id SERIAL PRIMARY KEY, user_id INTEGER REFERENCES user_account(id),
			sent_to_address VARCHAR(50) NOT NULL, token VARCHAR(15) UNIQUE, created_at TIMESTAMPTZ);`); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../sql/migrations/20261014_account_settings.pgsql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(migration)); err != nil {
		t.Fatal(err)
	}
	return db
}

func accountRequest(params url.Values) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(params.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func responseToken(t *testing.T, response interface{}) string {
	t.Helper()
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		BypassEmail bool   `json:"bypassEmail"`
		Token       string `json:"token"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.BypassEmail != (payload.Token != "") {
		t.Fatalf("bypassEmail flag (%v) must accompany a returned token", payload.BypassEmail)
	}
	return payload.Token
}

func TestAccountFlowsPostgres(t *testing.T) {
	db := accountTestDB(t)
	stubProfileModeration(t, func(context.Context, string, string) error { return nil })
	t.Setenv("APP_ENGINE", "0")
	t.Setenv("MAILJET_API_KEY", "")
	t.Setenv("MAILJET_SECRET", "")
	originalSend := sendAccountEmail
	sendAccountEmail = func(to string, msg emailpkg.Message) error {
		t.Fatal("local flows without Mailjet keys must not send email")
		return nil
	}
	t.Cleanup(func() { sendAccountEmail = originalSend })
	hash, err := bcrypt.GenerateFromPassword([]byte("old-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO user_account (email, handle, display_name, auth_hash) VALUES
		('old@example.com', 'first', 'Original', $1), ('other@example.com', 'other', 'Other', $1)`, string(hash)); err != nil {
		t.Fatal(err)
	}
	auth := ajax.Auth{UserID: 1, Role: "user"}
	missingHandle := url.Values{"displayName": {"Updated"}, "handle": {"  "}, "email": {"old@example.com"}}
	if _, status := AjaxSaveAccount(db, auth, httptest.NewRecorder(), accountRequest(missingHandle)); status != http.StatusBadRequest {
		t.Fatalf("empty handle must be rejected, got %d", status)
	}
	var currentHandle string
	if err := db.QueryRow(`SELECT handle FROM user_account WHERE id = 1`).Scan(&currentHandle); err != nil || currentHandle != "first" {
		t.Fatalf("handle cleared by rejected save: %q %v", currentHandle, err)
	}
	params := url.Values{"displayName": {"Updated"}, "handle": {"updated"}, "email": {"new@example.com"}}
	response, status := AjaxSaveAccount(db, auth, httptest.NewRecorder(), accountRequest(params))
	if status != http.StatusOK {
		t.Fatalf("save failed: %d %+v", status, response)
	}
	token := responseToken(t, response)
	var email, storedToken string
	if err := db.QueryRow(`SELECT email FROM user_account WHERE id = 1`).Scan(&email); err != nil || email != "old@example.com" {
		t.Fatalf("email changed before confirmation: %s %v", email, err)
	}
	if err := db.QueryRow(`SELECT token FROM user_email_change_request WHERE user_id = 1`).Scan(&storedToken); err != nil ||
		token == "" || storedToken != accountTokenHash(token) {
		t.Fatal("pending email token not stored securely")
	}
	confirm := accountRequest(url.Values{"token": {token}})
	if _, status := AjaxConfirmEmail(db, nil, httptest.NewRecorder(), confirm); status != http.StatusOK {
		t.Fatalf("confirmation failed: %d", status)
	}
	if _, status := AjaxConfirmEmail(db, nil, httptest.NewRecorder(), accountRequest(url.Values{"token": {token}})); status != http.StatusBadRequest {
		t.Fatal("confirmation token was reused")
	}
	if err := db.QueryRow(`SELECT email FROM user_account WHERE id = 1`).Scan(&email); err != nil || email != "new@example.com" {
		t.Fatal("confirmed email was not applied")
	}
	params.Set("email", email)
	params.Set("handle", "other")
	if response, status := AjaxSaveAccount(db, auth, httptest.NewRecorder(), accountRequest(params)); status != http.StatusBadRequest ||
		response.(ajax.AjaxErrorPayload).ErrorCode != "handle-exists" {
		t.Fatal("duplicate handle must fail")
	}
	params.Set("handle", "updated")
	params.Set("changePassword", "true")
	params.Set("oldPassword", "incorrect")
	params.Set("newPassword", "new-password")
	params.Set("confirmPassword", "new-password")
	if _, status := AjaxSaveAccount(db, auth, httptest.NewRecorder(), accountRequest(params)); status != http.StatusBadRequest {
		t.Fatal("incorrect old password accepted")
	}
	if _, err := db.Exec(`INSERT INTO user_session VALUES ('previous-session', 1, $1)`, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	params.Set("oldPassword", "old-password")
	w := httptest.NewRecorder()
	if response, status := AjaxSaveAccount(db, auth, w, accountRequest(params)); status != http.StatusOK {
		t.Fatalf("password change failed: %d %+v", status, response)
	}
	if len(w.Result().Cookies()) != 1 {
		t.Fatal("password change did not replace session")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_session WHERE token = 'previous-session'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("password change did not revoke old session")
	}
	response, status = AjaxRequestPasswordReset(db, nil, httptest.NewRecorder(), accountRequest(url.Values{"email": {email}}))
	if status != http.StatusOK {
		t.Fatalf("reset request failed: %d", status)
	}
	token = responseToken(t, response)
	resetParams := url.Values{"token": {token}, "newPassword": {"reset-password"}, "confirmPassword": {"reset-password"}}
	w = httptest.NewRecorder()
	if response, status := AjaxResetPassword(db, nil, w, accountRequest(resetParams)); status != http.StatusOK {
		t.Fatalf("password reset failed: %d %+v", status, response)
	}
	if len(w.Result().Cookies()) != 1 {
		t.Fatal("password reset did not log user in")
	}
	if err := db.QueryRow(`SELECT auth_hash FROM user_account WHERE id = 1`).Scan(&storedToken); err != nil ||
		bcrypt.CompareHashAndPassword([]byte(storedToken), []byte("reset-password")) != nil {
		t.Fatal("new password was not saved")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_session WHERE user_id = 1`).Scan(&count); err != nil || count != 1 {
		t.Fatal("reset must revoke previous sessions")
	}
	if _, status := AjaxResetPassword(db, nil, httptest.NewRecorder(), accountRequest(resetParams)); status != http.StatusBadRequest {
		t.Fatal("reset token was reused")
	}
	response, status = AjaxRequestPasswordReset(db, nil, httptest.NewRecorder(), accountRequest(url.Values{"email": {email}}))
	token = responseToken(t, response)
	if status != http.StatusOK || token == "" {
		t.Fatal("local reset token missing")
	}
	if _, err := db.Exec(`UPDATE password_reset_request SET created_at = $1`, time.Now().Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	resetParams.Set("token", token)
	if _, status := AjaxResetPassword(db, nil, httptest.NewRecorder(), accountRequest(resetParams)); status != http.StatusBadRequest {
		t.Fatal("expired reset token accepted")
	}
	response, status = AjaxRequestPasswordReset(db, nil, httptest.NewRecorder(), accountRequest(url.Values{"email": {"missing@example.com"}}))
	if status != http.StatusOK || responseToken(t, response) != "" {
		t.Fatal("unknown account should receive generic success without token")
	}
	if _, err := db.Exec(`UPDATE user_account SET user_role = 'banned' WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	response, status = AjaxRequestPasswordReset(db, nil, httptest.NewRecorder(), accountRequest(url.Values{"email": {email}}))
	if status != http.StatusOK || responseToken(t, response) != "" {
		t.Fatal("banned account must not receive a reset token")
	}
}

func TestProductionAccountEmailsPostgres(t *testing.T) {
	db := accountTestDB(t)
	stubProfileModeration(t, func(context.Context, string, string) error { return nil })
	t.Setenv("APP_ENGINE", "1")
	t.Setenv("DOMAIN", "pocket.example")
	t.Setenv("PUBLIC_URL", "")
	originalSend := sendAccountEmail
	type sentMessage struct{ to, subject, body, link string }
	var messages []sentMessage
	sendAccountEmail = func(to string, msg emailpkg.Message) error {
		if _, err := msg.HTML(); err != nil {
			t.Fatalf("rendering email HTML: %v", err)
		}
		link := ""
		if msg.Action != nil {
			link = msg.Action.URL
		}
		messages = append(messages, sentMessage{to, msg.Subject, msg.Text(), link})
		return nil
	}
	t.Cleanup(func() { sendAccountEmail = originalSend })
	if _, err := db.Exec(`INSERT INTO user_account (email, display_name, auth_hash)
		VALUES ('original@example.com', 'Original', 'unused')`); err != nil {
		t.Fatal(err)
	}
	params := url.Values{"displayName": {"Updated"}, "handle": {"updated"}, "email": {"changed@example.com"}}
	response, status := AjaxSaveAccount(db, ajax.Auth{UserID: 1, Role: "user"}, httptest.NewRecorder(), accountRequest(params))
	if status != http.StatusOK || responseToken(t, response) != "" || len(messages) != 1 {
		t.Fatal("production settings must send an email and never return a token")
	}
	if messages[0].to != "changed@example.com" || !strings.Contains(messages[0].body, "https://pocket.example/confirm-email?token=") {
		t.Fatal("confirmation email has wrong address or URL")
	}
	link, err := url.Parse(messages[0].link)
	if err != nil {
		t.Fatal(err)
	}
	token := link.Query().Get("token")
	if _, status := AjaxConfirmEmail(db, nil, httptest.NewRecorder(), accountRequest(url.Values{"token": {token}})); status != http.StatusOK {
		t.Fatal("emailed confirmation token did not work")
	}
	response, status = AjaxRequestPasswordReset(db, nil, httptest.NewRecorder(), accountRequest(url.Values{"email": {"changed@example.com"}}))
	if status != http.StatusOK || responseToken(t, response) != "" || len(messages) != 2 {
		t.Fatal("production reset must send an email and never return a token")
	}
	if messages[1].to != "changed@example.com" || !strings.Contains(messages[1].body, "https://pocket.example/reset-password?token=") {
		t.Fatal("password reset email has wrong address or URL")
	}
	if _, status := AjaxRequestPasswordReset(db, nil, httptest.NewRecorder(), accountRequest(url.Values{"email": {"changed@example.com"}})); status != http.StatusOK ||
		len(messages) != 2 {
		t.Fatal("reset email cooldown must prevent duplicate delivery")
	}
}

func TestValidDisplayName(t *testing.T) {
	for _, name := range []string{"Matt", "  Jos\u00e9 Smith  ", strings.Repeat("\u00e9", 50),
		"  Matt    Smith  ", strings.Repeat("a", 48) + "    b"} {
		if !validDisplayName(name) {
			t.Errorf("valid display name rejected: %q", name)
		}
	}
	for _, name := range []string{"", "   ", "Matt\n", "\tMatt", "Ma\u0000tt", "Ma\u200btt", "Matt\u2028",
		"Matt\u00a0Smith", "\xff", strings.Repeat("a", 51)} {
		if validDisplayName(name) {
			t.Errorf("invalid display name accepted: %q", name)
		}
	}
}

func stubProfileModeration(t *testing.T, check func(context.Context, string, string) error) {
	t.Helper()
	original := moderateAccountProfile
	moderateAccountProfile = check
	t.Cleanup(func() { moderateAccountProfile = original })
}

func TestSaveAccountModerationFailure(t *testing.T) {
	t.Setenv("APP_ENGINE", "0")
	for _, test := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"rejected", moderation.ErrRejected, http.StatusUnprocessableEntity, "profile-moderation-rejected"},
		{"unavailable", errors.New("moderation unavailable"), http.StatusInternalServerError, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			called := false
			stubProfileModeration(t, func(ctx context.Context, handle, name string) error {
				called = true
				if handle != "Matt_99" || name != "Matt Smith" {
					t.Fatalf("moderating unnormalized profile: %q %q", handle, name)
				}
				return test.err
			})
			params := url.Values{"displayName": {"  Matt   Smith  "}, "handle": {" Matt_99 "}, "email": {" user@example.com "}}
			payload, status := AjaxSaveAccount(nil, ajax.Auth{UserID: 1}, httptest.NewRecorder(), accountRequest(params))
			if !called || status != test.status {
				t.Fatalf("called=%v, status=%d, want %d", called, status, test.status)
			}
			if test.code != "" && payload.(ajax.AjaxErrorPayload).ErrorCode != test.code {
				t.Fatalf("unexpected error payload: %+v", payload)
			}
		})
	}
}

func TestProfileRejectsNonPrintableNames(t *testing.T) {
	stubProfileModeration(t, func(context.Context, string, string) error {
		t.Fatal("invalid names must be rejected before moderation")
		return nil
	})
	for _, name := range []string{"\tMatt", "Matt\n", "Ma\u200btt", "Matt\u00a0Smith", "Ma\x00tt"} {
		params := url.Values{"displayName": {name}, "handle": {"Matt"}, "email": {"user@example.com"},
			"token": {"signup-token"}, "password": {"valid-password"}}
		for _, signup := range []bool{false, true} {
			var payload interface{}
			var status int
			if signup {
				payload, status = AjaxSignupVerify(nil, nil, httptest.NewRecorder(), accountRequest(params))
			} else {
				payload, status = AjaxSaveAccount(nil, ajax.Auth{UserID: 1}, httptest.NewRecorder(), accountRequest(params))
			}
			if status != http.StatusBadRequest || payload.(ajax.AjaxErrorPayload).ErrorCode != "invalid-name" {
				t.Errorf("name %q, signup=%v: status=%d payload=%+v", name, signup, status, payload)
			}
		}
	}
}

func TestProfileModerationPostgres(t *testing.T) {
	db := accountTestDB(t)
	t.Setenv("APP_ENGINE", "0")
	if _, err := db.Exec(`INSERT INTO user_signup_request (email, token, created_at)
				VALUES ('user@example.com', 'signup-token', now())`); err != nil {
		t.Fatal(err)
	}
	var moderationErr error
	var checkedHandle, checkedName string
	stubProfileModeration(t, func(ctx context.Context, handle, name string) error {
		checkedHandle, checkedName = handle, name
		return moderationErr
	})
	params := url.Values{"token": {" signup-token "}, "password": {"valid-password"},
		"handle": {" Matt_99 "}, "displayName": {"  Matt   Smith  "}}
	for _, failure := range []struct {
		err    error
		status int
	}{
		{moderation.ErrRejected, http.StatusUnprocessableEntity},
		{errors.New("moderation unavailable"), http.StatusInternalServerError},
	} {
		moderationErr = failure.err
		if _, status := AjaxSignupVerify(db, nil, httptest.NewRecorder(), accountRequest(params)); status != failure.status {
			t.Fatalf("signup moderation failure: status=%d, want %d", status, failure.status)
		}
		var count int
		if err := db.QueryRow(`SELECT count(*) FROM user_account`).Scan(&count); err != nil || count != 0 {
			t.Fatalf("failed moderation created an account: count=%d err=%v", count, err)
		}
		if err := db.QueryRow(`SELECT count(*) FROM user_signup_request`).Scan(&count); err != nil || count != 1 {
			t.Fatalf("failed moderation consumed signup token: count=%d err=%v", count, err)
		}
	}
	moderationErr = nil
	if payload, status := AjaxSignupVerify(db, nil, httptest.NewRecorder(), accountRequest(params)); status != http.StatusOK {
		t.Fatalf("signup failed: %d %+v", status, payload)
	}
	if checkedHandle != "Matt_99" || checkedName != "Matt Smith" {
		t.Fatalf("unexpected moderated signup profile: %q %q", checkedHandle, checkedName)
	}
	var id uint
	var handle, name string
	if err := db.QueryRow(`SELECT id, handle, display_name FROM user_account`).Scan(&id, &handle, &name); err != nil ||
		handle != checkedHandle || name != checkedName {
		t.Fatalf("unexpected saved signup profile: %q %q %v", handle, name, err)
	}
	params.Set("displayName", "  New    Name  ")
	params.Set("handle", " New_handle ")
	params.Set("email", " USER@example.com ")
	auth := ajax.Auth{UserID: id, Role: "user"}
	moderationErr = moderation.ErrRejected
	if _, status := AjaxSaveAccount(db, auth, httptest.NewRecorder(), accountRequest(params)); status != http.StatusUnprocessableEntity {
		t.Fatalf("account moderation rejection: status=%d", status)
	}
	if err := db.QueryRow(`SELECT handle, display_name FROM user_account WHERE id = $1`, id).Scan(&handle, &name); err != nil ||
		handle != "Matt_99" || name != "Matt Smith" {
		t.Fatalf("rejected update changed profile: %q %q %v", handle, name, err)
	}
	moderationErr = nil
	if payload, status := AjaxSaveAccount(db, auth, httptest.NewRecorder(), accountRequest(params)); status != http.StatusOK {
		t.Fatalf("account save failed: %d %+v", status, payload)
	}
	if err := db.QueryRow(`SELECT handle, display_name FROM user_account WHERE id = $1`, id).Scan(&handle, &name); err != nil ||
		handle != "New_handle" || name != "New Name" || handle != checkedHandle || name != checkedName {
		t.Fatalf("unexpected saved account profile: %q %q %v", handle, name, err)
	}
}

func TestValidHandle(t *testing.T) {
	for _, handle := range []string{"a", "Matt_99", "a" + strings.Repeat("b", userHandleMaxLength-1)} {
		if !validHandle(handle) {
			t.Errorf("valid handle rejected: %q", handle)
		}
	}
	for _, handle := range []string{"", "_matt", "9matt", "matt smith", "matt-smith", "matt\n", "m\u00e9tt", "matt\u200b",
		"a" + strings.Repeat("b", userHandleMaxLength)} {
		if validHandle(handle) {
			t.Errorf("invalid handle accepted: %q", handle)
		}
	}
}
