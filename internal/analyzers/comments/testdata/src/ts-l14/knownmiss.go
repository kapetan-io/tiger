// known-miss: prose after a //nolint survives, because the reason belongs to the directive.
package l14

//nolint:errcheck // this sentence is prose, and nothing reports it
type Miss struct{}
