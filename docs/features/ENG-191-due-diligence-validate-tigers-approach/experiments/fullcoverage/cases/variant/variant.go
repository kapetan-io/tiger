// Package variant names one rewrite strategy of a real map-ordering site.
package variant

// Variant is one strategy's rendering of a case's output.
type Variant struct {
	Name string
	Run  func() string
}
