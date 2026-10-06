package slackappmanifest

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/yumemi-inc/terraform-provider-slackapp/internal/slack/manifest"
	"github.com/yumemi-inc/terraform-provider-slackapp/internal/typeconv"
)

type Scopes struct {
	Bot  types.Set `tfsdk:"bot"`
	User types.Set `tfsdk:"user"`
}

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
		Bot:  typeconv.MustStringSetAsArray(&s.Bot),
		User: typeconv.MustStringSetAsArray(&s.User),
	}
}

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
		RedirectURLs: typeconv.MustStringSetAsArray(&c.RedirectURLs),
		Scopes:       typeconv.MapOptionModel[manifest.Scopes](c.Scopes),
	}
}
