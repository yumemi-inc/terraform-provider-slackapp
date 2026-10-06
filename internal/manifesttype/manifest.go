// Package manifesttype provides a custom Terraform string type for Slack app
// manifests that compares values by semantic equality rather than by raw bytes.
//
// The Slack apps.manifest.export API re-serializes the manifest on every read:
// object keys come back in Go struct-field order, permission scopes come back in
// Slack's own order, and whitespace/escaping follow Go's json.Marshal. A manifest
// authored in Terraform (via jsonencode, which sorts object keys alphabetically,
// and via the slackapp_manifest data source, which emits scope sets sorted) can
// therefore never byte-match Slack's export even when the two describe the exact
// same app. With a plain types.String attribute that mismatch surfaces as
// perpetual, un-fixable plan drift.
//
// ManifestType fixes this at the type level: two manifests are equal when they
// decode to the same JSON value after the set-valued scope/event/domain arrays
// (which Slack treats as unordered sets) are sorted. Object key order and
// insignificant whitespace fall out for free because comparison happens on the
// decoded value, not the string. Every field takes part, including the ones
// internal/slack/manifest does not model.
package manifesttype

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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

	equal, err := manifestsEqual(v.ValueString(), newValue.ValueString())
	if err != nil {
		return v.ValueString() == newValue.ValueString(), diags
	}

	return equal, diags
}

// manifestsEqual decodes both manifest strings as generic JSON, sorts the
// arrays Slack treats as sets, and reports whether the results are equal.
//
// It decodes into generic values rather than manifest.App on purpose: that
// struct models only part of the manifest, and decoding into it would drop
// every other field (functions, workflows, outgoing_domains, ...) from both
// sides, so a change to only those fields would compare equal and never
// reach Slack.
//
//declscope:shared // suppress_equivalent.go applies the same comparison at plan time
func manifestsEqual(a, b string) (bool, error) {
	valueA, err := canonicalManifest(a)
	if err != nil {
		return false, err
	}

	valueB, err := canonicalManifest(b)
	if err != nil {
		return false, err
	}

	return reflect.DeepEqual(valueA, valueB), nil
}

// manifestSetPaths are the manifest arrays whose order means nothing to Slack: it
// exports them in its own order. Every other array, such as shortcuts or
// slash_commands, keeps its order in the comparison.
var manifestSetPaths = [][]string{
	{"oauth_config", "redirect_urls"},
	{"oauth_config", "scopes", "bot"},
	{"oauth_config", "scopes", "user"},
	{"settings", "allowed_ip_address_ranges"},
	{"settings", "event_subscriptions", "bot_events"},
	{"settings", "event_subscriptions", "user_events"},
	{"features", "unfurl_domains"},
}

// canonicalManifest decodes a manifest and sorts the arrays at manifestSetPaths.
// Numbers stay json.Number, so that no precision is lost in the comparison.
func canonicalManifest(s string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(s))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}

	if decoder.More() {
		return nil, fmt.Errorf("unexpected data after the manifest")
	}

	root, ok := value.(map[string]any)
	if !ok {
		return value, nil
	}

	for _, p := range manifestSetPaths {
		sortManifestSet(root, p)
	}

	return root, nil
}

// sortManifestSet sorts the array at path in place, when one is there. Elements are
// ordered by their JSON encoding, so that an array of anything sorts.
func sortManifestSet(root map[string]any, path []string) {
	parent := root
	for _, key := range path[:len(path)-1] {
		next, ok := parent[key].(map[string]any)
		if !ok {
			return
		}
		parent = next
	}

	values, ok := parent[path[len(path)-1]].([]any)
	if !ok {
		return
	}

	sort.SliceStable(values, func(i, j int) bool {
		return manifestSetElementKey(values[i]) < manifestSetElementKey(values[j])
	})
}

func manifestSetElementKey(v any) string {
	encoded, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}

	return string(encoded)
}
