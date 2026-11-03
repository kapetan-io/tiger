package comments_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/comments"
)

func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), comments.Analyzer, "ts-l14")
}

func TestCgoPreamble(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), comments.Analyzer, "ts-l14/cgo")
}
