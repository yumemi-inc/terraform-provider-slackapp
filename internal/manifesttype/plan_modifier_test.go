package manifesttype

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestSuppressEquivalentManifest(t *testing.T) {
	t.Parallel()

	const state = `{"display_information":{"background_color":"#000","description":"d","name":"A"},"oauth_config":{"scopes":{"bot":["chat:write","im:write"]}}}`

	cases := map[string]struct {
		state       basetypes.StringValue
		config      basetypes.StringValue
		wantPlan    string
		wantChanged bool // whether resp.PlanValue was set to state
	}{
		"equal but reserialized -> keep state": {
			state: basetypes.NewStringValue(state),
			// struct-order keys + reversed scope order; semantically identical.
			config:      basetypes.NewStringValue(`{"display_information":{"name":"A","description":"d","background_color":"#000"},"oauth_config":{"scopes":{"bot":["im:write","chat:write"]}}}`),
			wantChanged: true,
			wantPlan:    state,
		},
		"real change -> leave config": {
			state:       basetypes.NewStringValue(state),
			config:      basetypes.NewStringValue(`{"display_information":{"name":"A","description":"CHANGED","background_color":"#000"},"oauth_config":{"scopes":{"bot":["chat:write","im:write"]}}}`),
			wantChanged: false,
		},
		"create (null state) -> leave config": {
			state:       basetypes.NewStringNull(),
			config:      basetypes.NewStringValue(state),
			wantChanged: false,
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			resp := &planmodifier.StringResponse{PlanValue: tc.config}
			SuppressEquivalentManifest().PlanModifyString(
				context.Background(),
				planmodifier.StringRequest{StateValue: tc.state, ConfigValue: tc.config},
				resp,
			)

			if tc.wantChanged {
				if resp.PlanValue.ValueString() != tc.wantPlan {
					t.Fatalf("expected plan kept as prior state %q, got %q", tc.wantPlan, resp.PlanValue.ValueString())
				}
			} else {
				if resp.PlanValue.ValueString() != tc.config.ValueString() {
					t.Fatalf("expected plan left as config %q, got %q", tc.config.ValueString(), resp.PlanValue.ValueString())
				}
			}
		})
	}
}
