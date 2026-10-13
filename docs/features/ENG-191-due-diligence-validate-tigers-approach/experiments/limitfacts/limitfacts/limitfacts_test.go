package limitfacts_test

import (
	"testing"

	"limitfacts/limitfacts"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestLimitFacts(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), limitfacts.Analyzer, "lib", "types", "store", "app")
}
