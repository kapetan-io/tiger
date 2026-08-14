package testdoc_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/testdoc"
)

// TestCorpus runs the TS-T06 corpus through the analysistest driver.
//
// Goal: Test functions with no doc comment or no Goal line fire, compliant
// doc comments stay silent, and the benchmark/fuzz/helper known misses
// hold.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), testdoc.Analyzer, "ts-t06")
}
