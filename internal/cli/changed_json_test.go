package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/cli"
)

// report is the part of the tiger.changed/v1 document the tests read.
type report struct {
	Schema string `json:"schema"`
	Base   struct {
		Ref       string `json:"ref"`
		Commit    string `json:"commit"`
		MergeBase bool   `json:"merge_base"`
	} `json:"base"`
	Head  map[string]any `json:"head"`
	Scope struct {
		NotCompared []string `json:"not_compared"`
		Generated   []string `json:"skipped_generated_files"`
	} `json:"scope"`
	Counts struct {
		CallsAdded   int `json:"calls_added"`
		CallsRemoved int `json:"calls_removed"`
	} `json:"counts"`
	Functions []struct {
		Function string  `json:"function"`
		Owner    string  `json:"owner"`
		Method   *string `json:"method"`
		Calls    []struct {
			Change            string  `json:"change"`
			Callee            string  `json:"callee"`
			Target            string  `json:"target"`
			Method            string  `json:"method"`
			TargetPackage     string  `json:"target_package"`
			TargetInModule    bool    `json:"target_in_module"`
			SamePackage       bool    `json:"same_package"`
			PackageDependency *string `json:"package_dependency"`
			Sites             []struct {
				File string `json:"file"`
				Line int    `json:"line"`
				Rev  string `json:"rev"`
			} `json:"sites"`
		} `json:"calls"`
	} `json:"functions"`
	NewPackages []struct {
		Package   string `json:"package"`
		Owners    int    `json:"owners"`
		Calls     int    `json:"calls"`
		Functions []struct {
			Function string `json:"function"`
			Calls    []struct {
				Change      string  `json:"change"`
				SamePackage bool    `json:"same_package"`
				Dependency  *string `json:"package_dependency"`
				Sites       []struct {
					File string `json:"file"`
					Rev  string `json:"rev"`
				} `json:"sites"`
			} `json:"calls"`
		} `json:"functions"`
		Uses struct {
			Prior   []struct{ Package string } `json:"existing"`
			New     []struct{ Package string } `json:"new"`
			Outside []struct{ Package string } `json:"outside"`
		} `json:"uses"`
	} `json:"new_packages"`
	RemovedPackages []struct {
		Package   string `json:"package"`
		Owners    int    `json:"owners"`
		Calls     int    `json:"calls"`
		Functions []struct {
			Function string `json:"function"`
			Calls    []struct {
				Change string `json:"change"`
				Sites  []struct {
					Rev string `json:"rev"`
				} `json:"sites"`
			} `json:"calls"`
		} `json:"functions"`
	} `json:"removed_packages"`
	Renamed []struct {
		Package string `json:"package"`
		From    string `json:"from"`
		To      string `json:"to"`
	} `json:"renamed"`
}

func (c checkout) report(args ...string) report {
	c.t.Helper()
	got := c.changed(append([]string{"--format", "json"}, args...)...)
	require.Equal(c.t, cli.ExitClean, got.code)
	require.Empty(c.t, got.stderr)
	var doc report
	require.NoError(c.t, json.Unmarshal([]byte(got.stdout), &doc))
	return doc
}

// jsonGolden compares the JSON report with testdata/changed/<name>/json.golden,
// or rewrites the file when the test binary runs with -update.
func jsonGolden(t *testing.T, name string, got outcome) {
	t.Helper()
	path := filepath.Join("testdata", "changed", name, "json.golden")
	if *update {
		require.NoError(t, os.WriteFile(path, []byte(got.stdout), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), got.stdout)
}

// TestChangedJSONGoldens covers the exact bytes of the JSON report.
//
// Goal: the JSON report of each shape of change equals its golden file, exits
// 0, and writes nothing to stderr.
func TestChangedJSONGoldens(t *testing.T) {
	for _, name := range []string{
		"call-added", "call-removed", "sibling-owner", "empty", "new-package",
		"removed-package", "rename", "partial-load", "generated-head-only",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := fixture(t, name).changed("--format", "json")
			assert.Equal(t, cli.ExitClean, got.code)
			assert.Empty(t, got.stderr)
			jsonGolden(t, name, got)
		})
	}
}

