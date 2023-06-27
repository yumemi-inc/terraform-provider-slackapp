package slackappmanifest

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/yumemi-inc/terraform-provider-slackapp/internal/slack/manifest"
)

type DisplayInformation struct {
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	LongDescription types.String `tfsdk:"long_description"`
	BackgroundColor types.String `tfsdk:"background_color"`
}

func (*DisplayInformation) Schema() *schema.SingleNestedBlock {
	return &schema.SingleNestedBlock{
		MarkdownDescription: "A group of settings that describe parts of an app's appearance within Slack. If you're distributing the app via the App Directory, read our [listing guidelines](https://api.slack.com/start/distributing/guidelines#listing) to pick the best values for these settings.",
		Attributes: map[string]schema.Attribute{
			"name": &schema.StringAttribute{
				MarkdownDescription: "A string of the name of the app. Maximum length is 35 characters.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(35),
				},
			},
			"description": &schema.StringAttribute{
				MarkdownDescription: "A string with a short description of the app for display to users. Maximum length is 140 characters.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(140),
				},
			},
			"long_description": &schema.StringAttribute{
				MarkdownDescription: "A string with a longer version of the description of the app. Maximum length is 4000 characters.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(4000),
				},
			},
			"background_color": &schema.StringAttribute{
				MarkdownDescription: "A string containing a hex color value (including the hex sign) that specifies the background color used on hovercards that display information about your app. Can be 3-digit (`#000`) or 6-digit (`#000000`) hex values. Once an app has set a background color value, it cannot be removed, only updated.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile("^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$"),
						"must be #xxx or #xxxxxx format in hexadecimal",
					),
				},
			},
		},
		Validators: []validator.Object{
			objectvalidator.IsRequired(),
		},
	}
}

func (i *DisplayInformation) Read() manifest.DisplayInformation {
	return manifest.DisplayInformation{
		Name:            i.Name.ValueString(),
		Description:     i.Description.ValueStringPointer(),
		LongDescription: i.LongDescription.ValueStringPointer(),
		BackgroundColor: i.BackgroundColor.ValueStringPointer(),
	}
}
