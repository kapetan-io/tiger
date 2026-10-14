package maporder_test

import (
	"go/ast"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"

	"github.com/kapetan-io/tiger/internal/analyzers/maporder"
	"github.com/kapetan-io/tiger/internal/analyzers/maporder/testdata/soundness/boundary"
	actions "github.com/kapetan-io/tiger/internal/analyzers/maporder/testdata/soundness/checker"
	"github.com/kapetan-io/tiger/internal/analyzers/maporder/testdata/soundness/commentmap"
	"github.com/kapetan-io/tiger/internal/analyzers/maporder/testdata/soundness/compose"
	"github.com/kapetan-io/tiger/internal/analyzers/maporder/testdata/soundness/gaps"
	"github.com/kapetan-io/tiger/internal/analyzers/maporder/testdata/soundness/gitwants"
	"github.com/kapetan-io/tiger/internal/analyzers/maporder/testdata/soundness/images"
	"github.com/kapetan-io/tiger/internal/analyzers/maporder/testdata/soundness/linecount"
	"github.com/kapetan-io/tiger/internal/analyzers/maporder/testdata/soundness/mount"
	"github.com/kapetan-io/tiger/internal/analyzers/maporder/testdata/soundness/oid"
	"github.com/kapetan-io/tiger/internal/analyzers/maporder/testdata/soundness/tsidp"
	"github.com/kapetan-io/tiger/internal/analyzers/maporder/testdata/soundness/variant"
)

const runs = 200

// A row is the per-function table. The finding field is the analyzer's
// verdict, and varies says whether 200 calls print more than one output.
// The knownMiss field marks a function the analyzer passes although its
// output varies, one of the documented misses in ADR-0021.
type row struct {
	name      string
	finding   bool
	varies    bool
	knownMiss bool
}

var expected = []row{
	{name: "oid.Original", finding: true},
	{name: "oid.Bare", finding: true, varies: true},
	{name: "oid.S2", finding: true},
	{name: "oid.S2Keep"},
	{name: "oid.S2Partial", finding: true, varies: true},
	{name: "oid.S3", finding: true},
	{name: "oid.S4", finding: true},
	{name: "oid.S6Hex", finding: true},

	{name: "linecount.Original", finding: true, varies: true},
	{name: "linecount.S1"},
	{name: "linecount.S1Unstable"},
	{name: "linecount.S3"},
	{name: "linecount.S4"},
	{name: "linecount.S6"},

	{name: "mount.Original", finding: true, varies: true},
	{name: "mount.S1"},
	{name: "mount.S3"},
	{name: "mount.S4"},
	{name: "mount.S6"},

	{name: "images.Original", finding: true, varies: true},
	{name: "images.S1"},
	{name: "images.S1Unstable"},
	{name: "images.S4", finding: true},

	{name: "commentmap.Original", finding: true, varies: true},
	{name: "commentmap.S2", finding: true, varies: true},
	{name: "commentmap.S2Type", finding: true},
	{name: "commentmap.S4", finding: true},
	{name: "commentmap.S5"},

	{name: "tsidp.Original", finding: true},
	{name: "tsidp.S1"},
	{name: "tsidp.S1Stable"},
	{name: "tsidp.S4", finding: true},
	{name: "tsidp.S6", finding: true},

	{name: "compose.Original", finding: true},
	{name: "compose.S1"},
	{name: "compose.S4"},
	{name: "compose.S6"},

	{name: "checker.Original", finding: true, varies: true},
	{name: "checker.S2", finding: true},
	{name: "checker.S4", finding: true},
	{name: "checker.S5"},

	{name: "gitwants.Original", finding: true, varies: true},
	{name: "gitwants.Sorted", finding: true},
	{name: "gitwants.S2"},
	{name: "gitwants.S3"},
	{name: "gitwants.S5"},
	{name: "gitwants.S6", finding: true},

	{name: "gaps.FirstThree", finding: true, varies: true},
	{name: "gaps.RankByValue", finding: true, varies: true},
	{name: "gaps.SortedValues", finding: true, varies: true},
	{name: "gaps.SortedIDs"},
	{name: "gaps.UnsortedIDs", finding: true, varies: true},

	{name: "boundary.PartialArray", finding: true, varies: true},
	{name: "boundary.WholeArray"},
	{name: "boundary.OneField", finding: true, varies: true},
	{name: "boundary.EveryField"},
	{name: "boundary.StringMethod", finding: true, varies: true},
	{name: "boundary.PointerLeaf", finding: true, varies: true},
	{name: "boundary.InterfaceLeaf", finding: true, varies: true},
	{name: "boundary.FloatLeaf", finding: true, varies: true},
	{name: "boundary.BoolLeaf", finding: true, varies: true},
	{name: "boundary.TimeField", finding: true, varies: true},
	{name: "boundary.AsymmetricTieBreak", finding: true, varies: true},
	{name: "boundary.CallingTieBreak", finding: true, varies: true},
	{name: "boundary.TotalItems"},
	{name: "boundary.VisitRank", finding: true, varies: true},
	{name: "boundary.GroupByValue", finding: true, varies: true},
	{name: "boundary.CollectEscape", finding: true, varies: true},
	{name: "boundary.PromotedFields"},
	{name: "boundary.StoredComparator", finding: true},
	{name: "boundary.IndexComparator"},
	{name: "boundary.Invert", varies: true, knownMiss: true},
}

