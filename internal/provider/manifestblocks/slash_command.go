package manifestblocks

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/manifest"
	"github.com/ymm-oss/terraform-provider-slackapp/internal/provider/validators"
)

type SlashCommand struct {
	Command      types.String `tfsdk:"command"`
	Description  types.String `tfsdk:"description"`
	ShouldEscape types.Bool   `tfsdk:"should_escape"`
	URL          types.String `tfsdk:"url"`
	UsageHint    types.String `tfsdk:"usage_hint"`
}

//declscope:shared // features.go nests this block
func (*SlashCommand) schema() *schema.ListNestedBlock {
	return &schema.ListNestedBlock{
		MarkdownDescription: "An array of settings groups that describe [slash commands](https://api.slack.com/interactivity/slash-commands) configuration. A maximum of 5 slash commands can be included in this array.",
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"command": &schema.StringAttribute{
					MarkdownDescription: "A string containing the actual slash command. Maximum length is 32 characters, and should include the leading / character.",
					Required:            true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(32),
						stringvalidator.RegexMatches(regexp.MustCompile("^/.+$"), "must start with `/`"),
					},
				},
				"description": &schema.StringAttribute{
					MarkdownDescription: "A string containing a description of the slash command that will be displayed to users. Maximum length is 2000 characters.",
					Required:            true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(2000),
					},
				},
				"should_escape": &schema.BoolAttribute{
					MarkdownDescription: "A boolean that specifies whether or not channels, users, and links typed with the slash command should be escaped.",
					Optional:            true,
				},
				"url": &schema.StringAttribute{
					MarkdownDescription: "A string containing the full https URL that acts as the slash command's [request URL](https://api.slack.com/interactivity/slash-commands#creating_commands).",
					Optional:            true,
				},
				"usage_hint": &schema.StringAttribute{
					MarkdownDescription: "A string a short usage hint about the slash command for users. Maximum length is 1000 characters.",
					Optional:            true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(1000),
					},
				},
			},
		},
		Validators: []validator.List{
			validators.CommandCount(),
		},
	}
}

func (c SlashCommand) Read() manifest.SlashCommand {
	return manifest.SlashCommand{
		Command:      c.Command.ValueString(),
		Description:  c.Description.ValueString(),
		ShouldEscape: c.ShouldEscape.ValueBoolPointer(),
		URL:          c.URL.ValueStringPointer(),
		UsageHint:    c.UsageHint.ValueStringPointer(),
	}
}
