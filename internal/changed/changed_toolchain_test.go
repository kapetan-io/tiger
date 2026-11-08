package changed_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/command"
)

// reachKeyIn is the reach file key for the toolchain Go selects in dir, which
// differs from the test's own toolchain when dir's go.mod has a toolchain line.
func reachKeyIn(t *testing.T, dir string) string {
	t.Helper()
	names := []string{
		"GOVERSION", "GOOS", "GOARCH", "CGO_ENABLED", "GOEXPERIMENT", "GOFLAGS", "GOFIPS140",
		"GOAMD64", "GOARM", "GOARM64", "GO386", "GOMIPS", "GOMIPS64", "GOPPC64", "GORISCV64",
		"GOWASM",
	}
	cmd := exec.Command("go", append([]string{"env", "-json"}, names...)...)
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)
	var env map[string]string
	require.NoError(t, json.Unmarshal(out, &env))
	binary, err := os.Executable()
	require.NoError(t, err)
	data, err := os.ReadFile(binary)
	require.NoError(t, err)
	sum := sha256.Sum256(data)
	parts := []string{"tiger-reachfile/2", "tiger=" + hex.EncodeToString(sum[:8])}
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s=%q", name, env[name]))
	}
	return strings.Join(parts, " ")
}

const baseToolchain = "go1.25.14"

func toolchainModule(goLine string) string {
	return "module example.com/shop\n\ngo " + goLine + "\n"
}

// toolchainCheckout is a repository whose base commit builds with baseToolchain
// and whose head commit, by its go line, makes Go switch to the newer go1.26.10.
// Go downloads a toolchain it does not have, so the first run needs network access.
func toolchainCheckout(t *testing.T) checkout {
	t.Helper()
	t.Setenv("GOTOOLCHAIN", baseToolchain+"+auto")
	c := newCheckout(t)
	c.write("go.mod", toolchainModule("1.25.0"))
	c.write("shop.go", "package shop\n\nfunc Save() {}\n")
	c.commit("base")
	c.git("checkout", "-q", "-b", "feature")
	c.write("go.mod", toolchainModule("1.26.10"))
	c.write("shop.go", "package shop\n\nimport \"os\"\n\nfunc Save() { _ = os.Getpid() }\n")
	c.commit("head")
	return c
}

// TestChangedKeepsAReachFilePerToolchain covers a base that builds with
// another toolchain than the head.
//
// Goal: when the base and head go.mod files select different toolchains, the
// base tree is read from the reach file made for its own toolchain, not the
// head's. Each toolchain leaves one whole entry in Go's build cache.
func TestChangedKeepsAReachFilePerToolchain(t *testing.T) {
	c := toolchainCheckout(t)
	headKey := reachKeyIn(t, c.dir)
	c.git("checkout", "-q", "main")
	baseKey := reachKeyIn(t, c.dir)
	c.git("checkout", "-q", "feature")
	require.NotEqual(t, headKey, baseKey)
	require.NoError(t, os.RemoveAll(reachEntry(t, headKey)))
	require.NoError(t, os.RemoveAll(reachEntry(t, baseKey)))

	got := c.changed()

	require.Equal(t, command.ExitClean, got.code)
	assert.True(t, whole(t, headKey))
	assert.FileExists(t, reachEntry(t, baseKey))
	assert.True(t, whole(t, baseKey))
	assert.NotEqual(t, filepath.Base(reachEntry(t, headKey)), filepath.Base(reachEntry(t, baseKey)))
}

// TestChangedSaysWhyTheBaseToolchainFails covers a base whose go.mod needs a
// toolchain Go cannot provide.
//
// Goal: the failure carries Go's own reason, not only an exit status.
func TestChangedSaysWhyTheBaseToolchainFails(t *testing.T) {
	t.Setenv("GOTOOLCHAIN", "local")
	c := newCheckout(t)
	c.write("go.mod", toolchainModule("1.99.0"))
	c.write("shop.go", "package shop\n")
	c.commit("base")
	c.git("checkout", "-q", "-b", "feature")
	c.write("go.mod", toolchainModule("1.21.0"))
	c.commit("head")

	got := c.changed()

	assert.Equal(t, command.ExitOperational, got.code)
	assert.Contains(t, got.stderr, "go.mod requires go >= 1.99.0")
}
