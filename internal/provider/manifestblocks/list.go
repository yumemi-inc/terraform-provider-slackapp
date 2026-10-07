package manifestblocks

//declscope:shared // features.go reads its list blocks with it
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