// TestChangedJSONGivesEveryCallItsSites covers the point of the format.
//
// Goal: an added call carries a head site with the file and line where the
// call is written, a removed call carries a base site, and head has no ref.
func TestChangedJSONGivesEveryCallItsSites(t *testing.T) {
	c := fixture(t, "call-added")
	doc := c.report()
	assert.Equal(t, "tiger.changed/v1", doc.Schema)
	assert.Equal(t, "main", doc.Base.Ref)
	assert.Equal(t, c.git("merge-base", "main", "HEAD"), doc.Base.Commit)
	assert.Equal(t, map[string]any{"commit": c.git("rev-parse", "HEAD"), "dirty": false}, doc.Head)
	require.Len(t, doc.Functions, 1)
	call := doc.Functions[0].Calls[0]
	assert.Equal(t, "added", call.Change)
	assert.Equal(t, "example.com/fakedb.Pool.Exec", call.Callee)
	assert.Equal(t, "Exec", call.Method)
	require.Len(t, call.Sites, 1)
	assert.Equal(t, "head", call.Sites[0].Rev)
	assert.Equal(t, "web/web.go", call.Sites[0].File)
	line := strings.Split(c.read("web/web.go"), "\n")[call.Sites[0].Line-1]
	assert.Contains(t, line, "Exec(")

	removed := fixture(t, "call-removed").report()
	require.Len(t, removed.Functions, 1)
	require.Len(t, removed.Functions[0].Calls[0].Sites, 1)
	assert.Equal(t, "removed", removed.Functions[0].Calls[0].Change)
	assert.Equal(t, "base", removed.Functions[0].Calls[0].Sites[0].Rev)
}

// TestChangedJSONMatchesTheTextList covers acceptance criterion 16.
//
// Goal: functions has one entry per header and one calls entry per + or -
// line of the text list, in the same order, and the counts equal the lines.
func TestChangedJSONMatchesTheTextList(t *testing.T) {
	for _, name := range []string{
		"call-added", "call-removed", "sibling-owner", "new-package", "removed-package",
		"rename", "module-helper", "closure", "interface-call", "unchanged-caller",
		"sibling-owner-removed", "helper-refactor", "mixed-filter-method",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := fixture(t, name)
			var want []string
			lines := strings.Split(c.changed().stdout, "\n")
			for i, line := range lines {
				if !strings.HasPrefix(line, "Calls added:") {
					continue
				}
				for _, listed := range lines[i+2:] {
					if listed == "" || strings.HasPrefix(listed, "No function") {
						break
					}
					want = append(want, listed)
				}
			}
			var got []string
			doc := c.report()
			added, removed := 0, 0
			for _, fn := range doc.Functions {
				got = append(got, fn.Function)
				for _, call := range fn.Calls {
					sign := "-"
					if call.Change == "added" {
						sign = "+"
						added++
					} else {
						removed++
					}
					got = append(got, sign+"   "+call.Callee)
				}
			}
			assert.Equal(t, want, got)
			assert.Equal(t, doc.Counts.CallsAdded, added)
			assert.Equal(t, doc.Counts.CallsRemoved, removed)
		})
	}
}

// TestChangedJSONKeepsACallWhoseLineMoved covers sites beside identity.
//
// Goal: a call that exists at both revisions prints no entry even when its
// line number changed.
func TestChangedJSONKeepsACallWhoseLineMoved(t *testing.T) {
	doc := fixture(t, "line-shift").report()
	assert.Empty(t, doc.Functions)
	assert.Equal(t, 0, doc.Counts.CallsAdded)
}

// TestChangedJSONPrintsIdenticalBytes covers acceptance criterion 9.
//
// Goal: the same input prints the same JSON bytes on a second run and with one
// scheduler thread.
func TestChangedJSONPrintsIdenticalBytes(t *testing.T) {
	c := fixture(t, "new-package")
	first := c.changed("--format", "json")
	require.Equal(t, cli.ExitClean, first.code)
	assert.Equal(t, first.stdout, c.changed("--format", "json").stdout)

	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))
	assert.Equal(t, first.stdout, c.changed("--format", "json").stdout)
}

// TestChangedJSONDescribesPackages covers new, removed and renamed packages.
//
// Goal: a new package lists what it uses and its folded calls, a removed
// package lists the calls it had at base, and a rename names both owners.
func TestChangedJSONDescribesPackages(t *testing.T) {
	added := fixture(t, "new-package").report()
	require.Len(t, added.NewPackages, 1)
	pkg := added.NewPackages[0]
	assert.Equal(t, "billing", pkg.Package)
	assert.Positive(t, pkg.Owners)
	assert.Positive(t, pkg.Calls)
	assert.NotEmpty(t, pkg.Functions)

	gone := fixture(t, "removed-package").report()
	require.Len(t, gone.RemovedPackages, 1)
	assert.Positive(t, gone.RemovedPackages[0].Owners)
	assert.Positive(t, gone.RemovedPackages[0].Calls)
	for _, fn := range gone.RemovedPackages[0].Functions {
		for _, call := range fn.Calls {
			assert.Equal(t, "removed", call.Change)
			assert.Equal(t, "base", call.Sites[0].Rev)
		}
	}

	renamed := fixture(t, "rename").report()
	require.Len(t, renamed.Renamed, 1)
	assert.NotEmpty(t, renamed.Renamed[0].From)
	assert.NotEmpty(t, renamed.Renamed[0].To)
	assert.Empty(t, renamed.Functions)
}

