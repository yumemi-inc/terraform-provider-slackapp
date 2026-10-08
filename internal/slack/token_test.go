package slack_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/slack"
)

// A token printed or encoded by accident shows as [redacted], so that it
// cannot reach a log or an error. string(t) still gives the value.
func TestTokenRedacted(t *testing.T) {
	t.Parallel()

	tokens := map[string]any{
		"app configuration token": slack.AppConfigurationToken("xoxe.xoxp-secret"),
		"refresh token":           slack.RefreshToken("xoxe-secret"),
	}

	for name, token := range tokens {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			encoded, err := json.Marshal(map[string]any{"token": token})
			if err != nil {
				t.Fatal(err)
			}

			for _, got := range []string{
				fmt.Sprint(token),
				fmt.Sprintf("%s %v %q %+v %#v", token, token, token, token, token),
				fmt.Sprint(struct{ Token any }{token}),
				string(encoded),
			} {
				if strings.Contains(got, "secret") || !strings.Contains(got, "[redacted]") {
					t.Errorf("printed %q, want [redacted] and no token", got)
				}
			}
		})
	}

	if got := string(slack.AppConfigurationToken("xoxe.xoxp-secret")); got != "xoxe.xoxp-secret" {
		t.Errorf("string() = %q, want the token", got)
	}
}

// A token decodes from Slack's reply as the plain string it is.
func TestTokenUnmarshal(t *testing.T) {
	t.Parallel()

	var reply slack.ToolingTokensRotateResponse
	if err := json.Unmarshal([]byte(`{"ok":true,"token":"xoxe.xoxp-new","refresh_token":"xoxe-new"}`), &reply); err != nil {
		t.Fatal(err)
	}

	if string(reply.Token) != "xoxe.xoxp-new" || string(reply.RefreshToken) != "xoxe-new" {
		t.Errorf("decoded %q and %q", string(reply.Token), string(reply.RefreshToken))
	}
}
