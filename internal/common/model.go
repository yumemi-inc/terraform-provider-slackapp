package common

type Model[T any] interface {
	Read() T
}
