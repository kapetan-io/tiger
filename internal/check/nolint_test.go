package check_test

import (
	"maps"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/command"
)

// nolintTree copies the nolint fixture and, when config is not empty,
// writes it as the tree's tiger.yaml.
func nolintTree(t *testing.T, config string) string {
	t.Helper()
	dir := copyFixture(t, "nolint")
	if config != "" {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "tiger.yaml"), []byte(config), 0o644))
	}
	return dir
}

// TestNolintBanOffIgnoresNolint covers the ban being off by default.
//
// Goal: without a nolint entry, and with forbid: false, //nolint comments
// produce no finding.
func TestNolintBanOffIgnoresNolint(t *testing.T) {
	for _, test := range []struct {
		name   string
		config string
	}{
		{name: "NoConfigFile"},
		{
			name: "ForbidFalse",
			config: "version: 1\nnolint:\n  forbid: false\n" +
				"  reason: the team uses nolint for third-party linters\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := run(t, "check", "-C", nolintTree(t, test.config), "./...")
			assert.Equal(t, command.ExitClean, got.code)
			assert.Empty(t, got.stdout)
			assert.Empty(t, got.stderr)
		})
	}
}

// TestNolintBanOnBlocksEveryNolint covers nolint.forbid.
//
// Goal: with the ban on, each //nolint comment is its own blocking TS-L09
// finding reported at the file's package line, whichever spelling it uses.
func TestNolintBanOnBlocksEveryNolint(t *testing.T) {
	dir := nolintTree(t, "version: 1\nnolint:\n  forbid: true\n"+
		"  reason: agents must fix findings, not silence them\n")
	got := run(t, "check", "-C", dir, "./...")
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	const rest = " has a //nolint, and this repository's tiger.yaml forbids them " +
		"(nolint.forbid) — fix the finding it silences instead\n"
	assert.Contains(t, got.stdout, "nolint.go:2:1: TS-L09: line 7"+rest)
	assert.Contains(t, got.stdout, "nolint.go:2:1: TS-L09: line 13"+rest)
	assert.Contains(t, got.stdout, "tiger: 2 blocking\n")
}

// TestNolintBanRejectsInvalidEntries covers the nolint block's validation.
//
// Goal: a ban with no reason, a misspelled key and a non-mapping block each
// exit 2 with one line naming the problem, and no finding prints.
func TestNolintBanRejectsInvalidEntries(t *testing.T) {
	for _, test := range []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name:    "MissingReason",
			config:  "version: 1\nnolint:\n  forbid: true\n",
			wantErr: "tiger.yaml: nolint: forbid: the entry has no reason",
		},
		{
			name:    "MisspelledKey",
			config:  "version: 1\nnolint:\n  forbd: true\n  reason: x\n",
			wantErr: "tiger.yaml: nolint: forbd: no setting has this name",
		},
		{
			name:    "NotAMapping",
			config:  "version: 1\nnolint: true\n",
			wantErr: "tiger.yaml: nolint: the block must hold forbid: and reason:",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := run(t, "check", "-C", nolintTree(t, test.config), "./...")
			assert.Equal(t, command.ExitOperational, got.code)
			assert.Empty(t, got.stdout)
			assert.Contains(t, got.stderr, test.wantErr)
		})
	}
}

// TestOpenEnumCountsAgainstBudget covers openenum accounting.
//
// Goal: a //tiger:openenum with a reason passes TS-S08 with a plain
// default arm, and counts as one escape directive: a missing budget row
// fails the run, and a row of 1 passes it.
func TestOpenEnumCountsAgainstBudget(t *testing.T) {
	got := run(t, "check", "-C", filepath.Join("testdata", "fixtures", "openenum"), "./...")
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, "tiger.budget.yaml: TS-D06: . has 1 escape directive and "+
		"tiger.budget.yaml has no row for it — fix it, or run tiger budget --write to record "+
		"the current count\n"+
		"mode.go:7:1: TS-L09: //tiger:openenum \"peers on the wire add modes before we upgrade\" "+
		"waives a rule here; tiger cannot check the claim, so this notice stands on every run — "+
		"remove the directive when the constraint no longer holds\n"+
		"tiger: 2 blocking\n", got.stdout)

	dir := copyFixture(t, "openenum")
	require.NoError(t, os.WriteFile(filepath.Join(dir, budgetFile),
		[]byte(".:\n  TS-L09: 1\n"), 0o644))
	budgeted := run(t, "check", "-C", dir, "./...")
	assert.Equal(t, command.ExitClean, budgeted.code)
	assert.Empty(t, budgeted.stdout)
	assert.Empty(t, budgeted.stderr)
}

// TestOpenEnumWithoutReasonIsBlocking covers the reason requirement.
//
// Goal: a bare //tiger:openenum is a blocking TS-L09 finding, and because
// it does not parse it grants no waiver, so the default arm is a TS-S08
// finding too.
func TestOpenEnumWithoutReasonIsBlocking(t *testing.T) {
	got := run(t, "check", "-C", filepath.Join("testdata", "fixtures", "reasonless"), "./...")
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Contains(t, got.stdout, "mode.go:6:1: TS-L09: //tiger:openenum has no reason after it")
	assert.Contains(t, got.stdout, "mode.go:21:2: TS-S08: this switch over Mode has a default arm")
}

