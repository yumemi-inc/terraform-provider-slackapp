package myvalidator

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

const maxCommandCount = 5

type commandCountValidator struct{}

func CommandCount() validator.List {
	return commandCountValidator{}
}

func (v commandCountValidator) Description(context.Context) string {
	return fmt.Sprintf("list should contain at most %d commands", maxCommandCount)
}

func (v commandCountValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v commandCountValidator) ValidateList(_ context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	elems := req.ConfigValue.Elements()

	if len(elems) > maxCommandCount {
		resp.Diagnostics.Append(diag.NewAttributeWarningDiagnostic(
			req.Path,
			"More than 5 slash commands are defined",
			fmt.Sprintf(
				"Exceeding Slack's official limit of %d entries for commands may work now, but could be restricted in future updates.",
				maxCommandCount,
			),
		))
	}
}
