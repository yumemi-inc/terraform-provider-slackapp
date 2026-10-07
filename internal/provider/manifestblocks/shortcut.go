package manifestblocks

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/manifest"
	"github.com/ymm-oss/terraform-provider-slackapp/internal/provider/validators"
)

type Shortcut struct {
	Name        types.String `tfsdk:"name"`
	CallbackID  types.String `tfsdk:"callback_id"`
	Description types.String `tfsdk:"description"`
	Type        types.String `tfsdk:"type"`
}

//declscope:shared // features.go nests this block
func (*Shortcut) schema() *schema.ListNestedBlock {
	return &schema.ListNestedBlock{
		MarkdownDescription: "An array of settings groups that describe [shortcuts](https://api.slack.com/interactivity/shortcuts) configuration. A maximum of 5 shortcuts can be included in this array.",
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"name": &schema.StringAttribute{
					MarkdownDescription: "A string containing the name of the shortcut.",
					Required:            true,
				},
				"callback_id": &schema.StringAttribute{
					MarkdownDescription: "A string containing the `callback_id` of this shortcut. Maximum length is 255 characters.",
					Required:            true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(255),
					},
				},
				"description": &schema.StringAttribute{
					MarkdownDescription: "A string containing a short description of this shortcut. Maximum length is 150 characters.",
					Required:            true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(150),
					},
				},
				"type": &schema.StringAttribute{
					MarkdownDescription: "A string containing one of `message` or `global`. This specifies which [type of shortcut](https://api.slack.com/interactivity/shortcuts) is being described.",
					Required:            true,
					Validators: []validator.String{
						stringvalidator.OneOf("message", "global"),
					},
				},
			},
		},
		Validators: []validator.List{
			validators.MessageShortcutCount(),
		},
	}
}

func (s Shortcut) Read() manifest.Shortcut {
	return manifest.Shortcut{
		Name:        s.Name.ValueString(),
		CallbackID:  s.CallbackID.ValueString(),
		Description: s.Description.ValueString(),
		Type:        manifest.ShortcutType(s.Type.ValueString()),
	}
}
