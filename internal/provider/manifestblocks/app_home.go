package manifestblocks

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/manifest"
)

type AppHome struct {
	HomeTabEnabled             types.Bool `tfsdk:"home_tab_enabled"`
	MessagesTabEnabled         types.Bool `tfsdk:"messages_tab_enabled"`
	MessagesTabReadOnlyEnabled types.Bool `tfsdk:"messages_tab_read_only_enabled"`
}

//declscope:shared // features.go nests this block
func (*AppHome) schema() *schema.SingleNestedBlock {
	return &schema.SingleNestedBlock{
		MarkdownDescription: "A subgroup of settings that describe [App Home](https://api.slack.com/surfaces/tabs) configuration.",
		Attributes: map[string]schema.Attribute{
			"home_tab_enabled": &schema.BoolAttribute{
				MarkdownDescription: "A boolean that specifies whether or not the [Home tab](https://api.slack.com/surfaces/tabs) is enabled.",
				Optional:            true,
			},
			"messages_tab_enabled": &schema.BoolAttribute{
				MarkdownDescription: "A boolean that specifies whether or not the [Messages tab in your App Home](https://api.slack.com/surfaces/tabs) is enabled.",
				Optional:            true,
			},
			"messages_tab_read_only_enabled": &schema.BoolAttribute{
				MarkdownDescription: "A boolean that specifies whether or not the users can send messages to your app in the [Messages tab of your App Home](https://api.slack.com/surfaces/tabs).",
				Optional:            true,
			},
		},
	}
}

func (h AppHome) Read() manifest.AppHome {
	return manifest.AppHome{
		HomeTabEnabled:             h.HomeTabEnabled.ValueBoolPointer(),
		MessagesTabEnabled:         h.MessagesTabEnabled.ValueBoolPointer(),
		MessagesTabReadOnlyEnabled: h.MessagesTabReadOnlyEnabled.ValueBoolPointer(),
	}
}
