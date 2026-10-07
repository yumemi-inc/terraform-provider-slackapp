package manifestblocks

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/manifest"
)

type BotUser struct {
	DisplayName  types.String `tfsdk:"display_name"`
	AlwaysOnline types.Bool   `tfsdk:"always_online"`
}

//declscope:shared // features.go nests this block
func (*BotUser) schema() *schema.SingleNestedBlock {
	return &schema.SingleNestedBlock{
		MarkdownDescription: "A subgroup of settings that describe [bot user](https://api.slack.com/bot-users) configuration.",
		Attributes: map[string]schema.Attribute{
			"display_name": &schema.StringAttribute{
				MarkdownDescription: "A string containing the display name of the bot user. Maximum length is 80 characters. Allowed characters: `a-z`, `A-Z`, `0-9`, `-`, `_`, `.`, and space.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(80),
					stringvalidator.RegexMatches(
						regexp.MustCompile("^[0-9a-zA-Z-_. ]+$"),
						"must be a string that contains only `a-z`, `A-Z`, `0-9`, `-`, `_`, `.`, and space",
					),
				},
			},
			"always_online": &schema.BoolAttribute{
				MarkdownDescription: "A boolean that specifies whether or not the bot user will always appear to be online.",
				Optional:            true,
			},
		},
		Validators: []validator.Object{
			objectvalidator.AlsoRequires(
				path.MatchRelative().AtName("display_name"),
			),
		},
	}
}

func (u BotUser) Read() manifest.BotUser {
	return manifest.BotUser{
		DisplayName:  u.DisplayName.ValueString(),
		AlwaysOnline: u.AlwaysOnline.ValueBoolPointer(),
	}
}
