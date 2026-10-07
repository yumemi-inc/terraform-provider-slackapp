package manifesttype

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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

func TestManifestType(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	typ := ManifestType{}

	if !typ.Equal(ManifestType{}) {
		t.Error("ManifestType is not equal to itself")
	}
	if typ.Equal(basetypes.StringType{}) {
		t.Error("ManifestType equals a plain string type")
	}
	if typ.String() != "manifesttype.ManifestType" {
		t.Errorf("String() = %q", typ.String())
	}
	if _, ok := typ.ValueType(ctx).(Manifest); !ok {
		t.Errorf("ValueType() is %T, want Manifest", typ.ValueType(ctx))
	}

	value, err := typ.ValueFromTerraform(ctx, tftypes.NewValue(tftypes.String, `{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	manifest, ok := value.(Manifest)
	if !ok || manifest.ValueString() != `{"a":1}` {
		t.Errorf("ValueFromTerraform returned %#v", value)
	}

	if _, err := typ.ValueFromTerraform(ctx, tftypes.NewValue(tftypes.Number, 1)); err == nil {
		t.Error("ValueFromTerraform accepted a number")
	}
}

func TestManifestValue(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	v := NewManifestValue(`{"a":1}`)

	if _, ok := v.Type(ctx).(ManifestType); !ok {
		t.Errorf("Type() is %T, want ManifestType", v.Type(ctx))
	}
	if !v.Equal(NewManifestValue(`{"a":1}`)) {
		t.Error("a value is not equal to the same string")
	}
	// Equal compares the strings; meaning is StringSemanticEquals' job.
	if v.Equal(NewManifestValue(`{ "a": 1 }`)) {
		t.Error("Equal ignored formatting")
	}
	if v.Equal(basetypes.NewStringValue(`{"a":1}`)) {
		t.Error("a Manifest equals a plain string value")
	}

	if _, diags := v.StringSemanticEquals(ctx, basetypes.NewStringValue(`{"a":1}`)); !diags.HasError() {
		t.Error("StringSemanticEquals accepted a plain string value")
	}
}
