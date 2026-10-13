// Package maps is a stub of golang.org/x/exp/maps, which returns a slice of
// keys in map order.
package maps

func Keys[M ~map[K]V, K comparable, V any](m M) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
