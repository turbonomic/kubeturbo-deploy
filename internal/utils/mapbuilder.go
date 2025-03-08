package utils

type MapBuilder[K comparable, V any] interface {
	Build() map[K]V
	PutAll(map[K]V) MapBuilder[K, V]
	Put(K, V) MapBuilder[K, V]
}

type mapBuilder[K comparable, V any] struct {
	m map[K]V
}

func NewMapBuilder[K comparable, V any]() MapBuilder[K, V] {
	return &mapBuilder[K, V]{
		m: make(map[K]V),
	}
}

func (mb *mapBuilder[K, V]) Put(key K, value V) MapBuilder[K, V] {
	mb.m[key] = value
	return mb
}

func (mb *mapBuilder[K, V]) PutAll(m map[K]V) MapBuilder[K, V] {
	for key, value := range m {
		mb.Put(key, value)
	}
	return mb
}

func (mb *mapBuilder[K, V]) Build() map[K]V {
	return mb.m
}
