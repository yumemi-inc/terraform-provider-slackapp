package manifestblocks

//declscope:shared // metadata.go converts the version numbers with it
func int64PtrAsIntPtr(ptr *int64) *int {
	if ptr == nil {
		return nil
	}

	value := int(*ptr)

	return &value
}
