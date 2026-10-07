package planmodifiers

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/manifest"
)

// SuppressEquivalentManifest returns a plan modifier that keeps the prior state
// manifest whenever the configured manifest is semantically equal to it (same
// decoded structure, ignoring object-key order, whitespace, and the ordering of
// set-valued scope/event/domain arrays).
//
// The custom ManifestType's StringSemanticEquals only governs how Create/Read/
// Update *results* are reconciled against prior state; terraform-plugin-framework
// does not apply it to the config-vs-state comparison during planning. Without
// this modifier a manifest that is re-serialized differently (e.g. Terraform's
// jsonencode vs the data source's Go json.Marshal) shows as a spurious in-place
// update even though nothing changed. This modifier closes that gap so plans stay
// clean regardless of the byte form the manifest happens to be authored in.
func SuppressEquivalentManifest() planmodifier.String {
	return suppressEquivalentManifest{}
}

type suppressEquivalentManifest struct{}

func (m suppressEquivalentManifest) Description(_ context.Context) string {
	return "Keep the prior state manifest when the configured manifest is semantically equal to it."
}

func (m suppressEquivalentManifest) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m suppressEquivalentManifest) PlanModifyString(
	_ context.Context,
	req planmodifier.StringRequest,
	resp *planmodifier.StringResponse,
) {
	// No prior state (resource creation) or an unknown/absent configuration:
	// nothing to suppress, leave the planned value as-is.
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	equal, err := manifest.Equal(req.StateValue.ValueString(), req.ConfigValue.ValueString())
	if err != nil {
		// Unparseable JSON: fall back to the default plan (show the change).
		return
	}

	if equal {
		resp.PlanValue = req.StateValue
	}
}