// writeTree writes files (relative path to content) plus a go.mod into a
// fresh directory.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	tree := fstest.MapFS{"go.mod": {Data: []byte("module fixture.example/tree\n\ngo 1.26\n")}}
	maps.Copy(tree, mapFiles(files))
	dir := t.TempDir()
	require.NoError(t, os.CopyFS(dir, tree))
	return dir
}

// mapFiles wraps file contents as an in-memory filesystem's entries.
func mapFiles(files map[string]string) fstest.MapFS {
	entries := fstest.MapFS{}
	for name, content := range files {
		entries[name] = &fstest.MapFile{Data: []byte(content)}
	}
	return entries
}

const banConfig = "version: 1\nnolint:\n  forbid: true\n  reason: agents must fix findings\n"

// TestNolintBanSpellings covers which comments count as a //nolint.
//
// Goal: the bare, spaced, colon and trailing-prose spellings are findings;
// look-alikes such as //nolintlint, //nolinter and prose mentioning nolint
// are not.
func TestNolintBanSpellings(t *testing.T) {
	for _, test := range []struct {
		name     string
		comment  string
		wantHits int
	}{
		{name: "Bare", comment: "//nolint", wantHits: 1},
		{name: "SpaceAfterSlashes", comment: "// nolint", wantHits: 1},
		{name: "ManySpaces", comment: "//   nolint:all", wantHits: 1},
		{name: "ColonList", comment: "//nolint:gosec,errcheck", wantHits: 1},
		{name: "TrailingReason", comment: "//nolint // generated", wantHits: 1},
		{name: "LongerWord", comment: "//nolintlint", wantHits: 0},
		{name: "LongerWordColon", comment: "//nolinter:x", wantHits: 0},
		{name: "ProseMention", comment: "// we never use nolint here", wantHits: 0},
		{name: "ExtraSlashes", comment: "///nolint", wantHits: 1},
		{name: "SlashSpaceSlash", comment: "// / nolint", wantHits: 1},
		{name: "BlockComment", comment: "/* nolint */", wantHits: 0},
		{name: "TabAfterSlashes", comment: "//\tnolint", wantHits: 0},
		{name: "UpperCase", comment: "//NOLINT", wantHits: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := writeTree(t, map[string]string{
				"tiger.yaml": banConfig,
				"a.go":       "package tree\n\nvar x = 1 " + test.comment + "\n",
			})
			got := run(t, "check", "-C", dir, "./...")
			assert.Empty(t, got.stderr)
			if test.wantHits == 0 {
				assert.Equal(t, command.ExitClean, got.code)
				assert.Empty(t, got.stdout)
				return
			}
			assert.Equal(t, command.ExitFindings, got.code)
			assert.Contains(t, got.stdout, "a.go:1:1: TS-L09: line 3 has a //nolint")
			assert.Contains(t, got.stdout, "tiger: 1 blocking\n")
		})
	}
}

// TestNolintBanCoversTestFilesAndEveryPackage covers "every //nolint in a
// checked package".
//
// Goal: a //nolint in a _test.go file and in a nested package each produce
// a finding at that file's own package line.
func TestNolintBanCoversTestFilesAndEveryPackage(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"tiger.yaml":       banConfig,
		"a_test.go":        "package tree_test\n\nvar _ = 1 //nolint:unused\n",
		"sub/pkg/inner.go": "// Package pkg is nested.\npackage pkg\n\nvar y = 1 //nolint\n",
		"clean.go":         "package tree\n",
	})
	got := run(t, "check", "-C", dir, "./...")
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Contains(t, got.stdout, "a_test.go:1:1: TS-L09: line 3 has a //nolint")
	assert.Contains(t, got.stdout, "sub/pkg/inner.go:2:1: TS-L09: line 4 has a //nolint")
	assert.Contains(t, got.stdout, "tiger: 2 blocking\n")
}

// TestNolintBanSkipsGeneratedFiles covers the generated-file policy.
//
// Goal: a //nolint inside a generated file is not reported.
func TestNolintBanSkipsGeneratedFiles(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"tiger.yaml": banConfig,
		"gen.go": "// Code generated by hand. DO NOT EDIT.\n\npackage tree\n\n" +
			"var x = 1 //nolint\n",
	})
	got := run(t, "check", "-C", dir, "./...")
	assert.Equal(t, command.ExitClean, got.code)
	assert.Empty(t, got.stdout)
}

