package invariantnegative_test

import (
	"testing"

	"github.com/kapetan-io/tiger/internal/analyzers/invariantnegative"
	"github.com/kapetan-io/tiger/internal/driver/drivertest"
	"github.com/kapetan-io/tiger/internal/finish"
)

// TestCorpus runs the TS-A09 module corpus through the tiger driver.
//
// Goal: an invariant with no assert.Violates call anywhere, and one whose
// only assert.Violates call sits in a production function rather than a
// _test.go file, both fire TS-A09 at their declaration; a violation from
// an external test package (foo_test) and one from an in-package
// _test.go file both stay silent.
func TestCorpus(t *testing.T) {
	drivertest.Run(t, "testdata/module", finish.Finisher{
		Analyzer: invariantnegative.Analyzer,
		Run:      invariantnegative.Finish,
	})
}
