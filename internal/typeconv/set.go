package typeconv

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func MustStringSetAsArray(setValue *types.Set) []string {
	elements := setValue.Elements()
	strings := make([]string, 0, len(elements))
	for _, element := range elements {
		strings = append(strings, element.(types.String).ValueString())
	}

	return strings
}
