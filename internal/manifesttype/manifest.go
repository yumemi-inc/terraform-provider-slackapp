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
// decode to the same manifest.App after the set-valued scope/event/domain arrays
// (which Slack treats as unordered sets) are sorted. Object key order and
// insignificant whitespace fall out for free because comparison happens on the
// decoded structure, not the string.
package manifesttype

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/ymm-oss/terraform-provider-slackapp/internal/slack/manifest"
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

// manifestsEqual decodes both manifest strings, canonicalizes their set-valued
// fields, and reports structural equality.
func manifestsEqual(a, b string) (bool, error) {
	appA, err := parseAndCanonicalize(a)
	if err != nil {
		return false, err
	}

	appB, err := parseAndCanonicalize(b)
	if err != nil {
		return false, err
	}

	return reflect.DeepEqual(appA, appB), nil
}

func parseAndCanonicalize(s string) (*manifest.App, error) {
	var app manifest.App
	if err := json.Unmarshal([]byte(s), &app); err != nil {
		return nil, err
	}

	canonicalize(&app)

	return &app, nil
}

// canonicalize sorts every manifest field that the slackapp_manifest data source
// models as a set (types.Set), so equal sets compare equal regardless of the
// order Slack or Terraform happens to serialize them in. Ordered lists
// (shortcuts, slash_commands, workflow_steps) are intentionally left untouched.
func canonicalize(app *manifest.App) {
	if app.OauthConfig != nil {
		sort.Strings(app.OauthConfig.RedirectURLs)
		if app.OauthConfig.Scopes != nil {
			sort.Strings(app.OauthConfig.Scopes.Bot)
			sort.Strings(app.OauthConfig.Scopes.User)
		}
	}

	if app.Settings != nil {
		sort.Strings(app.Settings.AllowedIPAddressRanges)
		if app.Settings.EventSubscriptions != nil {
			sort.Strings(app.Settings.EventSubscriptions.BotEvents)
			sort.Strings(app.Settings.EventSubscriptions.UserEvents)
		}
	}

	if app.Features != nil {
		sort.Strings(app.Features.UnfurlDomains)
	}
}
