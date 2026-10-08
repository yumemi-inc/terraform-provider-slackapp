package tokens_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/slack/tokens"
)

// A token printed or encoded by accident shows as [redacted], so that it
// cannot reach a log or an error. string(t) still gives the value.
func TestTokenRedacted(t *testing.T) {
	t.Parallel()

	values := map[string]any{
		"app configuration token": tokens.AppConfigurationToken("xoxe.xoxp-secret"),
		"refresh token":           tokens.RefreshToken("xoxe-secret"),
	}

	for name, token := range values {
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

	if got := string(tokens.AppConfigurationToken("xoxe.xoxp-secret")); got != "xoxe.xoxp-secret" {
		t.Errorf("string() = %q, want the token", got)
	}
}