// TestNolintBlockLoadsAlongsideAnalyzerBlocks covers the shared document.
//
// Goal: a nolint block next to a participle block loads, both take effect,
// and the nolint block comes after or before the analyzer block alike.
func TestNolintBlockLoadsAlongsideAnalyzerBlocks(t *testing.T) {
	source := "package tree\n\ntype KeySigning struct{}\n\nvar x = 1 //nolint\n"
	analyzer := "participle:\n  allow:\n    - value: signing\n" +
		"      reason: KeySigning is a credential record\n"
	for _, test := range []struct {
		name   string
		config string
	}{
		{name: "NolintFirst", config: banConfig + analyzer},
		{name: "NolintLast", config: "version: 1\n" + analyzer + banConfig[len("version: 1\n"):]},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := writeTree(t, map[string]string{"tiger.yaml": test.config, "a.go": source})
			got := run(t, "check", "-C", dir, "./...")
			assert.Empty(t, got.stderr)
			assert.Equal(t, command.ExitFindings, got.code)
			assert.Contains(t, got.stdout, "TS-L09: line 5 has a //nolint")
			assert.NotContains(t, got.stdout, "TS-N14")
			assert.Contains(t, got.stdout, "tiger: 1 blocking\n")
		})
	}
}

// TestNolintBanRejectsBlankReasonAndWrongTypes covers more invalid blocks.
//
// Goal: a whitespace-only reason, a non-boolean forbid and a duplicate
// nolint block each exit 2 and print no finding.
func TestNolintBanRejectsBlankReasonAndWrongTypes(t *testing.T) {
	for _, test := range []struct {
		name   string
		config string
	}{
		{name: "BlankReason", config: "version: 1\nnolint:\n  forbid: true\n  reason: \"  \"\n"},
		{name: "ForbidNotBool", config: "version: 1\nnolint:\n  forbid: [a]\n  reason: x\n"},
		{name: "ListBlock", config: "version: 1\nnolint:\n  - forbid: true\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := run(t, "check", "-C", nolintTree(t, test.config), "./...")
			assert.Equal(t, command.ExitOperational, got.code)
			assert.Empty(t, got.stdout)
			assert.Contains(t, got.stderr, "tiger.yaml: nolint")
		})
	}
}

// TestNolintBanStaysOnAcrossRuns covers the process-wide config install.
//
// Goal: a run with the ban on followed by a run on a tree with no config
// reports nothing, so the ban does not leak between invocations.
func TestNolintBanStaysOnAcrossRuns(t *testing.T) {
	on := run(t, "check", "-C", nolintTree(t, banConfig), "./...")
	assert.Equal(t, command.ExitFindings, on.code)

	off := run(t, "check", "-C", nolintTree(t, ""), "./...")
	assert.Equal(t, command.ExitClean, off.code)
	assert.Empty(t, off.stdout)
}

// TestOpenEnumReasonRequirement covers the reason on every declaration form.
//
// Goal: a whitespace-only reason is a TS-L09 finding, and a reasoned
// openenum on a grouped type declaration counts against the budget.
func TestOpenEnumReasonRequirement(t *testing.T) {
	const body = "\n\nconst (\n\tA Mode = iota\n\tB\n)\n\nfunc Name(mode Mode) string {\n" +
		"\tswitch mode {\n\tcase A:\n\t\treturn \"a\"\n\tcase B:\n\t\treturn \"b\"\n" +
		"\tdefault:\n\t\treturn \"?\"\n\t}\n}\n"
	blank := writeTree(t, map[string]string{
		"a.go": "package tree\n\n//tiger:openenum   \ntype Mode int" + body,
	})
	got := run(t, "check", "-C", blank, "./...")
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Contains(t, got.stdout, "a.go:3:1: TS-L09: //tiger:openenum has no reason after it")

	grouped := writeTree(t, map[string]string{
		"a.go": "package tree\n\ntype (\n\t//tiger:openenum peers add modes before we upgrade\n" +
			"\tMode int\n\n\tOther int\n)" + body,
		"tiger.budget.yaml": ".:\n  TS-L09: 1\n",
	})
	again := run(t, "check", "-C", grouped, "./...")
	assert.Empty(t, again.stderr)
	assert.Equal(t, command.ExitClean, again.code)
	assert.Empty(t, again.stdout)
}

// TestBudgetWriteCountsOpenEnum covers the ratchet record.
//
// Goal: tiger budget --write records one TS-L09 for a reasoned openenum, and
// a following check is clean.
func TestBudgetWriteCountsOpenEnum(t *testing.T) {
	dir := copyFixture(t, "openenum")
	written := run(t, "budget", "-C", dir, "--write")
	assert.Equal(t, command.ExitClean, written.code)
	assert.Empty(t, written.stdout)
	assert.Equal(t, ".:\n  TS-L09: 1\n", readFile(t, filepath.Join(dir, budgetFile)))

	checked := run(t, "check", "-C", dir, "./...")
	assert.Equal(t, command.ExitClean, checked.code)
	assert.Empty(t, checked.stdout)
}

// TestOpenEnumOverBudgetFails covers the budget cap.
//
// Goal: a budget row of 0 cannot hold the openenum.
func TestOpenEnumOverBudgetFails(t *testing.T) {
	dir := copyFixture(t, "openenum")
	require.NoError(t, os.WriteFile(filepath.Join(dir, budgetFile),
		[]byte(".:\n  TS-L09: 0\n"), 0o644))
	got := run(t, "check", "-C", dir, "./...")
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Contains(t, got.stdout, "TS-D06")
}
