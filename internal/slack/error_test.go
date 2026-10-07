package slack_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/slack"
)

func TestIsAppNotFoundError(t *testing.T) {
	cases := map[string]struct {
		err  error
		want bool
	}{
		"app_not_found":         {&slack.ErrorResponse{Error_: "app_not_found"}, true},
		"wrapped app_not_found": {fmt.Errorf("export: %w", &slack.ErrorResponse{Error_: "app_not_found"}), true},
		"another Slack error":   {&slack.ErrorResponse{Error_: "internal_error"}, false},
		"not a Slack error":     {errors.New("app_not_found"), false},
		"nil":                   {nil, false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := slack.IsAppNotFoundError(tc.err); got != tc.want {
				t.Errorf("IsAppNotFoundError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
