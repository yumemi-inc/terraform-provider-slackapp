package typeconv

import (
	"github.com/yumemi-inc/terraform-provider-slackapp/internal/common"
)

func MapOption[T, U any](value *T, fn func(T) U) *U {
	if value == nil {
		return nil
	}

	v := fn(*value)

	return &v
}

func MapOptionModel[T any, M common.Model[T]](model *M) *T {
	return MapOption[M, T](
		model, func(m M) T {
			return m.Read()
		},
	)
}
