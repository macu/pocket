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
}

type mailjetAddress struct {
	Email string `json:"Email"`
	Name  string `json:"Name,omitempty"`
}

// Send sends a plain-text email through Mailjet.
func Send(to, subject, text string) error {
	apiKey := env.GetMailjetApiKey()
	secret := env.GetMailjetSecret()
	from := strings.TrimSpace(env.GetMailjetFromEmail())
	if apiKey == "" || secret == "" || from == "" {
		return nil
	}

	payload, err := json.Marshal(message{
		Messages: []mailjetMessage{{
			From:    mailjetAddress{Email: from, Name: "Pocket"},
			To:      []mailjetAddress{{Email: to}},
			Subject: subject,
			Text:    text,
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
