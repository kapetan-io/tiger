package effects_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/effects"
	"github.com/kapetan-io/tiger/internal/directive"
)

// quietRun swallows analysistest's expectation mismatches; TestFactsRoundTrip
// only consumes the collected diagnostics — TestCorpus owns expectation
// matching, mirroring the meta-test pattern in internal/rules/meta_test.go.
type quietRun struct{}

func (quietRun) Errorf(format string, args ...any) {}

// TestCorpus runs the TS-F01 and TS-F02 corpora through the analysistest
// driver. The ts-f02helper package is a plain dependency package, not a
// pattern: analysistest analyzes it automatically to produce the
// cross-package fact ts-f02's pinned callers consume.
//
// Goal: an own-instruction effect a pin does not declare fails TS-F01, a
// pin declaring an effect the computed set lacks fails TS-F01, a pin on an
// unexported function fails TS-F01, exact pins over own instructions and
// the stdlib table stay silent, a widening introduced beneath a pin —
// through a same-package helper or across a package boundary — fails
// TS-F02 at the pin naming the introducing call, modular pin-to-pin and
// pin-over-unexported-helper calls stay silent, unpinned exported
// functions print their computed effects as TS-F01-facts, and the
// documented function-value known-miss stays silent.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), effects.Analyzer, "ts-f01", "ts-f02")
}

// TestFactsRoundTrip covers invariant 1 at the analyzer level: every
// TS-F01-facts message's pin-syntax suffix parses back through the grammar
// package to itself.
//
// Goal: for every reported fact in the corpus, the substring starting at
// "//tiger:" parses without error and directive.Format reprints it
// byte-identical, so a fact printed today is a pin ENG-151 can freeze
// tomorrow by pasting it.
func TestFactsRoundTrip(t *testing.T) {
	results := analysistest.Run(
		quietRun{}, analysistest.TestData(), effects.Analyzer, "ts-f01", "ts-f02",
	)

	found := 0
	for _, result := range results {
		for _, diag := range result.Diagnostics {
			if diag.Category != "TS-F01-facts" {
				continue
			}
			found++
			at := strings.Index(diag.Message, "//tiger:")
			require.GreaterOrEqual(t, at, 0)
			text := diag.Message[at:]
			parsed, err := directive.Parse(text)
			require.NoError(t, err)
			assert.Equal(t, text, directive.Format(parsed))
		}
	}
	assert.Positive(t, found)
}
