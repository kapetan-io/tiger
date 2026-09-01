package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/cli"
)

// budgetFile is the fixed file name the ratchet reads and tiger budget
// --write rewrites.
const budgetFile = "tiger.budget.yaml"

// skipMessage is skipcheck's TS-D07 message, printed as an ordinary
// blocking line under a TS-D06 overrun.
const skipMessage = "TS-D07: this test is skipped, so it passes without running — remove " +
	"the Skip call when the test can run again; this notice stands until then\n"

// cliSkips is the four counted findings of the budget tree's internal/cli
// package, in position order.
const cliSkips = "internal/cli/skip_test.go:9:2: " + skipMessage +
	"internal/cli/skip_test.go:16:2: " + skipMessage +
	"internal/cli/skip_test.go:23:2: " + skipMessage +
	"internal/cli/skip_test.go:30:2: " + skipMessage

// budgetTree copies the budget fixture tree and replaces its budget file
// with rows; an empty rows removes the file.
func budgetTree(t *testing.T, rows string) string {
	t.Helper()
	dir := copyFixture(t, filepath.Join("budget", "tree"))
	path := filepath.Join(dir, budgetFile)
	if rows == "" {
		require.NoError(t, os.Remove(path))
		return dir
	}
	require.NoError(t, os.WriteFile(path, []byte(rows), 0o644))
	return dir
}

// TestCheckOverBudgetFailsAtTheRow covers acceptance criterion 1 and 9.
//
// Goal: four skipped tests against a row of 2 exit 1 with one TS-D06 line
// positioned at the row, followed by the four TS-D07 findings as blocking
// lines, and a run over internal/cli alone never mentions internal/driver's
// missing row.
func TestCheckOverBudgetFailsAtTheRow(t *testing.T) {
	got := run(t, "check", "-C", filepath.Join("testdata", "fixtures", "budget", "tree"),
		"./internal/cli")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, "tiger.budget.yaml:2: TS-D06: internal/cli has 4 skipped tests but "+
		"tiger.budget.yaml allows 2 — fix 2 of them, or raise the number in a reviewed edit\n"+
		cliSkips+
		"tiger: 5 blocking\n", got.stdout)
}

// TestCheckWholeTreeReportsEveryOverrun covers the ratchet over a run with
// blocking analyzer findings and a package with no row.
//
// Goal: blocking analyzer findings print first, then one group per
// overrun package in path order, and the trailer counts every printed
// line; a package under budget prints nothing.
func TestCheckWholeTreeReportsEveryOverrun(t *testing.T) {
	got := run(t, "check", "-C", filepath.Join("testdata", "fixtures", "budget", "tree"), "./...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, "internal/store/store.go:18:2: TS-S18: panic is called directly here — "+
		"use assert.Ok (condition), assert.Fail (formatted failure), or assert.Unreachable "+
		"(impossible arm) instead so every crash goes through one path\n"+
		"tiger.budget.yaml:2: TS-D06: internal/cli has 4 skipped tests but "+
		"tiger.budget.yaml allows 2 — fix 2 of them, or raise the number in a reviewed edit\n"+
		cliSkips+
		"tiger.budget.yaml: TS-D06: internal/driver has 1 skipped test and tiger.budget.yaml "+
		"has no row for it — fix it, or run tiger budget --write to record the current count\n"+
		"internal/driver/skip_test.go:9:2: "+skipMessage+
		"tiger: 8 blocking\n", got.stdout)
	assert.NotContains(t, got.stdout, "TS-L09")
}

// TestCheckWithinBudgetIsSilent covers acceptance criteria 2 and 3 and
// behavioral constraint B7.
//
// Goal: a row equal to or above the count exits 0 and prints nothing.
func TestCheckWithinBudgetIsSilent(t *testing.T) {
	for _, test := range []struct {
		name string
		rows string
	}{
		{name: "AtBudget", rows: "internal/cli:\n  TS-D07: 4\n"},
		{name: "UnderBudget", rows: "internal/cli:\n  TS-D07: 6\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := run(t, "check", "-C", budgetTree(t, test.rows), "./internal/cli")
			assert.Equal(t, cli.ExitClean, got.code)
			assert.Empty(t, got.stdout)
			assert.Empty(t, got.stderr)
		})
	}
}