// TestChangedJSONNamesPackagesThatDidNotLoad covers partial loads.
//
// Goal: scope.not_compared names the packages left out, and is an empty array
// rather than null when every package loaded.
func TestChangedJSONNamesPackagesThatDidNotLoad(t *testing.T) {
	partial := fixture(t, "partial-load").changed("--format", "json")
	require.Equal(t, cli.ExitClean, partial.code)
	var doc report
	require.NoError(t, json.Unmarshal([]byte(partial.stdout), &doc))
	assert.NotEmpty(t, doc.Scope.NotCompared)

	clean := fixture(t, "call-added").changed("--format", "json")
	assert.Contains(t, clean.stdout, `"not_compared": []`)
	assert.Contains(t, clean.stdout, `"skipped_generated_files": []`)
}

// TestChangedJSONLabelsADirtyHead covers head naming.
//
// Goal: uncommitted edits show as head.dirty true, and head sites point into
// the working tree.
func TestChangedJSONLabelsADirtyHead(t *testing.T) {
	c := fixture(t, "call-added")
	edited := strings.Replace(c.read("web/web.go"),
		`return "hello " + name`, "_, _ = h.pool.Query(name)\n\treturn \"hello \" + name", 1)
	c.write("web/web.go", edited)
	doc := c.report()
	assert.Equal(t, true, doc.Head["dirty"])
	assert.NotContains(t, doc.Head, "ref")
	require.Len(t, doc.Functions, 1)
	require.Len(t, doc.Functions[0].Calls, 2)
	query := doc.Functions[0].Calls[1]
	assert.Equal(t, "example.com/fakedb.Pool.Query", query.Callee)
	line := strings.Split(edited, "\n")[query.Sites[0].Line-1]
	assert.Contains(t, line, "Query(")
}

// TestChangedJSONTellsWhatACallDidToDependencies covers package_dependency.
//
// Goal: a call into a package the owner already called is kept, a call into a
// package it did not call is new, a removed call into a package it no longer
// calls is dropped, and a same-package call has no dependency.
func TestChangedJSONTellsWhatACallDidToDependencies(t *testing.T) {
	added := fixture(t, "call-added").report()
	dependency := added.Functions[0].Calls[0].PackageDependency
	require.NotNil(t, dependency)
	assert.Equal(t, "new", *dependency)

	removed := fixture(t, "call-removed").report()
	dependency = removed.Functions[0].Calls[0].PackageDependency
	require.NotNil(t, dependency)
	assert.Equal(t, "dropped", *dependency)

	sibling := fixture(t, "sibling-owner").report()
	call := sibling.Functions[0].Calls[0]
	assert.True(t, call.SamePackage)
	assert.Nil(t, call.PackageDependency)
}

// TestChangedRefusesAnUnknownFormat covers the format flag.
//
// Goal: a format tiger does not print exits 2 with one line on stderr and
// nothing on stdout.
func TestChangedRefusesAnUnknownFormat(t *testing.T) {
	got := fixture(t, "call-added").changed("--format", "yaml")
	assert.Equal(t, cli.ExitOperational, got.code)
	assert.Empty(t, got.stdout)
	assert.Equal(t,
		"tiger changed: unknown format \"yaml\"; the formats are: text, markdown, json\n", got.stderr)
}

func sourceLines(source string, sites ...int) []string {
	lines := strings.Split(source, "\n")
	var found []string
	for _, line := range sites {
		found = append(found, strings.TrimSpace(lines[line-1]))
	}
	return found
}

