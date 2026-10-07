// Package planmodifiers holds slackapp_application's plan modifiers:
// SuppressEquivalentManifest for the manifest attribute, and KeepPrior* for
// the computed attributes.
//
// The Slack apps.manifest.export API never returns an app's credentials or OAuth
// authorize URL, so a resource brought under management with `import` has those
// computed attributes as null in state (Read cannot populate them). The stock
// UseStateForUnknown modifier deliberately does nothing when the prior state
// value is null — it can't tell an imported-null apart from a create — so on any
// plan that re-evaluates the resource those attributes go unknown ("known after
// apply") and show a spurious in-place update forever.
//
// KeepPrior* keeps the prior state value for an *existing* resource even when it
// is null, and only falls back to unknown-on-create (letting Create populate it)
// when the whole prior state is null.
package planmodifiers

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// KeepPriorString keeps the prior state value for a computed string attribute of
// an existing resource, including when that value is null.
func KeepPriorString() planmodifier.String {
	return keepPriorStringModifier{}
}

type keepPriorStringModifier struct{}

func (keepPriorStringModifier) Description(_ context.Context) string {
	return "Keep the prior state value on update, even when it is null (imported resources)."
}

func (m keepPriorStringModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (keepPriorStringModifier) PlanModifyString(
	_ context.Context,
	req planmodifier.StringRequest,
	resp *planmodifier.StringResponse,
) {
	// Whole prior state is null => resource is being created; leave the value
	// unknown so Create can populate it.
	if req.State.Raw.IsNull() {
		return
	}
	if req.ConfigValue.IsUnknown() {
		return
	}
	if !resp.PlanValue.IsUnknown() {
		return
	}

	resp.PlanValue = req.StateValue
}

// KeepPriorObject keeps the prior state value for a computed object attribute of
// an existing resource, including when that value is null.
func KeepPriorObject() planmodifier.Object {
	return keepPriorObjectModifier{}
}

type keepPriorObjectModifier struct{}

func (keepPriorObjectModifier) Description(_ context.Context) string {
	return "Keep the prior state value on update, even when it is null (imported resources)."
}

func (m keepPriorObjectModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (keepPriorObjectModifier) PlanModifyObject(
	_ context.Context,
	req planmodifier.ObjectRequest,
	resp *planmodifier.ObjectResponse,
) {
	if req.State.Raw.IsNull() {
		return
	}
	if req.ConfigValue.IsUnknown() {
		return
	}
	if !resp.PlanValue.IsUnknown() {
		return
	}

	resp.PlanValue = req.StateValue
}
