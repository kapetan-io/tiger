package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var ciJobs = []string{"test", "patterns", "lint", "dogfood", "determinism", "plugin-smoke"}

type workflowFile struct {
	Jobs map[string]struct {
		Steps []struct {
			ID  string `yaml:"id"`
			Run string `yaml:"run"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

func decideScript(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../../.github/workflows/ci.yml")
	require.NoError(t, err)
	var workflow workflowFile
	require.NoError(t, yaml.Unmarshal(raw, &workflow))
	for _, step := range workflow.Jobs["changes"].Steps {
		if step.ID == "which" {
			return step.Run
		}
	}
	require.FailNow(t, "changes job has no step with id which")
	return ""
}

// baseCheckout holds the base commit of a pull request, with the file
// written first among its contents.
func baseCheckout(t *testing.T, first repoPath) checkout {
	t.Helper()
	c := newCheckout(t)
	c.write("keep.txt", "keep\n")
	c.write(first, "base\n")
	c.commit("base")
	return c
}

// pullRequestEditing is a pull request that writes one file on top of the base.
func pullRequestEditing(t *testing.T, name repoPath) checkout {
	t.Helper()
	c := baseCheckout(t, "keep2.txt")
	c.write(name, "changed\n")
	c.commit("head")
	return c
}

// decide runs the changes job's script in the checkout, as the event, with
// the commit before HEAD as the pull request's base, and returns the per-job
// outputs it wrote.
func (c checkout) decide(event string) map[string]string {
	c.t.Helper()
	output := filepath.Join(c.t.TempDir(), "output")
	cmd := exec.Command("bash", "-e", "-c", decideScript(c.t))
	cmd.Dir = c.dir
	cmd.Env = append(cleanGitEnviron(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"EVENT="+event, "BASE="+c.git("rev-parse", "HEAD~1"), "GITHUB_OUTPUT="+output)
	out, err := cmd.CombinedOutput()
	require.NoError(c.t, err, string(out))

	raw, err := os.ReadFile(output)
	require.NoError(c.t, err)
	got := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		key, value, _ := strings.Cut(line, "=")
		got[key] = value
	}
	return got
}

func runsOnly(jobs ...string) map[string]string {
	want := map[string]string{}
	for _, job := range ciJobs {
		want[job] = "false"
	}
	for _, job := range jobs {
		want[job] = "true"
	}
	return want
}

// TestCIChangesJobSkipsJobsAPullRequestCannotAffect runs the changes job's
// script against temp repositories holding one pull request each.
//
// Goal: each file class turns on exactly the jobs that read it, so a pull
// request touching nothing a job reads skips that job.
func TestCIChangesJobSkipsJobsAPullRequestCannotAffect(t *testing.T) {
	for _, test := range []struct {
		name string
		file repoPath
		want map[string]string
	}{
		{name: "goSource", file: "internal/x/x.go", want: runsOnly(ciJobs...)},
		{name: "goMod", file: "go.mod", want: runsOnly(ciJobs...)},
		{name: "goSum", file: "go.sum", want: runsOnly(ciJobs...)},
		{name: "patternsTree", file: "patterns/ledger/README.md", want: runsOnly(ciJobs...)},
		{name: "workflow", file: ".github/workflows/ci.yml", want: runsOnly(ciJobs...)},
		{name: "docs", file: "docs/adr/0001.md", want: runsOnly("test")},
		{name: "docsWithSpaces", file: "docs/Assert Adoption Playbook.md", want: runsOnly("test")},
		{name: "config", file: "config/golangci.yml", want: runsOnly("test")},
		{name: "configLayers", file: "config/tiger.layers.yaml", want: runsOnly("test")},
		{
			name: "fixture", file: "internal/cli/testdata/fixtures/x/data.txt",
			want: runsOnly("test", "determinism", "plugin-smoke"),
		},
		{
			name: "rootFixture", file: "testdata/plugin-layers/tiger.layers.yaml",
			want: runsOnly("test", "determinism", "plugin-smoke"),
		},
		{
			name: "tigerBudget", file: "tiger.budget.yaml",
			want: runsOnly("test", "patterns", "dogfood", "determinism", "plugin-smoke"),
		},
		{
			name: "golangciConfig", file: ".golangci.yml",
			want: runsOnly("test", "lint", "plugin-smoke"),
		},
		{
			name: "customGclConfig", file: ".custom-gcl.yml",
			want: runsOnly("test", "lint", "plugin-smoke"),
		},
		{name: "reportAwk", file: ".github/report.awk", want: runsOnly("dogfood")},
		{
			name: "reportFixture", file: ".github/testdata/report.txt",
			want: runsOnly("test", "dogfood", "determinism", "plugin-smoke"),
		},
		{name: "readme", file: "README.md", want: runsOnly()},
		{name: "claudeNotes", file: "CLAUDE.md", want: runsOnly()},
		{name: "otherWorkflow", file: ".github/workflows/release.yml", want: runsOnly()},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, pullRequestEditing(t, test.file).decide("pull_request"))
		})
	}
}

// TestCIChangesJobRunsEveryJobWhenAFileClassTheTestsReadChanges guards the
// test job's inputs.
//
// Goal: every non-Go file class the repository's tests read triggers test.
func TestCIChangesJobRunsEveryJobWhenAFileClassTheTestsReadChanges(t *testing.T) {
	for _, file := range []repoPath{
		"docs/Assert Adoption Playbook.md", "docs/CONTEXT.md", "config/tiger.layers.yaml",
		"internal/cli/testdata/changed/x/base/a.txt", "testdata/plugin-smoke/findings.txt",
		"tiger.budget.yaml", ".golangci.yml", ".custom-gcl.yml", "patterns/ledger/README.md",
	} {
		t.Run(string(file), func(t *testing.T) {
			got := pullRequestEditing(t, file).decide("pull_request")
			assert.Equal(t, "true", got["test"])
		})
	}
}

// TestCIChangesJobCombinesFiles covers a pull request touching several classes.
//
// Goal: the jobs that run are the union of what each changed file triggers.
func TestCIChangesJobCombinesFiles(t *testing.T) {
	c := baseCheckout(t, "keep2.txt")
	c.write("docs/a.md", "x")
	c.write(".golangci.yml", "x")
	c.write("README.md", "x")
	c.commit("head")
	assert.Equal(t, runsOnly("test", "lint", "plugin-smoke"), c.decide("pull_request"))
}

// TestCIChangesJobSkipsEverythingWhenNothingChanged covers an empty diff.
//
// Goal: a pull request with no file changes runs no job.
func TestCIChangesJobSkipsEverythingWhenNothingChanged(t *testing.T) {
	c := baseCheckout(t, "keep2.txt")
	c.git("commit", "-q", "--allow-empty", "-m", "head")
	assert.Equal(t, runsOnly(), c.decide("pull_request"))
}

// TestCIChangesJobRunsEveryJobOnPush covers pushes to main.
//
// Goal: a push runs every job in full, even when it changes only prose.
func TestCIChangesJobRunsEveryJobOnPush(t *testing.T) {
	assert.Equal(t, runsOnly(ciJobs...), pullRequestEditing(t, "README.md").decide("push"))
}

// TestCIChangesJobSeesDeletedAndRenamedGoFiles covers removals.
//
// Goal: deleting a Go file, or renaming it to a non-Go name, still runs every job.
func TestCIChangesJobSeesDeletedAndRenamedGoFiles(t *testing.T) {
	deleted := baseCheckout(t, "internal/x/x.go")
	deleted.git("rm", "-q", "internal/x/x.go")
	deleted.commit("head")
	assert.Equal(t, runsOnly(ciJobs...), deleted.decide("pull_request"))

	// Identical content makes git pair the two paths as a rename.
	renamed := baseCheckout(t, "internal/x/x.go")
	renamed.git("mv", "internal/x/x.go", "notes.txt")
	renamed.commit("head")
	assert.Equal(t, runsOnly(ciJobs...), renamed.decide("pull_request"))
}

// TestCIChangesJobSeesNonASCIIPaths covers file names git quotes.
//
// Goal: a docs file with a non-ASCII name still triggers test.
func TestCIChangesJobSeesNonASCIIPaths(t *testing.T) {
	assert.Equal(t, runsOnly("test"), pullRequestEditing(t, "docs/café.md").decide("pull_request"))
}

// TestCIJobsHonorTheChangesDecision reads the workflow's job conditions.
//
// Goal: every job waits on changes and runs when changes failed or its own
// output is true, and the test job restores the Go cache only on pull requests.
func TestCIJobsHonorTheChangesDecision(t *testing.T) {
	raw, err := os.ReadFile("../../.github/workflows/ci.yml")
	require.NoError(t, err)
	var workflow struct {
		Jobs map[string]struct {
			Needs string `yaml:"needs"`
			If    string `yaml:"if"`
		} `yaml:"jobs"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &workflow))
	for _, job := range ciJobs {
		t.Run(job, func(t *testing.T) {
			assert.Equal(t, "changes", workflow.Jobs[job].Needs)
			assert.Equal(t, "${{ !cancelled() && (needs.changes.result != 'success' || "+
				"needs.changes.outputs."+job+" == 'true') }}", workflow.Jobs[job].If)
		})
	}
	var steps struct {
		Jobs struct {
			Test struct {
				Steps []struct {
					Uses string `yaml:"uses"`
					If   string `yaml:"if"`
				} `yaml:"steps"`
			} `yaml:"test"`
		} `yaml:"jobs"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &steps))
	ifs := map[string]string{}
	for _, step := range steps.Jobs.Test.Steps {
		ifs[step.Uses] = step.If
	}
	assert.Equal(t, "github.event_name == 'pull_request'", ifs["actions/cache/restore@v4"])
	assert.Empty(t, ifs["actions/cache/save@v4"])
	assert.Contains(t, ifs, "actions/cache/save@v4")
}
