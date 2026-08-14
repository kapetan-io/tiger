package participle_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/kapetan-io/tiger/internal/analyzers/participle"
)

// TestCorpus runs the TS-N14 corpus through the analysistest driver.
//
// Goal: exported identifiers ending in a non-noun participle fire,
// unexported and noun-allowlisted identifiers stay silent, and the
// interface-method known miss stays documented and silent.
func TestCorpus(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), participle.Analyzer, "ts-n14")
}

// TestAllowFlag proves the -allow flag extends the default noun allowlist
// with a project-owned term, leaving the rest of the check enforced.
//
// Goal: a flag-allowlisted participle stays silent, an un-allowlisted one
// still fires, and a built-in noun stays silent alongside the extension.
func TestAllowFlag(t *testing.T) {
	require.NoError(t, participle.Analyzer.Flags.Set("allow", "timing"))
	defer func() {
		require.NoError(t, participle.Analyzer.Flags.Set("allow", ""))
	}()
	analysistest.Run(t, analysistest.TestData(), participle.Analyzer, "allowextended")
}
