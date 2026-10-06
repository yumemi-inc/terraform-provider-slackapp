package common

import (
	"github.com/ymm-oss/terraform-provider-slackapp/internal/slack"
)

type ProviderContext struct {
	SlackClient *slack.Client
}
