package assert_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestModuleHasNoRequirements fails if assert/go.mod gains a requirement. A
// project that requires this package must not inherit any other module.
func TestModuleHasNoRequirements(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "all").Output()
	if err != nil {
		t.Fatal(err)
	}
	const want = "github.com/kapetan-io/tiger/assert"
	if got := strings.TrimSpace(string(out)); got != want {
		t.Fatalf("go list -m all in the assert directory = %q, want only %q", got, want)
	}
}
