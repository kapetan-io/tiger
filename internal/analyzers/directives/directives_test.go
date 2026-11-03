package directives_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/directives"
)

// TestCorpus runs the TS-L09 corpus through the analysistest driver.
//
// Goal: unknown verbs, reasonless escapes, and trackerless skips fire as
// blocking findings; a well-formed escape fires the standing advisory; pins,
// intent declarations, and prose stay silent. A //nolint ban finding sits at
// the file's package line so the comment cannot cover itself; that is pinned
// in internal/check/nolint_test.go, not by this corpus. A rule code written
// as the verb is the unshipped deviation form and is named generically in
// the message, which keeps a second rule code out of the body. Parse can fail
// only as unknown verb, missing reason or malformed args, and each has its
// own fixture; the analyzer's final `err != nil` arm silently drops any other
// failure, so a new Parse failure mode gets no finding until it gets a case
// here and an arm in the analyzer.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), directives.Analyzer, "ts-l09")
}
