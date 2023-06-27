package common

import (
	"github.com/yumemi-inc/terraform-provider-slackapp/internal/slack"
)

type ProviderContext struct {
	SlackClient *slack.Client
}
