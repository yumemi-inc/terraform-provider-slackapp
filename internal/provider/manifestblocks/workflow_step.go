package manifestblocks

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/manifest"
)

type WorkflowStep struct {
	Name       types.String `tfsdk:"name"`
	CallbackID types.String `tfsdk:"callback_id"`
}

//declscope:shared // features.go nests this block
func (*WorkflowStep) schema() *schema.ListNestedBlock {
	return &schema.ListNestedBlock{
		MarkdownDescription: "An array of settings groups that describe [workflow steps](https://api.slack.com/workflows/steps) configuration. A maximum of 10 workflow steps can be included in this array.",
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"name": &schema.StringAttribute{
					MarkdownDescription: "A string containing the name of the workflow step. Maximum length of 50 characters.",
					Required:            true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(50),
					},
				},
				"callback_id": &schema.StringAttribute{
					MarkdownDescription: "A string containing the `callback_id` of the workflow step. Maximum length of 50 characters.",
					Required:            true,
					Validators: []validator.String{
						stringvalidator.LengthAtMost(50),
					},
				},
			},
		},
		Validators: []validator.List{
			listvalidator.SizeAtMost(10),
		},
	}
}

func (s WorkflowStep) Read() manifest.WorkflowStep {
	return manifest.WorkflowStep{
		Name:       s.Name.ValueString(),
		CallbackID: s.CallbackID.ValueString(),
	}
}
