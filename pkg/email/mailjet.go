package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"pocket/pkg/env"
)

const mailjetSendURL = "https://api.mailjet.com/v3.1/send"

type message struct {
	Messages []mailjetMessage `json:"Messages"`
}

type mailjetMessage struct {
	From    mailjetAddress   `json:"From"`
	To      []mailjetAddress `json:"To"`
	Subject string           `json:"Subject"`
	Text    string           `json:"TextPart"`
	HTML    string           `json:"HTMLPart"`
}

type mailjetAddress struct {
	Email string `json:"Email"`
	Name  string `json:"Name,omitempty"`
}

// Bypass reports whether email delivery should be skipped: only when running
// locally with no Mailjet API keys configured. Flows that require an emailed
// link should instead return their token so the frontend can proceed directly.
func Bypass() bool {
	return env.IsLocal() &&
		strings.TrimSpace(env.GetMailjetApiKey()) == "" &&
		strings.TrimSpace(env.GetMailjetSecret()) == ""
}

// Send sends a site-styled HTML email, with a plain-text alternative, through Mailjet.
// It is a no-op when Bypass reports true.
func Send(to string, msg Message) error {
	if Bypass() {
		return nil
	}
	html, err := msg.HTML()
	if err != nil {
		return err
	}
	apiKey := env.GetMailjetApiKey()
	secret := env.GetMailjetSecret()
	from := strings.TrimSpace(env.GetMailjetFromEmail())
	if apiKey == "" || secret == "" || from == "" {
		return fmt.Errorf("Mailjet email configuration is incomplete")
	}

	payload, err := json.Marshal(message{
		Messages: []mailjetMessage{{
			From:    mailjetAddress{Email: from, Name: "Pocket"},
			To:      []mailjetAddress{{Email: to}},
			Subject: msg.Subject,
			Text:    msg.Text(),
			HTML:    html,
		}},
	})
	if err != nil {
		return fmt.Errorf("encoding Mailjet message: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mailjetSendURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("creating Mailjet request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(apiKey, secret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending Mailjet message: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if readErr != nil {
			return fmt.Errorf("Mailjet returned status %d (reading response: %w)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("Mailjet returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	return nil
}
