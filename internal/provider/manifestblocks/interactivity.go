package manifestblocks

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/manifest"
)

type Interactivity struct {
	IsEnabled             types.Bool   `tfsdk:"is_enabled"`
	RequestURL            types.String `tfsdk:"request_url"`
	MessageMenuOptionsURL types.String `tfsdk:"message_menu_options_url"`
}

//declscope:shared // settings.go nests this block
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
