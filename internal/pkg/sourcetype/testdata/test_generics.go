package test

type G[T any] struct{}

func (G[T]) M(_ T) {
}
