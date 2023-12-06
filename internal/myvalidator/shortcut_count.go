package myvalidator

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

const maxShortcutCount = 5

type shortcutCountValidator struct{}

func MessageShortcutCount() validator.List {
	return shortcutCountValidator{}
}

func (v shortcutCountValidator) Description(context.Context) string {
	return fmt.Sprintf("list should contain at most %d shortcuts", maxShortcutCount)
}

func (v shortcutCountValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v shortcutCountValidator) ValidateList(_ context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	elems := req.ConfigValue.Elements()

	if len(elems) > maxShortcutCount {
		resp.Diagnostics.Append(diag.NewAttributeWarningDiagnostic(
			req.Path,
			"More than 5 message shortcuts are defined",
			fmt.Sprintf(
				"Exceeding Slack's official limit of %d entries for shortcuts may work now, but could be restricted in future updates.",
				maxShortcutCount,
			),
		))
	}
}
