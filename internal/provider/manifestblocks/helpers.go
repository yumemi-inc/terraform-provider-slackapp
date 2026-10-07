// Conversions from the plugin framework's values into the plain Go values
// internal/manifest takes, which every block file's Read() uses.
//
//declscope:core // the package's shared helpers, used by most block files
//declscope:shared // the block files and manifest_data_source.go read their values with these

package manifestblocks

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

type model[T any] interface {
	Read() T
}

func int64PtrAsIntPtr(ptr *int64) *int {
	if ptr == nil {
		return nil
	}

	value := int(*ptr)

	return &value
}

func mapListModel[T any, M model[T]](model []M) []T {
	if model == nil {
		return nil
	}

	read := make([]T, 0, len(model))
	for _, m := range model {
		read = append(read, m.Read())
	}

	return read
}

//declscope:private // only mapOptionModel below uses it
func mapOption[T, U any](value *T, fn func(T) U) *U {
	if value == nil {
		return nil
	}

	v := fn(*value)

	return &v
}

func MapOptionModel[T any, M model[T]](model *M) *T {
	return mapOption[M, T](
		model, func(m M) T {
			return m.Read()
		},
	)
}

func mustStringSetAsArray(setValue *types.Set) []string {
	elements := setValue.Elements()
	strings := make([]string, 0, len(elements))
	for _, element := range elements {
		value, ok := element.(types.String)
		if !ok {
			panic(fmt.Sprintf("Expected types.String, got %T", element))
		}

		strings = append(strings, value.ValueString())
	}

	return strings
}
