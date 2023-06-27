package slackappmanifest

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/yumemi-inc/terraform-provider-slackapp/internal/slack/manifest"
	"github.com/yumemi-inc/terraform-provider-slackapp/internal/typeconv"
)

type EventSubscription struct {
	RequestURL types.String `tfsdk:"request_url"`
	BotEvents  types.Set    `tfsdk:"bot_events"`
	UserEvents types.Set    `tfsdk:"user_events"`
}

func (*EventSubscription) schema() *schema.ListNestedBlock {
	return &schema.ListNestedBlock{
		MarkdownDescription: "A subgroup of settings that describe [Events API](https://api.slack.com/events-api) configuration for the app.",
		NestedObject: schema.NestedBlockObject{
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
		},
	}
}

func (s EventSubscription) Read() manifest.EventSubscription {
	return manifest.EventSubscription{
		RequestURL: s.RequestURL.ValueStringPointer(),
		BotEvents:  typeconv.MustStringSetAsArray(&s.BotEvents),
		UserEvents: typeconv.MustStringSetAsArray(&s.UserEvents),
	}
}

type Interactivity struct {
	IsEnabled             types.Bool   `tfsdk:"is_enabled"`
	RequestURL            types.String `tfsdk:"request_url"`
	MessageMenuOptionsURL types.String `tfsdk:"message_menu_options_url"`
}

func (*Interactivity) schema() *schema.SingleNestedBlock {
	return &schema.SingleNestedBlock{
		MarkdownDescription: "A subgroup of settings that describe [interactivity](https://api.slack.com/interactivity) configuration for the app.",
		Attributes: map[string]schema.Attribute{
			"is_enabled": &schema.BoolAttribute{
				MarkdownDescription: "A boolean that specifies whether or not interactivity features are enabled.",
				Optional:            true,
			},
			"request_url": &schema.StringAttribute{
				MarkdownDescription: "A string containing the full `https` URL that acts as the [interactive **Request URL**](https://api.slack.com/interactivity/handling#setup).",
				Optional:            true,
			},
			"message_menu_options_url": &schema.StringAttribute{
				MarkdownDescription: "A string containing the full `https` URL that acts as the [interactive **Options Load URL**](https://api.slack.com/interactivity/handling#setup).",
				Optional:            true,
			},
		},
		Validators: []validator.Object{
			objectvalidator.AlsoRequires(
				path.MatchRelative().AtName("is_enabled"),
			),
		},
	}
}

func (i Interactivity) Read() manifest.Interactivity {
	return manifest.Interactivity{
		IsEnabled:             i.IsEnabled.ValueBool(),
		RequestURL:            i.RequestURL.ValueStringPointer(),
		MessageMenuOptionsURL: i.MessageMenuOptionsURL.ValueStringPointer(),
	}
}

type Settings struct {
	// Blocks
	EventSubscriptions []EventSubscription `tfsdk:"event_subscription"`
	Interactivity      *Interactivity      `tfsdk:"interactivity"`

	// Arguments
	AllowedIPAddressRanges types.Set  `tfsdk:"allowed_ip_address_ranges"`
	OrgDeployEnabled       types.Bool `tfsdk:"org_deploy_enabled"`
	SocketModeEnabled      types.Bool `tfsdk:"socket_mode_enabled"`
}

func (*Settings) Schema() *schema.SingleNestedBlock {
	return &schema.SingleNestedBlock{
		MarkdownDescription: "A group of settings corresponding to the **Settings** section of the app config pages.",
		Blocks: map[string]schema.Block{
			"event_subscription": (*EventSubscription)(nil).schema(),
			"interactivity":      (*Interactivity)(nil).schema(),
		},
		Attributes: map[string]schema.Attribute{
			"allowed_ip_address_ranges": &schema.SetAttribute{
				MarkdownDescription: "An array of strings that contain IP addresses that conform to the [Allowed IP Ranges feature](https://api.slack.com/authentication/best-practices#ip_allowlisting).",
				ElementType:         types.StringType,
				Optional:            true,
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(
						stringvalidator.RegexMatches(
							regexp.MustCompile(`^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}(?:/[0-9]{1,2})?$`),
							"must be a valid IPv4 address or CIDR block",
						),
					),
				},
			},
			"org_deploy_enabled": &schema.BoolAttribute{
				MarkdownDescription: "A boolean that specifies whether or not [org-wide deploy](https://api.slack.com/enterprise/apps) is enabled.",
				Optional:            true,
			},
			"socket_mode_enabled": &schema.BoolAttribute{
				MarkdownDescription: "A boolean that specifies whether or not [Socket Mode](https://api.slack.com/apis/connections/socket) is enabled.",
				Optional:            true,
			},
		},
	}
}

func (s Settings) Read() manifest.Settings {
	return manifest.Settings{
		AllowedIPAddressRanges: typeconv.MustStringSetAsArray(&s.AllowedIPAddressRanges),
		EventSubscriptions:     typeconv.MapListModel[manifest.EventSubscription](s.EventSubscriptions),
		Interactivity:          typeconv.MapOptionModel[manifest.Interactivity](s.Interactivity),
		OrgDeployEnabled:       s.OrgDeployEnabled.ValueBoolPointer(),
		SocketModeEnabled:      s.SocketModeEnabled.ValueBoolPointer(),
	}
}