// TestChangedJSONKeepsEverySiteOfACall covers sites of one call.
//
// Goal: a call written on three lines is one entry with three head sites in
// ascending line order, a removed call written twice has two base sites, a free
// function has a null method, and calls into a package the function still
// calls are kept.
func TestChangedJSONKeepsEverySiteOfACall(t *testing.T) {
	c := fixture(t, "json-sites")
	doc := c.report()
	require.Len(t, doc.Functions, 1)
	fn := doc.Functions[0]
	assert.Equal(t, "web.Save", fn.Function)
	assert.Equal(t, "web.Save", fn.Owner)
	assert.Nil(t, fn.Method)
	require.Len(t, fn.Calls, 2)

	// The text list prints removed calls before added ones, and so does JSON.
	removed, added := fn.Calls[0], fn.Calls[1]
	assert.Equal(t, "added", added.Change)
	assert.Equal(t, "example.com/fakedb.Pool.Exec", added.Callee)
	require.Len(t, added.Sites, 3)
	var lines []int
	for _, site := range added.Sites {
		assert.Equal(t, "head", site.Rev)
		assert.Equal(t, "web/web.go", site.File)
		lines = append(lines, site.Line)
	}
	assert.IsIncreasing(t, lines)
	assert.Equal(t, []string{`_ = pool.Exec("b")`, `_ = pool.Exec("c")`, `_ = pool.Exec("d")`},
		sourceLines(c.read("web/web.go"), lines...))
	require.NotNil(t, added.PackageDependency)
	assert.Equal(t, "kept", *added.PackageDependency)

	assert.Equal(t, "removed", removed.Change)
	assert.Equal(t, "example.com/fakedb.Pool.Close", removed.Callee)
	require.Len(t, removed.Sites, 2)
	lines = nil
	for _, site := range removed.Sites {
		assert.Equal(t, "base", site.Rev)
		assert.Equal(t, "web/web.go", site.File)
		lines = append(lines, site.Line)
	}
	assert.IsIncreasing(t, lines)
	assert.Equal(t, []string{"_ = pool.Close()", "_ = pool.Close()"},
		sourceLines(c.git("show", "main:web/web.go"), lines...))
	require.NotNil(t, removed.PackageDependency)
	assert.Equal(t, "kept", *removed.PackageDependency)
}

// TestChangedJSONPointsAtTheCallInsideAClosure covers calls in closures.
//
// Goal: a call written inside a closure belongs to the enclosing method and
// its site is the line of the call in the closure.
func TestChangedJSONPointsAtTheCallInsideAClosure(t *testing.T) {
	c := fixture(t, "closure")
	doc := c.report()
	require.Len(t, doc.Functions, 1)
	assert.Equal(t, "web.Handler.Greet", doc.Functions[0].Function)
	for _, call := range doc.Functions[0].Calls {
		require.NotEmpty(t, call.Sites)
		for _, site := range call.Sites {
			line := sourceLines(c.read("web/web.go"), site.Line)[0]
			assert.Contains(t, line, call.Method+"(")
		}
	}
}

// TestChangedJSONNamesSitesRelativeToTheRunDirectory covers site paths.
//
// Goal: when tiger runs in a subdirectory of the repository, head and base site
// files are relative to that directory and the lines are those of the file at
// that revision.
func TestChangedJSONNamesSitesRelativeToTheRunDirectory(t *testing.T) {
	build := func(base, head string) checkout {
		c := newCheckout(t)
		c.write("go.mod", "module example.com/mono\n\ngo 1.26\n")
		c.write("libs/ids/ids.go", "package ids\n\nfunc New() string { return \"id\" }\n")
		c.write("app/internal/store/store.go", storeSource)
		c.write("app/app.go", base)
		c.commit("base")
		c.git("checkout", "-q", "-b", "feature")
		c.write("app/app.go", head)
		c.commit("head")
		return c
	}
	load := func(c checkout) report {
		got := run(t, "changed", "-C", filepath.Join(c.dir, "app"), "--format", "json", "main")
		require.Equal(t, cli.ExitClean, got.code)
		require.Empty(t, got.stderr)
		var doc report
		require.NoError(t, json.Unmarshal([]byte(got.stdout), &doc))
		return doc
	}

	c := build(appBase, appHead)
	doc := load(c)
	require.Len(t, doc.Functions, 1)
	require.Len(t, doc.Functions[0].Calls, 2)
	for _, call := range doc.Functions[0].Calls {
		require.Len(t, call.Sites, 1)
		assert.Equal(t, "head", call.Sites[0].Rev)
		assert.Equal(t, "app.go", call.Sites[0].File)
		line := call.Sites[0].Line
		assert.Contains(t, sourceLines(c.read("app/app.go"), line)[0], call.Method+"(")
	}

	c = build(appHead, appBase)
	doc = load(c)
	require.Len(t, doc.Functions, 1)
	require.Len(t, doc.Functions[0].Calls, 2)
	for _, call := range doc.Functions[0].Calls {
		assert.Equal(t, "removed", call.Change)
		require.Len(t, call.Sites, 1)
		assert.Equal(t, "base", call.Sites[0].Rev)
		assert.Equal(t, "app.go", call.Sites[0].File)
		line := call.Sites[0].Line
		assert.Contains(t, sourceLines(c.git("show", "main:app/app.go"), line)[0], call.Method+"(")
	}
}

