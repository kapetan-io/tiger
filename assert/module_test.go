package assert_test

import (
	"os/exec"
	"strings"
	"testing"
)

// ModuleHasNoRequirements covers the shipping surface: a project that requires
// this package must not inherit a requirement on any other module.
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
