package validators_test

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/provider/validators"
)

// validatorsList is a list of n strings, standing for n blocks.
func validatorsList(n int) types.List {
	elements := make([]attr.Value, n)
	for i := range elements {
		elements[i] = types.StringValue("block")
	}

	return types.ListValueMust(types.StringType, elements)
}

func validatorsRun(v validator.List, value types.List) diag.Diagnostics {
	request := validator.ListRequest{Path: path.Root("block"), ConfigValue: value}
	response := &validator.ListResponse{}
	v.ValidateList(context.Background(), request, response)

	return response.Diagnostics
}

func TestCountValidators(t *testing.T) {
	t.Parallel()

	validatorsUnderTest := map[string]struct {
		validator validator.List
		summary   string
	}{
		"CommandCount":         {validators.CommandCount(), "More than 5 slash commands are defined"},
		"MessageShortcutCount": {validators.MessageShortcutCount(), "More than 5 message shortcuts are defined"},
	}

	for name, tc := range validatorsUnderTest {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Slack documents a limit of 5, but accepts more today, so more
			// than 5 is a warning, not an error.
			for _, n := range []int{0, 1, 5} {
				if diags := validatorsRun(tc.validator, validatorsList(n)); len(diags) != 0 {
					t.Errorf("%d blocks: got %v, want no diagnostics", n, diags)
				}
			}

			diags := validatorsRun(tc.validator, validatorsList(6))
			if len(diags) != 1 {
				t.Fatalf("6 blocks: got %d diagnostics, want 1", len(diags))
			}
			if diags[0].Severity() != diag.SeverityWarning {
				t.Errorf("6 blocks: severity = %v, want a warning", diags[0].Severity())
			}
			if diags[0].Summary() != tc.summary {
				t.Errorf("6 blocks: summary = %q", diags[0].Summary())
			}
			if !strings.Contains(diags[0].Detail(), "5 entries") {
				t.Errorf("6 blocks: detail = %q", diags[0].Detail())
			}

			for _, value := range []types.List{types.ListNull(types.StringType), types.ListUnknown(types.StringType)} {
				if diags := validatorsRun(tc.validator, value); len(diags) != 0 {
					t.Errorf("%v: got %v, want no diagnostics", value, diags)
				}
			}

			if d := tc.validator.Description(t.Context()); !strings.Contains(d, "at most 5") {
				t.Errorf("Description() = %q", d)
			}
			if tc.validator.MarkdownDescription(t.Context()) != tc.validator.Description(t.Context()) {
				t.Error("MarkdownDescription() differs from Description()")
			}
		})
	}
}
