package email

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"strings"
)

// Colors mirror the site theme in src/styles/vars.scss. Hex values are used
// because email clients have inconsistent support for rgb().
type Colors struct {
	Background      string
	Card            string
	Border          string
	Text            string
	Muted           string
	Brand           string
	BrandText       string
	Accent          string
	QuoteBackground string
	Button          string
	ButtonBorder    string
	ButtonText      string
	Link            string
}

var siteColors = Colors{
	Background:      "#000000", // $app-bg-color
	Card:            "#1C1C24", // $message-bg-color
	Border:          "#3A3A45",
	Text:            "#FFFFFF", // $app-fg-color
	Muted:           "#AAAFBE", // $message-info-color
	Brand:           "#E2670F", // $app-header-bg-color
	BrandText:       "#FFFFFF",
	Accent:          "#E2670F", // $app-separator-color
	QuoteBackground: "#2A2A33",
	Button:          "#5656D3", // $button-primary-bg-color
	ButtonBorder:    "#7878EB", // $button-primary-border-color
	ButtonText:      "#FFFFFF", // $button-solid-fg-color
	Link:            "#8C8CF0",
}

type Action struct {
	Label string
	URL   string
}

type Detail struct {
	Label string
	Value string
}

// Message describes an email's content; it is rendered to both HTML and plain text.
type Message struct {
	Subject    string
	Heading    string
	Paragraphs []string
	Details    []Detail
	QuoteLabel string
	Quote      string
	Action     *Action
	Note       string
}

//go:embed templates/message.html
var messageTemplateSource string

var messageTemplate = template.Must(template.New("message").Parse(messageTemplateSource))

// HTML renders the message using the site-styled HTML template.
// All content is escaped by html/template.
func (m Message) HTML() (string, error) {
	var preheader string
	if len(m.Paragraphs) > 0 {
		preheader = m.Paragraphs[0]
	}
	var actionURL template.URL
	if m.Action != nil {
		if !strings.HasPrefix(m.Action.URL, "https://") && !strings.HasPrefix(m.Action.URL, "http://") {
			return "", fmt.Errorf("email action URL must be absolute http(s)")
		}
		actionURL = template.URL(m.Action.URL)
	}
	data := struct {
		Message
		Preheader string
		Colors    Colors
		Action    *struct {
			Label string
			URL   template.URL
		}
	}{Message: m, Preheader: preheader, Colors: siteColors}
	if m.Action != nil {
		data.Action = &struct {
			Label string
			URL   template.URL
		}{m.Action.Label, actionURL}
	}
	var b bytes.Buffer
	if err := messageTemplate.Execute(&b, data); err != nil {
		return "", fmt.Errorf("rendering email HTML: %w", err)
	}
	return b.String(), nil
}

// Text renders the plain-text alternative of the message.
func (m Message) Text() string {
	var parts []string
	if m.Heading != "" {
		parts = append(parts, m.Heading)
	}
	parts = append(parts, m.Paragraphs...)
	if len(m.Details) > 0 {
		lines := make([]string, len(m.Details))
		for i, d := range m.Details {
			lines[i] = d.Label + ": " + d.Value
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}
	if m.Quote != "" {
		if m.QuoteLabel != "" {
			parts = append(parts, m.QuoteLabel+":\n"+m.Quote)
		} else {
			parts = append(parts, m.Quote)
		}
	}
	if m.Action != nil {
		parts = append(parts, m.Action.Label+":\n"+m.Action.URL)
	}
	if m.Note != "" {
		parts = append(parts, m.Note)
	}
	parts = append(parts, "-- \nThis is an automated message from Pocket.")
	return strings.Join(parts, "\n\n")
}
