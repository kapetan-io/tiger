// A package comment above the package clause survives.
package l14

//go:generate x
type Generate struct{}

//go:embed f
var embedded string

//tiger:batched the provider offers no bulk endpoint
type Batched struct{}

//nolint:errcheck // reason
type Reasoned struct{}

//nolint
type Bare struct{}

//export F
type Exported struct{}

//extern f
type External struct{}
