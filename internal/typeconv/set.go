package typeconv

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func MustStringSetAsArray(setValue *types.Set) []string {
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
