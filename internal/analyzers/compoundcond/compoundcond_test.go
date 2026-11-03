package compoundcond_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/compoundcond"
)

// TestCorpus runs the TS-S06, TS-S07, and TS-S08 corpus through the
// analysistest driver.
//
// Goal: mixed &&/|| conditions and unsplit compound assertions fire, closed
// switches missing a proper assert.Unreachable default fire, the compliant
// rewrites of all three stay silent, and the documented known misses hold.
// A switch over a //tiger:openenum type still needs a default arm, because
// the value came off the wire, but any body is compliant there since the
// catch-all is legitimate. The directive is recognized only in the package
// that declares the type, so a marker on a type in another package is
// invisible. Type switches, tagless switches, and switches over plain basic
// types or open sets stay silent. A condition mixing && and || fires TS-S06
// even through parens and !, and a nil condition (a for with no condition)
// is silent.
func TestCorpus(t *testing.T) {
	analysistest.Run(
		t,
		analysistest.TestData(),
		compoundcond.Analyzer,
		"ts-s06",
		"ts-s07",
		"ts-s08",
		"lookalikeuse",
	)
}