// TestChangedJSONDescribesCallsInANewPackage covers calls beside a new package.
//
// Goal: the new package lists its functions, each call of each function has a
// head site in a file of that package, and a call between two functions of the
// package is marked same_package with a null package_dependency.
func TestChangedJSONDescribesCallsInANewPackage(t *testing.T) {
	c := fixture(t, "new-package")
	doc := c.report()
	require.Len(t, doc.NewPackages, 1)
	calls, same := 0, 0
	for _, fn := range doc.NewPackages[0].Functions {
		for _, call := range fn.Calls {
			calls++
			assert.Equal(t, "added", call.Change)
			require.NotEmpty(t, call.Sites)
			for _, site := range call.Sites {
				assert.Equal(t, "head", site.Rev)
				assert.Equal(t, "billing/billing.go", site.File)
			}
			if call.SamePackage {
				same++
				assert.Nil(t, call.Dependency)
			}
		}
	}
	assert.Equal(t, doc.NewPackages[0].Calls, calls)
	assert.Positive(t, same)
}

// TestChangedJSONCountsInitializerCallsAsTheOwnersDependencies covers
// package_dependency for package-level initializers.
//
// Goal: a call added in a package initializer into a package that the
// initializer already called at base is kept, not new.
func TestChangedJSONCountsInitializerCallsAsTheOwnersDependencies(t *testing.T) {
	doc := fixture(t, "initializer-kept").report()
	require.Len(t, doc.Functions, 1)
	require.Len(t, doc.Functions[0].Calls, 1)
	call := doc.Functions[0].Calls[0]
	assert.Equal(t, "example.com/fakedb.Pool.Query", call.Callee)
	require.NotNil(t, call.PackageDependency)
	assert.Equal(t, "kept", *call.PackageDependency)
}

// TestChangedJSONGivesOneSitePerLine covers what a site identifies.
//
// Goal: two calls to the same callee on one line give one site, because a
// site is a file and line and the reader opens the same place for both.
func TestChangedJSONGivesOneSitePerLine(t *testing.T) {
	c := newCheckout(t)
	c.write("go.mod", shopModule)
	c.copyFixture("fakedb", "fakedb")
	c.write("web/web.go", "package web\n\nimport \"example.com/fakedb\"\n\n"+
		"func Run(pool *fakedb.Pool) {\n}\n")
	c.commit("base")
	c.git("checkout", "-q", "-b", "feature")
	c.write("web/web.go", "package web\n\nimport \"example.com/fakedb\"\n\n"+
		"func Run(pool *fakedb.Pool) {\n"+
		"\t_, _ = pool.Query(\"a\"); _, _ = pool.Query(\"b\")\n}\n")
	c.commit("head")

	doc := c.report()
	require.Len(t, doc.Functions, 1)
	require.Len(t, doc.Functions[0].Calls, 1)
	require.Len(t, doc.Functions[0].Calls[0].Sites, 1)
	assert.Equal(t, 6, doc.Functions[0].Calls[0].Sites[0].Line)
}

// TestChangedJSONIgnoresLineDirectives covers what a site points at.
//
// Goal: a call under a //line directive is reported at the file and line that
// hold it, not at the file and line the directive names.
func TestChangedJSONIgnoresLineDirectives(t *testing.T) {
	const base = "package web\n\nimport \"example.com/fakedb\"\n\n" +
		"func Run(pool *fakedb.Pool) {\n}\n"
	c := newCheckout(t)
	c.write("go.mod", shopModule)
	c.copyFixture("fakedb", "fakedb")
	c.write("web/web.go", base)
	c.commit("base")
	c.git("checkout", "-q", "-b", "feature")
	c.write("web/web.go", "package web\n\nimport \"example.com/fakedb\"\n\n"+
		"func Run(pool *fakedb.Pool) {\n//line elsewhere.go:100\n\t_, _ = pool.Query(\"a\")\n}\n")
	c.commit("head")

	site := c.report().Functions[0].Calls[0].Sites[0]
	assert.Equal(t, "web/web.go", site.File)
	assert.Equal(t, 7, site.Line)
}
