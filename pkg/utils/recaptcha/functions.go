package recaptcha

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"pocket/pkg/env"
	"pocket/pkg/utils/net"
	"pocket/pkg/utils/secrets"
)

const DefaultMinScore = 0.5

var siteVerifyURL = "https://www.google.com/recaptcha/api/siteverify"

type Response struct {
	Success     bool     `json:"success"`
	Score       float64  `json:"score"`
	Action      string   `json:"action"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	ErrorCodes  []string `json:"error-codes"`
}

func extractToken(r *http.Request) string {
	token := r.FormValue("g-recaptcha-response")
	if token == "" {
		token = r.Header.Get("X-Recaptcha-Token")
	}
	if token == "" {
		token = r.FormValue("recaptcha_token")
	}
	return token
}

func getSecretKey() (string, error) {
	if env.IsAppEngine() {
		secretName := env.GetRecaptchaSecret()
		if secretName == "" {
			return "", fmt.Errorf("reCAPTCHA secret key undefined")
		}
		secretKey, err := secrets.LoadSecret(secretName)
		if err != nil {
			return "", fmt.Errorf("loading reCAPTCHA secret key: %w", err)
		}
		return secretKey, nil
	}

	secretKey := env.GetRecaptchaSecret()
	if secretKey == "" {
		return "", fmt.Errorf("reCAPTCHA secret key undefined")
	}
	return secretKey, nil
}

// Verify sends the token to Google's reCAPTCHA v3 siteverify API and returns the full response.
func Verify(r *http.Request) (*Response, error) {
	token := extractToken(r)
	if token == "" {
		if env.IsLocal() {
			return &Response{
				Success: true,
				Score:   1.0,
			}, nil
		}
		return nil, fmt.Errorf("reCAPTCHA token undefined")
	}

	secretKey, err := getSecretKey()
	if err != nil {
		return nil, err
	}

	ip := net.GetUserIP(r)

	form := url.Values{}
	form.Set("secret", secretKey)
	form.Set("response", token)
	if ip != "" {
		form.Set("remoteip", ip)
	}

	client := http.Client{
		Timeout: 5 * time.Second,
	}

	res, err := client.PostForm(siteVerifyURL, form)
	if err != nil {
		return nil, fmt.Errorf("error fetching URL: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reCAPTCHA verify error: status %d", res.StatusCode)
	}

	var response Response
	err = json.NewDecoder(res.Body).Decode(&response)
	if err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	return &response, nil
}

// VerifyRecaptcha validates that the reCAPTCHA token is valid and score meets DefaultMinScore (0.5).
// An optional expected action name can be provided to ensure the token was generated for the intended action.
func VerifyRecaptcha(r *http.Request, expectedAction ...string) (bool, error) {
	return VerifyRecaptchaWithMinScore(r, DefaultMinScore, expectedAction...)
}

// VerifyRecaptchaWithMinScore validates that the reCAPTCHA token is valid, score meets minScore,
// and optionally verifies the expected action name.
func VerifyRecaptchaWithMinScore(r *http.Request, minScore float64, expectedAction ...string) (bool, error) {
	token := extractToken(r)
	if token == "" && env.IsLocal() {
		// Allow bypass on local
		return true, nil
	}

	resp, err := Verify(r)
	if err != nil {
		return false, err
	}

	if !resp.Success {
		return false, nil
	}

	if len(expectedAction) > 0 && expectedAction[0] != "" && resp.Action != expectedAction[0] {
		return false, nil
	}

	if resp.Score < minScore {
		return false, nil
	}

	return true, nil
}
