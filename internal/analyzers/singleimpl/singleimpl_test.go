package singleimpl_test

import (
	"testing"

	"github.com/kapetan-io/tiger/internal/analyzers/singleimpl"
	"github.com/kapetan-io/tiger/internal/driver/drivertest"
	"github.com/kapetan-io/tiger/internal/finish"
)

// TestCorpus runs the TS-X01 module corpus through the finish-step
// harness, since analysistest cannot exercise a finish function.
//
// Goal: a lone non-test implementation fires TS-X01 at the interface,
// naming the implementation (bare in the same package, package-qualified
// across packages, including across the pkg [pkg.test] variant seam); a
// second non-test implementation clears it; a second implementation living
// only in a _test.go file does not (state invariant 3, the file suffix is
// exact); and zero implementations, an empty interface, and an interface
// declared in a _test.go file all stay silent.
func TestCorpus(t *testing.T) {
	drivertest.Run(t, "testdata/module", finish.Finisher{
		Analyzer: singleimpl.Analyzer,
		Run:      singleimpl.Finish,
	})
}