// TestCheckMissingRowFails covers acceptance criterion 4 and behavioral
// constraint B3.
//
// Goal: a package with counted findings and no row fails with a TS-D06
// line positioned at the file path with no line number, whether the file
// has other rows or does not exist.
func TestCheckMissingRowFails(t *testing.T) {
	want := "tiger.budget.yaml: TS-D06: internal/cli has 4 skipped tests and " +
		"tiger.budget.yaml has no row for it — fix them, or run tiger budget --write to " +
		"record the current count\n" + cliSkips + "tiger: 5 blocking\n"
	for _, test := range []struct {
		name string
		rows string
	}{
		{name: "FilePresent", rows: "internal/store:\n  TS-L09: 3\n"},
		{name: "FileAbsent", rows: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := run(t, "check", "-C", budgetTree(t, test.rows), "./internal/cli")
			assert.Equal(t, cli.ExitFindings, got.code)
			assert.Empty(t, got.stderr)
			assert.Equal(t, want, got.stdout)
		})
	}
}

// TestCheckNeverWritesTheBudgetFile covers behavioral constraint B1.
//
// Goal: tiger check leaves tiger.budget.yaml byte-identical, over budget
// and with slack alike, and creates no file when none exists.
func TestCheckNeverWritesTheBudgetFile(t *testing.T) {
	slack := budgetTree(t, "internal/cli:\n  TS-D07: 9\n")
	require.Equal(t, cli.ExitClean, run(t, "check", "-C", slack, "./internal/cli").code)
	assert.Equal(t, "internal/cli:\n  TS-D07: 9\n", readFile(t, filepath.Join(slack, budgetFile)))

	absent := budgetTree(t, "")
	require.Equal(t, cli.ExitFindings, run(t, "check", "-C", absent, "./...").code)
	assert.NoFileExists(t, filepath.Join(absent, budgetFile))
}

// TestBudgetWriteLowersSlack covers acceptance criterion 5 and invariants
// I2 and I4.
//
// Goal: --write lowers a row of 6 to the count of 4 and a row of 3 to 1,
// exits 0, and a second run leaves the file byte-identical.
func TestBudgetWriteLowersSlack(t *testing.T) {
	dir := budgetTree(t, "internal/cli:\n  TS-D07: 6\ninternal/driver:\n  TS-D07: 1\n"+
		"internal/store:\n  TS-L09: 3\n")
	first := run(t, "budget", "-C", dir, "--write")
	assert.Equal(t, cli.ExitClean, first.code)
	assert.Empty(t, first.stdout)
	assert.Empty(t, first.stderr)
	want := "internal/cli:\n  TS-D07: 4\ninternal/driver:\n  TS-D07: 1\n" +
		"internal/store:\n  TS-L09: 1\n"
	assert.Equal(t, want, readFile(t, filepath.Join(dir, budgetFile)))

	second := run(t, "budget", "-C", dir, "--write")
	assert.Equal(t, cli.ExitClean, second.code)
	assert.Equal(t, want, readFile(t, filepath.Join(dir, budgetFile)))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, entry := range entries {
		assert.Contains(t, []string{"go.mod", "internal", budgetFile}, entry.Name())
	}
}

// TestBudgetWriteKeepsOverrunRows covers acceptance criterion 6 and core
// principle 1: tiger never raises a budget.
//
// Goal: a row below the count stays where it is and is reported, the run
// exits 1, and every other row with slack in the same run is still
// lowered.
func TestBudgetWriteKeepsOverrunRows(t *testing.T) {
	dir := budgetTree(t, "internal/cli:\n  TS-D07: 2\ninternal/driver:\n  TS-D07: 1\n"+
		"internal/store:\n  TS-L09: 3\n")
	got := run(t, "budget", "-C", dir, "--write")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, "tiger.budget.yaml:2: TS-D06: internal/cli has 4 skipped tests but "+
		"tiger.budget.yaml allows 2 — fix 2 of them, or raise the number in a reviewed edit\n"+
		cliSkips, got.stdout)
	assert.Equal(t, "internal/cli:\n  TS-D07: 2\ninternal/driver:\n  TS-D07: 1\n"+
		"internal/store:\n  TS-L09: 1\n", readFile(t, filepath.Join(dir, budgetFile)))
}

// TestBudgetWriteDeletesEmptyRows covers acceptance criterion 7.
//
// Goal: a row whose count is now 0 is deleted, a package block with no
// rows left disappears, and a package outside the run is untouched.
func TestBudgetWriteDeletesEmptyRows(t *testing.T) {
	dir := budgetTree(t, "internal/cli:\n  TS-D07: 4\n  TS-L09: 1\ninternal/driver:\n"+
		"  TS-D07: 1\n  TS-L09: 2\ninternal/other:\n  TS-D07: 7\n")
	got := run(t, "budget", "-C", dir, "--write", "./internal/cli", "./internal/driver")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Equal(t, "internal/cli:\n  TS-D07: 4\ninternal/driver:\n  TS-D07: 1\n"+
		"internal/other:\n  TS-D07: 7\n", readFile(t, filepath.Join(dir, budgetFile)))
}

