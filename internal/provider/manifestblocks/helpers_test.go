package manifestblocks

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestInt64PtrAsIntPtr(t *testing.T) {
	t.Parallel()

	if got := int64PtrAsIntPtr(nil); got != nil {
		t.Errorf("int64PtrAsIntPtr(nil) = %v, want nil", *got)
	}

	v := int64(42)
	if got := int64PtrAsIntPtr(&v); got == nil || *got != 42 {
		t.Errorf("int64PtrAsIntPtr(&42) = %v", got)
	}
}

func TestMustStringSetAsArray(t *testing.T) {
	t.Parallel()

	set := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("a"), types.StringValue("b")})
	if got := mustStringSetAsArray(&set); len(got) != 2 {
		t.Errorf("mustStringSetAsArray = %v", got)
	}

	// A set left out of the config is empty, which internal/manifest's
	// omitempty then leaves out of the JSON.
	null := types.SetNull(types.StringType)
	if got := mustStringSetAsArray(&null); got == nil || len(got) != 0 {
		t.Errorf("mustStringSetAsArray(null) = %#v, want an empty slice", got)
	}

	defer func() {
		if recover() == nil {
			t.Error("mustStringSetAsArray accepted a set of numbers")
		}
	}()

	numbers := types.SetValueMust(types.Int64Type, []attr.Value{types.Int64Value(1)})
	mustStringSetAsArray(&numbers)
}

// A metadata block without versions reads as no versions, not as zeros.
func TestMetadataReadWithoutVersions(t *testing.T) {
	t.Parallel()

	got := Metadata{MajorVersion: types.Int64Null(), MinorVersion: types.Int64Null()}.Read()
	if got.MajorVersion != nil || got.MinorVersion != nil {
		t.Errorf("Read() = %+v, want no versions", got)
	}
}
