package manifesttype

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// The comparison itself is manifest.Equal's, tested there. These check what
// the type adds around it.
func TestStringSemanticEquals(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		prior string
		next  string
		want  bool
	}{
		"compared by manifest.Equal": {
			prior: `{"display_information":{"name":"A","description":"d"}}`,
			next:  `{"display_information":{"description":"d","name":"A"}}`,
			want:  true,
		},
		"a real change is not equal": {
			prior: `{"display_information":{"name":"A"}}`,
			next:  `{"display_information":{"name":"B"}}`,
			want:  false,
		},
		"invalid JSON falls back to comparing the strings": {
			prior: `not json`,
			next:  `not json`,
			want:  true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, diags := NewManifestValue(tc.prior).StringSemanticEquals(context.Background(), NewManifestValue(tc.next))
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			if got != tc.want {
				t.Fatalf("StringSemanticEquals(%q, %q) = %v, want %v", tc.prior, tc.next, got, tc.want)
			}
		})
	}
}

func TestStringSemanticEqualsNullUnknown(t *testing.T) {
	t.Parallel()

	known := NewManifestValue(`{"display_information":{"name":"A"}}`)
	null := Manifest{StringValue: basetypes.NewStringNull()}
	unknown := Manifest{StringValue: basetypes.NewStringUnknown()}

	if eq, _ := known.StringSemanticEquals(context.Background(), null); eq {
		t.Fatal("known vs null should not be equal")
	}
	if eq, _ := null.StringSemanticEquals(context.Background(), null); !eq {
		t.Fatal("null vs null should be equal")
	}
	if eq, _ := unknown.StringSemanticEquals(context.Background(), unknown); !eq {
		t.Fatal("unknown vs unknown should be equal")
	}
}
