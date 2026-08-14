// known-miss: skipcheck resolves a Skip call's receiver by its immediate
// static type. A Skip reached through a struct that embeds *testing.T has
// a receiver whose static type is the wrapping struct, not testing.T, so
// the call below is silently exempt even though it really does skip a
// test.
package fixture

import "testing"

type wrapper struct {
	*testing.T
}

func TestEmbeddedSkip(t *testing.T) {
	w := wrapper{T: t}
	w.Skip()
}
