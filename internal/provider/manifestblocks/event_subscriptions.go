package manifestblocks

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/manifest"
)

type EventSubscriptions struct {
	RequestURL types.String `tfsdk:"request_url"`
	BotEvents  types.Set    `tfsdk:"bot_events"`
	UserEvents types.Set    `tfsdk:"user_events"`
}

//declscope:shared // settings.go nests this block
func (*EventSubscriptions) schema() *schema.SingleNestedBlock {
	return &schema.SingleNestedBlock{
		MarkdownDescription: "A subgroup of settings that describe [Events API](https://api.slack.com/events-api) configuration for the app.",
		Attributes: map[string]schema.Attribute{
			"request_url": &schema.StringAttribute{
				MarkdownDescription: "A string containing the full `https` URL that acts as the [Events API request URL](https://api.slack.com/events-api#the-events-api__subscribing-to-event-types__events-api-request-urls). If set, you'll need to manually verify the Request URL in the App Manifest section of [App Management](https://app.slack.com/app-settings).",
				Optional:            true,
			},
			"bot_events": &schema.SetAttribute{
				MarkdownDescription: "An array of strings matching the [event types](https://api.slack.com/events) you want to the app to subscribe to. A maximum of 100 event types can be used.",
				ElementType:         types.StringType,
				Optional:            true,
				Validators: []validator.Set{
					setvalidator.SizeAtMost(100),
				},
			},
			"user_events": &schema.SetAttribute{
				MarkdownDescription: "An array of strings matching the [event types](https://api.slack.com/events) you want to the app to subscribe to on behalf of authorized users. A maximum of 100 event types can be used.",
				ElementType:         types.StringType,
				Optional:            true,
				Validators: []validator.Set{
					setvalidator.SizeAtMost(100),
				},
			},
		},
	}
}

func (s EventSubscriptions) Read() manifest.EventSubscriptions {
	return manifest.EventSubscriptions{
		RequestURL: s.RequestURL.ValueStringPointer(),
		BotEvents:  mustStringSetAsArray(&s.BotEvents),
		UserEvents: mustStringSetAsArray(&s.UserEvents),
	}
}
