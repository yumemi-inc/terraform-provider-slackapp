package manifestblocks

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/manifest"
)

type Scopes struct {
	Bot  types.Set `tfsdk:"bot"`
	User types.Set `tfsdk:"user"`
}

//declscope:shared // oauth_config.go nests this block
func (*Scopes) schema() *schema.SingleNestedBlock {
	return &schema.SingleNestedBlock{
		MarkdownDescription: "A subgroup of settings that describe [permission scopes](https://api.slack.com/scopes) configuration.",
		Attributes: map[string]schema.Attribute{
			"bot": &schema.SetAttribute{
				MarkdownDescription: "An array of strings containing [bot scopes](https://api.slack.com/scopes) to request upon app installation. A maximum of 255 scopes can included in this array.",
				ElementType:         types.StringType,
				Optional:            true,
				Validators: []validator.Set{
					setvalidator.SizeAtMost(255),
				},
			},
			"user": &schema.SetAttribute{
				MarkdownDescription: "An array of strings containing [user scopes](https://api.slack.com/scopes) to request upon app installation. A maximum of 255 scopes can included in this array.",
				ElementType:         types.StringType,
				Optional:            true,
				Validators: []validator.Set{
					setvalidator.SizeAtMost(255),
				},
			},
		},
	}
}

func (s Scopes) Read() manifest.Scopes {
	return manifest.Scopes{
		Bot:  mustStringSetAsArray(&s.Bot),
		User: mustStringSetAsArray(&s.User),
	}
}
