package manifestblocks

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/manifest"
)

type Features struct {
	// Blocks
	AppHome       *AppHome       `tfsdk:"app_home"`
	BotUser       *BotUser       `tfsdk:"bot_user"`
	Shortcuts     []Shortcut     `tfsdk:"shortcut"`
	SlashCommands []SlashCommand `tfsdk:"slash_command"`
	WorkflowSteps []WorkflowStep `tfsdk:"workflow_step"`

	// Arguments
	UnfurlDomains types.Set `tfsdk:"unfurl_domains"`
}

func (*Features) Schema() *schema.SingleNestedBlock {
	return &schema.SingleNestedBlock{
		MarkdownDescription: "A group of settings corresponding to the **Features** section of the app config pages.",
		Blocks: map[string]schema.Block{
			"app_home":      (*AppHome)(nil).schema(),
			"bot_user":      (*BotUser)(nil).schema(),
			"shortcut":      (*Shortcut)(nil).schema(),
			"slash_command": (*SlashCommand)(nil).schema(),
			"workflow_step": (*WorkflowStep)(nil).schema(),
		},
		Attributes: map[string]schema.Attribute{
			"unfurl_domains": &schema.SetAttribute{
				MarkdownDescription: "An array of strings containing valid [unfurl domains](https://api.slack.com/reference/messaging/link-unfurling#configuring_domains) to register. A maximum of 5 unfurl domains can be included in this array. Please consult the [unfurl docs](https://api.slack.com/reference/messaging/link-unfurling#configuring_domains) for a list of domain requirements.",
				ElementType:         types.StringType,
				Optional:            true,
			},
		},
	}
}

func (f Features) Read() manifest.Features {
	return manifest.Features{
		AppHome:       MapOptionModel[manifest.AppHome](f.AppHome),
		BotUser:       MapOptionModel[manifest.BotUser](f.BotUser),
		Shortcuts:     mapListModel[manifest.Shortcut](f.Shortcuts),
		SlashCommands: mapListModel[manifest.SlashCommand](f.SlashCommands),
		UnfurlDomains: mustStringSetAsArray(&f.UnfurlDomains),
		WorkflowSteps: mapListModel[manifest.WorkflowStep](f.WorkflowSteps),
	}
}
