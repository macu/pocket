package email

import (
	"strings"
	"testing"
)

func TestMessageHTMLEscapesContentAndUsesSiteColors(t *testing.T) {
	msg := Message{
		Subject:    "Subject <b>",
		Heading:    "Heading & more",
		Paragraphs: []string{"<script>alert(1)</script>"},
		Details:    []Detail{{Label: "Name", Value: "<img src=x>"}},
		QuoteLabel: "Deleted post",
		Quote:      "line one\n<a href=\"javascript:alert(1)\">x</a>",
		Action:     &Action{Label: "Go", URL: "https://pocket.example/reset-password?token=abc&x=1"},
		Note:       "Ignore if not you.",
	}
	html, err := msg.HTML()
	if err != nil {
		t.Fatal(err)
	}
	for _, unsafe := range []string{"<script>", "<img src=x>", `<a href="javascript:`} {
		if strings.Contains(html, unsafe) {
			t.Errorf("HTML contains unescaped content %q", unsafe)
		}
	}
	for _, want := range []string{
		"&lt;script&gt;", "Heading &amp; more", "Deleted post",
		`href="https://pocket.example/reset-password?token=abc&amp;x=1"`,
		siteColors.Brand, siteColors.Button, siteColors.Background,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
}

func TestMessageHTMLRejectsUnsafeActionURL(t *testing.T) {
	for _, u := range []string{"javascript:alert(1)", "/relative", ""} {
		if _, err := (Message{Action: &Action{Label: "Go", URL: u}}).HTML(); err == nil {
			t.Errorf("action URL %q accepted", u)
		}
	}
}

func TestMessageText(t *testing.T) {
	text := Message{
		Heading:    "Reset your password",
		Paragraphs: []string{"First.", "Second."},
		Details:    []Detail{{Label: "Name", Value: "Matt"}},
		QuoteLabel: "Post",
		Quote:      "Hello",
		Action:     &Action{Label: "Reset password", URL: "https://pocket.example/x"},
		Note:       "Ignore if not you.",
	}.Text()
	want := "Reset your password\n\nFirst.\n\nSecond.\n\nName: Matt\n\nPost:\nHello\n\n" +
		"Reset password:\nhttps://pocket.example/x\n\nIgnore if not you.\n\n-- \nThis is an automated message from Pocket."
	if text != want {
		t.Fatalf("unexpected text:\n%s", text)
	}
}