// variants indexes every case function's runner by package.function.
func variants() map[string]func() string {
	all := map[string]func() string{}
	for _, group := range []struct {
		pkg  string
		list []variant.Variant
	}{
		{"oid", oid.Variants},
		{"linecount", linecount.Variants},
		{"mount", mount.Variants},
		{"images", images.Variants},
		{"commentmap", commentmap.Variants},
		{"tsidp", tsidp.Variants},
		{"compose", compose.Variants},
		{"checker", actions.Variants},
		{"gitwants", gitwants.Variants},
		{"gaps", gaps.Variants},
		{"boundary", boundary.Variants},
	} {
		for _, v := range group.list {
			all[group.pkg+"."+v.Name] = v.Run
		}
	}
	return all
}

// verdicts runs the analyzer over the case packages and reports, for
// every top-level function, whether a TS-T02 finding falls inside it.
func verdicts(t *testing.T) map[string]bool {
	t.Helper()
	cfg := &packages.Config{Mode: packages.LoadAllSyntax}
	pkgs, err := packages.Load(cfg, "./testdata/soundness/...")
	require.NoError(t, err)
	require.NotEmpty(t, pkgs)
	for _, pkg := range pkgs {
		require.Empty(t, pkg.Errors)
	}
	graph, err := checker.Analyze([]*analysis.Analyzer{maporder.Analyzer}, pkgs, nil)
	require.NoError(t, err)
	found := map[string]bool{}
	for _, act := range graph.Roots {
		require.NoError(t, act.Err)
		for _, file := range act.Package.Syntax {
			for _, decl := range file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok || fd.Recv != nil {
					continue
				}
				name := act.Package.Name + "." + fd.Name.Name
				for _, diag := range act.Diagnostics {
					if diag.Category == "TS-T02" && diag.Pos >= fd.Pos() && diag.Pos < fd.End() {
						found[name] = true
					}
				}
				if _, ok := found[name]; !ok {
					found[name] = false
				}
			}
		}
	}
	return found
}

// distinct calls run the given number of times and counts the different
// outputs it prints.
func distinct(run func() string) int {
	seen := map[string]bool{}
	for range runs {
		seen[run()] = true
	}
	return len(seen)
}

// TestVerdicts proves the analyzer fires or passes on each case function
// as the table says, and that no case function fires without a table row.
func TestVerdicts(t *testing.T) {
	found := verdicts(t)
	for _, test := range expected {
		t.Run(test.name, func(t *testing.T) {
			finding, ok := found[test.name]
			require.True(t, ok)
			assert.Equal(t, test.finding, finding)
		})
	}
	listed := map[string]bool{}
	for _, test := range expected {
		listed[test.name] = true
	}
	for _, name := range slices.Sorted(maps.Keys(found)) {
		t.Run("unlisted/"+name, func(t *testing.T) {
			assert.True(t, listed[name] || !found[name])
		})
	}
}

// TestDeterminism proves, per the table, which case functions print one
// output over 200 calls and which print more: every function the table says
// the analyzer passes prints one (except the known miss), and every
// original with ties prints more than one.
func TestDeterminism(t *testing.T) {
	all := variants()
	assert.Len(t, all, len(expected))
	for _, test := range expected {
		t.Run(test.name, func(t *testing.T) {
			run, ok := all[test.name]
			require.True(t, ok)
			if test.varies {
				assert.Greater(t, distinct(run), 1)
				return
			}
			assert.Equal(t, 1, distinct(run))
		})
	}
}

// TestSoundness is the core claim, joined live from the analyzer and from
// the outputs rather than from the table: no case function the analyzer
// passes prints more than one output over 200 calls. The known miss is the
// one exception, and it must keep both passing and varying, so closing the
// gap shows up here as a changed verdict.
func TestSoundness(t *testing.T) {
	found := verdicts(t)
	all := variants()
	known := map[string]bool{}
	for _, test := range expected {
		if test.knownMiss {
			known[test.name] = true
		}
	}
	accepted := 0
	for _, name := range slices.Sorted(maps.Keys(all)) {
		finding, ok := found[name]
		require.True(t, ok)
		if finding {
			continue
		}
		if !known[name] {
			accepted++
		}
		t.Run(name, func(t *testing.T) {
			if known[name] {
				assert.Greater(t, distinct(all[name]), 1)
				return
			}
			assert.Equal(t, 1, distinct(all[name]))
		})
	}
	assert.GreaterOrEqual(t, accepted, 25)
}
