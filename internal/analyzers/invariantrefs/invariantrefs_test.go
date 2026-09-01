package invariantrefs_test

import (
	"testing"

	"github.com/kapetan-io/tiger/internal/analyzers/invariantrefs"
	"github.com/kapetan-io/tiger/internal/driver/drivertest"
	"github.com/kapetan-io/tiger/internal/finish"
)

// TestCorpus runs the TS-A07 module corpus through the tiger driver.
//
// Goal: an invariant asserted only from a _test.go file, and one never
// referenced anywhere, both fire TS-A07 at their declaration; an
// invariant asserted once in production — directly, from a closure
// folded into its enclosing function, or from a method body — stays
// silent, and a named string type no assert call ever names is not an
// invariant and is ignored.
func TestCorpus(t *testing.T) {
	drivertest.Run(t, "testdata/module", finish.Finisher{
		Analyzer: invariantrefs.Analyzer,
		Run:      invariantrefs.Finish,
	})
}
