package typeconv

import (
	"github.com/yumemi-inc/terraform-provider-slackapp/internal/common"
)

func MapListModel[T any, M common.Model[T]](model []M) []T {
	if model == nil {
		return nil
	}

	read := make([]T, 0, len(model))
	for _, m := range model {
		read = append(read, m.Read())
	}

	return read
}
