package manifestblocks

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/manifest"
)

type Settings struct {
	// Blocks
	EventSubscriptions *EventSubscriptions `tfsdk:"event_subscriptions"`
	Interactivity      *Interactivity      `tfsdk:"interactivity"`

	// Arguments
	AllowedIPAddressRanges types.Set  `tfsdk:"allowed_ip_address_ranges"`
	OrgDeployEnabled       types.Bool `tfsdk:"org_deploy_enabled"`
	SocketModeEnabled      types.Bool `tfsdk:"socket_mode_enabled"`
	TokenRotationEnabled   types.Bool `tfsdk:"token_rotation_enabled"`
}

func (*Settings) Schema() *schema.SingleNestedBlock {
	return &schema.SingleNestedBlock{
		MarkdownDescription: "A group of settings corresponding to the **Settings** section of the app config pages.",
		Blocks: map[string]schema.Block{
			"event_subscriptions": (*EventSubscriptions)(nil).schema(),
			"interactivity":       (*Interactivity)(nil).schema(),
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
			"token_rotation_enabled": &schema.BoolAttribute{
				MarkdownDescription: "A boolean that specifies whether or not [token rotation](https://api.slack.com/authentication/rotation) is enabled.",
				Optional:            true,
			},
		},
	}
}

func (s Settings) Read() manifest.Settings {
	return manifest.Settings{
		AllowedIPAddressRanges: mustStringSetAsArray(&s.AllowedIPAddressRanges),
		EventSubscriptions:     MapOptionModel[manifest.EventSubscriptions](s.EventSubscriptions),
		Interactivity:          MapOptionModel[manifest.Interactivity](s.Interactivity),
		OrgDeployEnabled:       s.OrgDeployEnabled.ValueBoolPointer(),
		SocketModeEnabled:      s.SocketModeEnabled.ValueBoolPointer(),
		TokenRotationEnabled:   s.TokenRotationEnabled.ValueBoolPointer(),
	}
}
