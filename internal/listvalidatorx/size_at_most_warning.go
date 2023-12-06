package listvalidatorx

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	"github.com/yumemi-inc/terraform-provider-slackapp/internal/validatordiagx"
)

var _ validator.List = sizeAtMostWarningValidator{}

// sizeAtMostWarningValidator validates that list contains at most max elements.
type sizeAtMostWarningValidator struct {
	max int
}

// Description describes the validation in plain text formatting.
func (v sizeAtMostWarningValidator) Description(_ context.Context) string {
	return fmt.Sprintf("list should contain at most %d elements", v.max)
}

// MarkdownDescription describes the validation in Markdown formatting.
func (v sizeAtMostWarningValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

// ValidateList performs the validation.
func (v sizeAtMostWarningValidator) ValidateList(ctx context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	elems := req.ConfigValue.Elements()

	if len(elems) > v.max {
		resp.Diagnostics.Append(validatordiagx.DiscouragedAttributeValueDiagnostic(
			req.Path,
			v.Description(ctx),
			fmt.Sprintf("%d", len(elems)),
		))
	}
}

// SizeAtMostWarning returns an AttributeValidator which ensures that any configured
// attribute value:
//
//   - Is a List.
//   - Contains at most max elements.
//
// Null (unconfigured) and unknown (known after apply) values are skipped.
func SizeAtMostWarning(max int) validator.List {
	return sizeAtMostWarningValidator{
		max: max,
	}
}
