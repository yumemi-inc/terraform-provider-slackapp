package manifestblocks

//declscope:shared // list.go and option.go constrain their block types with it
type model[T any] interface {
	Read() T
}
