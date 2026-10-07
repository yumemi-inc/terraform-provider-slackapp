package planmodifiers

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// keepPriorState is the whole prior state of a resource: null on create,
// and an object once the resource exists.
func keepPriorState(exists bool) tfsdk.State {
	objectType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"id": tftypes.String}}
	if !exists {
		return tfsdk.State{Raw: tftypes.NewValue(objectType, nil)}
	}

	return tfsdk.State{Raw: tftypes.NewValue(objectType, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "A1"),
	})}
}

func TestKeepPriorString(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		exists bool
		state  types.String
		config types.String
		plan   types.String
		want   types.String
	}{
		"create leaves the value unknown for Create to fill": {
			exists: false, state: types.StringNull(), config: types.StringNull(), plan: types.StringUnknown(),
			want: types.StringUnknown(),
		},
		"an existing resource keeps its prior value": {
			exists: true, state: types.StringValue("https://slack.com/oauth"), config: types.StringNull(), plan: types.StringUnknown(),
			want: types.StringValue("https://slack.com/oauth"),
		},
		"an imported resource keeps its null": {
			exists: true, state: types.StringNull(), config: types.StringNull(), plan: types.StringUnknown(),
			want: types.StringNull(),
		},
		"an unknown config is left alone": {
			exists: true, state: types.StringValue("x"), config: types.StringUnknown(), plan: types.StringUnknown(),
			want: types.StringUnknown(),
		},
		"a known plan is left alone": {
			exists: true, state: types.StringValue("x"), config: types.StringNull(), plan: types.StringValue("y"),
			want: types.StringValue("y"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			response := &planmodifier.StringResponse{PlanValue: tc.plan}
			KeepPriorString().PlanModifyString(context.Background(), planmodifier.StringRequest{
				State:       keepPriorState(tc.exists),
				StateValue:  tc.state,
				ConfigValue: tc.config,
				PlanValue:   tc.plan,
			}, response)

			if !response.PlanValue.Equal(tc.want) {
				t.Errorf("plan = %v, want %v", response.PlanValue, tc.want)
			}
		})
	}
}

func TestKeepPriorObject(t *testing.T) {
	t.Parallel()

	attributeTypes := map[string]attr.Type{"client_secret": types.StringType}
	secret := types.ObjectValueMust(attributeTypes, map[string]attr.Value{"client_secret": types.StringValue("s")})
	other := types.ObjectValueMust(attributeTypes, map[string]attr.Value{"client_secret": types.StringValue("t")})

	cases := map[string]struct {
		exists bool
		state  types.Object
		config types.Object
		plan   types.Object
		want   types.Object
	}{
		"create leaves the value unknown for Create to fill": {
			exists: false, state: types.ObjectNull(attributeTypes), config: types.ObjectNull(attributeTypes), plan: types.ObjectUnknown(attributeTypes),
			want: types.ObjectUnknown(attributeTypes),
		},
		"an existing resource keeps its prior value": {
			exists: true, state: secret, config: types.ObjectNull(attributeTypes), plan: types.ObjectUnknown(attributeTypes),
			want: secret,
		},
		"an imported resource keeps its null": {
			exists: true, state: types.ObjectNull(attributeTypes), config: types.ObjectNull(attributeTypes), plan: types.ObjectUnknown(attributeTypes),
			want: types.ObjectNull(attributeTypes),
		},
		"an unknown config is left alone": {
			exists: true, state: secret, config: types.ObjectUnknown(attributeTypes), plan: types.ObjectUnknown(attributeTypes),
			want: types.ObjectUnknown(attributeTypes),
		},
		"a known plan is left alone": {
			exists: true, state: secret, config: types.ObjectNull(attributeTypes), plan: other,
			want: other,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			response := &planmodifier.ObjectResponse{PlanValue: tc.plan}
			KeepPriorObject().PlanModifyObject(context.Background(), planmodifier.ObjectRequest{
				State:       keepPriorState(tc.exists),
				StateValue:  tc.state,
				ConfigValue: tc.config,
				PlanValue:   tc.plan,
			}, response)

			if !response.PlanValue.Equal(tc.want) {
				t.Errorf("plan = %v, want %v", response.PlanValue, tc.want)
			}
		})
	}
}

func TestPlanModifierDescriptions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	modifiers := map[string]interface {
		Description(context.Context) string
		MarkdownDescription(context.Context) string
	}{
		"KeepPriorString":            KeepPriorString(),
		"KeepPriorObject":            KeepPriorObject(),
		"SuppressEquivalentManifest": SuppressEquivalentManifest(),
	}

	for name, m := range modifiers {
		if m.Description(ctx) == "" {
			t.Errorf("%s has no description", name)
		}
		if m.MarkdownDescription(ctx) != m.Description(ctx) {
			t.Errorf("%s: MarkdownDescription() differs from Description()", name)
		}
	}
}
