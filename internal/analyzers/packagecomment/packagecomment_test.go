package packagecomment_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/packagecomment"
)

func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), packagecomment.Analyzer,
		"ts-l15/...", "ts-l16/...")
}
