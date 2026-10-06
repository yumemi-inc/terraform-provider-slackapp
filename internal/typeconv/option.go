package typeconv

import (
	"github.com/ymm-oss/terraform-provider-slackapp/internal/common"
)

func mapOption[T, U any](value *T, fn func(T) U) *U {
	if value == nil {
		return nil
	}

	v := fn(*value)

	return &v
}

func MapOptionModel[T any, M common.Model[T]](model *M) *T {
	return mapOption[M, T](
		model, func(m M) T {
			return m.Read()
		},
	)
}
