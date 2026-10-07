package manifestblocks

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/manifest"
)

type OauthConfig struct {
	// Blocks
	Scopes *Scopes `tfsdk:"scopes"`

	// Arguments
	RedirectURLs types.Set `tfsdk:"redirect_urls"`
}

func (*OauthConfig) Schema() *schema.SingleNestedBlock {
	return &schema.SingleNestedBlock{
		MarkdownDescription: "A group of settings describing OAuth configuration for the app.",
		Blocks: map[string]schema.Block{
			"scopes": (*Scopes)(nil).schema(),
		},
		Attributes: map[string]schema.Attribute{
			"redirect_urls": &schema.SetAttribute{
				MarkdownDescription: "An array of strings containing [OAuth redirect URLs](https://api.slack.com/authentication/oauth-v2#asking). A maximum of 1000 redirect URLs can be included in this array.",
				ElementType:         types.StringType,
				Optional:            true,
				Validators: []validator.Set{
					setvalidator.SizeAtMost(1000),
				},
			},
		},
	}
}

func (c OauthConfig) Read() manifest.OauthConfig {
	return manifest.OauthConfig{
		RedirectURLs: mustStringSetAsArray(&c.RedirectURLs),
		Scopes:       MapOptionModel[manifest.Scopes](c.Scopes),
	}
}