// TestBudgetWriteCreatesMissingRows covers first adoption.
//
// Goal: with no file, --write records every analyzed package's current
// count, creates no row at 0, and exits 0.
func TestBudgetWriteCreatesMissingRows(t *testing.T) {
	dir := budgetTree(t, "")
	got := run(t, "budget", "-C", dir, "--write")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Equal(t, "internal/cli:\n  TS-D07: 4\ninternal/driver:\n  TS-D07: 1\n"+
		"internal/store:\n  TS-L09: 1\n", readFile(t, filepath.Join(dir, budgetFile)))
}

// TestBudgetReportsOverrunsOnly covers the tiger budget verdict.
//
// Goal: without --write the command prints the overrun groups and no
// analyzer finding, exits 1 on overrun, exits 0 and prints nothing within
// budget, and never touches the file.
func TestBudgetReportsOverrunsOnly(t *testing.T) {
	rows := "internal/cli:\n  TS-D07: 2\ninternal/driver:\n  TS-D07: 1\ninternal/store:\n" +
		"  TS-L09: 3\n"
	dir := budgetTree(t, rows)
	over := run(t, "budget", "-C", dir)
	assert.Equal(t, cli.ExitFindings, over.code)
	assert.Empty(t, over.stderr)
	assert.Equal(t, "tiger.budget.yaml:2: TS-D06: internal/cli has 4 skipped tests but "+
		"tiger.budget.yaml allows 2 — fix 2 of them, or raise the number in a reviewed edit\n"+
		cliSkips, over.stdout)
	assert.NotContains(t, over.stdout, "TS-S18")
	assert.Equal(t, rows, readFile(t, filepath.Join(dir, budgetFile)))

	within := run(t, "budget", "-C", dir, "./internal/store")
	assert.Equal(t, cli.ExitClean, within.code)
	assert.Empty(t, within.stdout)
	assert.Empty(t, within.stderr)
}

// TestBudgetRejectsInvalidRows covers acceptance criterion 8 and invariant
// I1 from both commands.
//
// Goal: a row naming a blocking code, an unregistered code, a negative or
// non-integer value, or a malformed package key exits 2 from tiger check
// and tiger budget, naming the row, and prints no finding.
func TestBudgetRejectsInvalidRows(t *testing.T) {
	for _, test := range []struct {
		name    string
		rows    string
		wantErr string
	}{
		{
			name:    "BlockingCode",
			rows:    "internal/cli:\n  TS-S02: 1\n",
			wantErr: "TS-S02",
		},
		{
			name:    "UnregisteredCode",
			rows:    "internal/cli:\n  TS-X99: 1\n",
			wantErr: "TS-X99",
		},
		{
			name:    "Negative",
			rows:    "internal/cli:\n  TS-D07: -1\n",
			wantErr: "internal/cli",
		},
		{
			name:    "NonInteger",
			rows:    "internal/cli:\n  TS-D07: many\n",
			wantErr: "internal/cli",
		},
		{
			name:    "AbsolutePackage",
			rows:    "/internal/cli:\n  TS-D07: 1\n",
			wantErr: "/internal/cli",
		},
		{
			name:    "NotYAML",
			rows:    "internal/cli: [\n",
			wantErr: "tiger.budget.yaml",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := budgetTree(t, test.rows)
			for _, command := range []string{"check", "budget"} {
				got := run(t, command, "-C", dir, "./internal/cli")
				assert.Equal(t, cli.ExitOperational, got.code)
				assert.Empty(t, got.stdout)
				assert.Contains(t, got.stderr, "tiger.budget.yaml")
				assert.Contains(t, got.stderr, test.wantErr)
			}
			assert.Equal(t, test.rows, readFile(t, filepath.Join(dir, budgetFile)))
		})
	}
}

// TestBudgetRejectsUnknownFlag covers the subcommand's flag surface.
//
// Goal: a flag tiger budget does not define exits 2.
func TestBudgetRejectsUnknownFlag(t *testing.T) {
	got := run(t, "budget", "-C", filepath.Join("testdata", "fixtures", "budget", "tree"),
		"--show-facts")
	assert.Equal(t, cli.ExitOperational, got.code)
	assert.Contains(t, got.stderr, "show-facts")
}
