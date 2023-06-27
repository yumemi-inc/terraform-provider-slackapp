package typeconv

func Int64PtrAsIntPtr(ptr *int64) *int {
	if ptr == nil {
		return nil
	}

	value := int(*ptr)

	return &value
}
