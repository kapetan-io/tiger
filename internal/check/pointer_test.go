package check_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/check"
	"github.com/kapetan-io/tiger/internal/command"
	"github.com/kapetan-io/tiger/internal/rules"
)

var findingLine = regexp.MustCompile(`^\S+: (TS-[A-Z]+[0-9]+): (.*)$`)

const childEnv = "TIGER_CHECK_TEST_CHILD"

// TestMain lets runIsolated re-execute this binary as a plain `tiger check`.
func TestMain(m *testing.M) {
	if os.Getenv(childEnv) != "" {
		os.Exit(check.Run(os.Args[1:], command.Streams{Stdout: os.Stdout, Stderr: os.Stderr}))
	}
	os.Exit(m.Run())
}

// runIsolated runs `tiger check` with args in its own process so runs can
// overlap. A run installs process-global config that two runs in one process
// race on, which is why every other test runs `check` in-process, one at a time.
func runIsolated(t *testing.T, args ...string) outcome {
	t.Helper()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), childEnv+"=1")
	cmd.Stdout, cmd.Stderr = stdout, stderr
	code := 0
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Errorf("start child: %v", err)
		}
		code = cmd.ProcessState.ExitCode()
	}
	return outcome{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// TestCheckFindingsEndWithTheirPointerWithinTheCap reads every finding the
// fixtures produce.
//
// Goal: every printed finding ends with the pointer the registry defines for
// its rule (one pattern, several, or none), and the body of a pointed
// finding, pointer included, stays within 240 characters. The fixtures must exercise a one-pattern
// pointer, a several-pattern pointer, and a rule with no pattern.
func TestCheckFindingsEndWithTheirPointerWithinTheCap(t *testing.T) {
	const maxBodyRunes = 240
	possible := map[string]map[string]bool{}
	for _, rule := range rules.CustomRules() {
		if possible[rule.RuleID] == nil {
			possible[rule.RuleID] = map[string]bool{}
		}
		possible[rule.RuleID][rules.Pointer(rule.Category)] = true
	}
	entries, err := os.ReadDir("testdata/fixtures")
	require.NoError(t, err)
	fixtures := []string{}
	for _, entry := range entries {
		fixtures = append(fixtures, "testdata/fixtures/"+entry.Name())
	}
	fixtures = append(fixtures, "testdata/fixtures/invariants/unasserted",
		"testdata/fixtures/invariants/unviolated", "testdata/fixtures/invariants/singlesite")

	outcomes := make([]outcome, len(fixtures))
	var group sync.WaitGroup
	for i, dir := range fixtures {
		group.Go(func() { outcomes[i] = runIsolated(t, "-C", dir, "./...") })
	}
	group.Wait()

	forms := map[string]bool{}
	for i, dir := range fixtures {
		got := outcomes[i]
		// The broken fixture exits ExitOperational on purpose; a crash is not that.
		require.NotEqual(t, -1, got.code)
		require.NotContains(t, got.stderr, "panic:")
		if got.code != command.ExitFindings {
			continue
		}
		for _, line := range strings.Split(got.stdout, "\n") {
			match := findingLine.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			code, body := match[1], match[2]
			pointer := ""
			if at := strings.Index(body, " — see `tiger pattern"); at >= 0 {
				pointer = body[at:]
			}
			if _, registered := possible[code]; registered {
				assert.True(t, possible[code][pointer], "%s: %s", dir, line)
			}
			if pointer != "" {
				runes := utf8.RuneCountInString(body)
				assert.LessOrEqual(t, runes, maxBodyRunes, "%s: %s", dir, line)
			}
			switch {
			case pointer == "":
				forms["none"] = true
			case strings.Contains(pointer, "--rule"):
				forms["several"] = true
			default:
				forms["one"] = true
			}
		}
	}
	assert.Equal(t, map[string]bool{"none": true, "several": true, "one": true}, forms)
}

// TestCheckFindingEndsWithThePatternPointer covers the driver's suffix.
//
// Goal: a TS-C02 finding ends with the list of patterns that back it, and a
// finding from a rule no pattern backs is printed unchanged.
func TestCheckFindingEndsWithThePatternPointer(t *testing.T) {
	got := run(t, "check", "-C", "testdata/fixtures/pointed", "./...")
	assert.Equal(t, command.ExitFindings, got.code)
	lines := strings.Split(got.stdout, "\n")
	require.Greater(t, len(lines), 1)
	assert.Contains(t, lines[0], ": TS-C02: ")
	assert.True(t, strings.HasSuffix(lines[0], " — see `tiger pattern --rule TS-C02`"))

	plain := run(t, "check", "-C", "testdata/fixtures/findings", "./...")
	assert.NotContains(t, plain.stdout, "tiger pattern")
}
