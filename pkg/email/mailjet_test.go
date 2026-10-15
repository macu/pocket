package email

import "testing"

func TestBypass(t *testing.T) {
	cases := []struct {
		name      string
		appEngine string
		apiKey    string
		secret    string
		want      bool
	}{
		{"local without keys", "", "", "", true},
		{"local with blank keys", "", "  ", " ", true},
		{"local with keys", "", "key", "secret", false},
		{"local with only API key", "", "key", "", false},
		{"local with only secret", "", "", "secret", false},
		{"app engine without keys", "1", "", "", false},
		{"app engine with keys", "1", "key", "secret", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("APP_ENGINE", c.appEngine)
			t.Setenv("MAILJET_API_KEY", c.apiKey)
			t.Setenv("MAILJET_SECRET", c.secret)
			if got := Bypass(); got != c.want {
				t.Fatalf("Bypass() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestSendFailsInAppEngineWithoutKeys(t *testing.T) {
	t.Setenv("APP_ENGINE", "1")
	t.Setenv("MAILJET_API_KEY", "")
	t.Setenv("MAILJET_SECRET", "")
	if err := Send("user@example.com", Message{Subject: "subject", Paragraphs: []string{"body"}}); err == nil {
		t.Fatal("expected configuration error when not bypassed")
	}
}
