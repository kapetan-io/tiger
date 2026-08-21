package frames_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/frames"
	"github.com/kapetan-io/tiger/internal/directive"
)

// quietRun swallows analysistest's expectation mismatches; TestFactsRoundTrip
// only consumes the collected diagnostics, and TestCorpus owns expectation
// matching.
type quietRun struct{}

func (quietRun) Errorf(format string, args ...any) {}

// TestCorpus runs the TS-F07 corpus through the analysistest driver.
//
// Goal: a write outside a pinned frame, a pinned location the body never
// writes, and a frame pin on an unexported function all fire blocking
// findings at the pin; every compliant pin and every documented known-miss
// stays silent.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), frames.Analyzer, "ts-f07")
}

// TestFactsRoundTrip proves invariant 1: every TS-F07-facts message carries
// a //tiger:frame directive that parses back to the identical text it
// printed.
//
// Goal: computed-frame facts for unpinned exported functions are printed
// byte-identical to the pin syntax ENG-151 will freeze them into.
func TestFactsRoundTrip(t *testing.T) {
	results := analysistest.Run(quietRun{}, analysistest.TestData(), frames.Analyzer, "ts-f07")
	found := 0
	for _, result := range results {
		for _, diag := range result.Diagnostics {
			if diag.Category != "TS-F07-facts" {
				continue
			}
			found++
			cut := strings.Index(diag.Message, directive.Prefix)
			require.GreaterOrEqual(t, cut, 0)
			text := diag.Message[cut:]
			parsed, err := directive.Parse(text)
			require.NoError(t, err)
			require.Equal(t, text, directive.Format(parsed))
		}
	}
	require.Positive(t, found)
}
