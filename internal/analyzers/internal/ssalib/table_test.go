package ssalib_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/analyzers/internal/ssalib"
	"github.com/kapetan-io/tiger/internal/directive"
)

// TestTableCoversTheBlueprintExamples covers the stdlib effects table's
// contract.
//
// Goal: the curated entries the blueprint names resolve to their effects —
// file writes to io(disk), dialing to io(net), the clock to time, process
// spawning by package prefix to io(exec), the environment to io(env), and
// math/rand to rand — and each lookup formats to valid pin syntax.
func TestTableCoversTheBlueprintExamples(t *testing.T) {
	for _, test := range []struct {
		name string
		call ssalib.CallName
		want string
	}{
		{
			name: "FileWrite",
			call: ssalib.CallName{Full: "(*os.File).Write", Package: "os"},
			want: "io(disk)",
		},
		{
			name: "Dial",
			call: ssalib.CallName{Full: "net.Dial", Package: "net"},
			want: "io(net)",
		},
		{
			name: "Clock",
			call: ssalib.CallName{Full: "time.Now", Package: "time"},
			want: "time",
		},
		{
			name: "SleepBlocksAndReadsTheClock",
			call: ssalib.CallName{Full: "time.Sleep", Package: "time"},
			want: "block, time",
		},
		{
			name: "ExecByPackagePrefix",
			call: ssalib.CallName{Full: "os/exec.Command", Package: "os/exec"},
			want: "io(exec)",
		},
		{
			name: "ExecMethodByPackagePrefix",
			call: ssalib.CallName{Full: "(*os/exec.Cmd).Run", Package: "os/exec"},
			want: "io(exec)",
		},
		{
			name: "Environment",
			call: ssalib.CallName{Full: "os.Getenv", Package: "os"},
			want: "io(env)",
		},
		{
			name: "RandByPackagePrefix",
			call: ssalib.CallName{Full: "math/rand.Intn", Package: "math/rand"},
			want: "rand",
		},
		{
			name: "MutexLockBlocks",
			call: ssalib.CallName{Full: "(*sync.Mutex).Lock", Package: "sync"},
			want: "block",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			found, ok := ssalib.Lookup(test.call)
			require.True(t, ok)
			assert.Equal(t, test.want, directive.FormatEffects(found))
		})
	}
}

// TestTableGapsContributeNothing covers the known-miss boundary.
//
// Goal: an unlisted stdlib call resolves to nothing — the table's coverage
// gaps are the analyzer's known misses, never a guess.
func TestTableGapsContributeNothing(t *testing.T) {
	found, ok := ssalib.Lookup(ssalib.CallName{Full: "strings.TrimSpace", Package: "strings"})
	assert.False(t, ok)
	assert.True(t, found.Empty())
}

// TestTableEntriesAreValidPinSyntax covers invariant 1 for the table.
//
// Goal: every committed table entry formats to argument text the grammar
// parses back to the identical set, so a fact sourced from the table can
// always be frozen into a pin.
func TestTableEntriesAreValidPinSyntax(t *testing.T) {
	for _, name := range ssalib.TableNames() {
		found, ok := ssalib.Lookup(name)
		require.True(t, ok)
		require.False(t, found.Empty())
		parsed, err := directive.ParseEffects(directive.FormatEffects(found))
		require.NoError(t, err)
		assert.True(t, parsed.Equal(found))
	}
	assert.NotEmpty(t, ssalib.TableNames())
	assert.IsIncreasing(t, tableKeys(t))
}

// tableKeys renders the table's names for the determinism assertion: exact
// entries by function string, then whole-package entries prefixed past the
// printable range so the two sorted runs concatenate in order.
func tableKeys(t *testing.T) []string {
	t.Helper()
	keys := []string{}
	for _, name := range ssalib.TableNames() {
		if name.Full != "" {
			keys = append(keys, name.Full)
			continue
		}
		keys = append(keys, "~"+name.Package)
	}
	return keys
}
