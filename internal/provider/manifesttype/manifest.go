// Package manifesttype is the type of slackapp_application's manifest
// attribute: a JSON string that Terraform compares by meaning, not by bytes.
//
// Slack's apps.manifest.export does not give back the bytes it was sent: its
// keys come in its own order, and its formatting is its own. With a plain
// string attribute, that difference shows as drift that applying cannot
// clear. Manifest's StringSemanticEquals compares with manifest.Equal
// instead, so key order and whitespace do not matter, while every field,
// including the ones the slackapp_manifest data source does not model,
// takes part.
//
// The same comparison is applied at plan time by
// planmodifiers.SuppressEquivalentManifest, and the defaults Slack fills in
// are dropped on refresh by manifest.PruneToPrior.
package manifesttype

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/manifest"
)

// ManifestType is the attr.Type for a Slack app manifest string.
type ManifestType struct {
	basetypes.StringType
}

var _ basetypes.StringTypable = ManifestType{}

func (t ManifestType) Equal(o attr.Type) bool {
	other, ok := o.(ManifestType)
	if !ok {
		return false
	}

	return t.StringType.Equal(other.StringType)
}

func (t ManifestType) String() string {
	return "manifesttype.ManifestType"
}

func (t ManifestType) ValueFromString(
	_ context.Context,
	in basetypes.StringValue,
) (basetypes.StringValuable, diag.Diagnostics) {
	return Manifest{StringValue: in}, nil
}

func (t ManifestType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}

	stringValue, ok := attrValue.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type %T", attrValue)
	}

	stringValuable, diags := t.ValueFromString(ctx, stringValue)
	if diags.HasError() {
		return nil, fmt.Errorf("unexpected error converting StringValue to StringValuable: %v", diags)
	}

	return stringValuable, nil
}

func (t ManifestType) ValueType(_ context.Context) attr.Value {
	return Manifest{}
}

// Manifest is the attr.Value for a Slack app manifest string.
type Manifest struct {
	basetypes.StringValue
}

var _ basetypes.StringValuableWithSemanticEquals = Manifest{}

// NewManifestValue returns a known Manifest value wrapping the given JSON string.
func NewManifestValue(value string) Manifest {
	return Manifest{StringValue: basetypes.NewStringValue(value)}
}

func (v Manifest) Type(_ context.Context) attr.Type {
	return ManifestType{}
}

func (v Manifest) Equal(o attr.Value) bool {
	other, ok := o.(Manifest)
	if !ok {
		return false
	}

	return v.StringValue.Equal(other.StringValue)
}

// StringSemanticEquals reports whether the prior value and the refreshed/planned
// value describe the same Slack app manifest, ignoring object key order,
// insignificant whitespace, and the ordering of set-valued scope/event/domain
// arrays. Unparseable JSON on either side falls back to exact string comparison
// so genuine (or malformed) changes are never silently suppressed.
func (v Manifest) StringSemanticEquals(
	_ context.Context,
	newValuable basetypes.StringValuable,
) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	newValue, ok := newValuable.(Manifest)
	if !ok {
		diags.AddError(
			"Semantic Equality Check Error",
			fmt.Sprintf("expected value type %T but got %T", v, newValuable),
		)

		return false, diags
	}

	// Null/unknown can't be parsed; defer to base string equality.
	if v.IsNull() || v.IsUnknown() || newValue.IsNull() || newValue.IsUnknown() {
		return v.StringValue.Equal(newValue.StringValue), diags
	}

	equal, err := manifest.Equal(v.ValueString(), newValue.ValueString())
	if err != nil {
		return v.ValueString() == newValue.ValueString(), diags
	}

	return equal, diags
}
