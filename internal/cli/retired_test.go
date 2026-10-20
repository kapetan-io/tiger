package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/cli"
)

// sourceFile is one file of a throwaway module.
type sourceFile struct {
	name    string
	content string
}

// writeModule lays out a throwaway module so check runs against exactly the
// source a test names.
func writeModule(t *testing.T, files ...sourceFile) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module retired.example/project\n\ngo 1.26\n"), 0o644))
	write := func(file sourceFile) error {
		path := filepath.Join(dir, file.name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, []byte(file.content), 0o644)
	}
	require.NoError(t, write(files[0]))
	if len(files) > 1 {
		require.NoError(t, write(files[1]))
	}
	return dir
}

// TestCheckRetiredRulesStaySilent covers the removal of TS-X01, TS-P01,
// TS-P02, TS-P03, and TS-K03.
//
// Goal: a single-implementation interface, a reflect import, and exported
// closed-dispatch functions without pins exit 0 with no output.
func TestCheckRetiredRulesStaySilent(t *testing.T) {
	dir := writeModule(t,
		sourceFile{"store/store.go", `// Package store holds one interface with one implementation.
package store

import "reflect"

// Reader reads a value.
type Reader interface{ Read() int }

type memory struct{}

// Read returns a constant.
func (memory) Read() int { return 1 }

// New returns the only Reader.
func New() Reader { return memory{} }

// Kind names a value's reflect kind.
func Kind(v any) string { return reflect.TypeOf(v).Kind().String() }
`},
		sourceFile{"shape/shape.go", `// Package shape dispatches over a closed set of types.
package shape

// Shape is a closed set.
type Shape interface{ shape() }

// Circle is a shape.
type Circle struct{}

func (Circle) shape() {}

// Square is a shape.
type Square struct{}

func (Square) shape() {}

// Sides counts sides.
func Sides(s Shape) int {
	switch s.(type) {
	case Circle:
		return 0
	case Square:
		return 4
	}
	return -1
}
`})

	got := run(t, "check", "-C", dir, "./...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestCheckRetiredCommentRulesStaySilent covers the removal of TS-T06 and
// TS-S22.
//
// Goal: a test function with no doc comment and a limit constant whose
// derivation comment is wrong exit 0 with no output.
func TestCheckRetiredCommentRulesStaySilent(t *testing.T) {
	dir := writeModule(t,
		sourceFile{"limits/limits.go", `// Package limits holds a limit with a stale derivation.
package limits

const writeBufferSize = 32768
const headerSizeBytes = 12
const entrySize = 4

// batchMax = (writeBufferSize - headerSizeBytes) / entrySize.
const batchMax = 8188

const _ = uint((writeBufferSize - headerSizeBytes) - batchMax*entrySize)

// Fits reports whether n entries fit in a batch.
func Fits(n int) bool { return n <= batchMax && n*entrySize <= writeBufferSize-headerSizeBytes }
`},
		sourceFile{"limits/limits_test.go", `package limits_test

import (
	"testing"

	"retired.example/project/limits"
)

func TestFitsOne(t *testing.T) {
	if !limits.Fits(1) {
		t.Fatal("one entry must fit")
	}
}
`})

	got := run(t, "check", "-C", dir, "./...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestCheckCommentedDiscardFailsE02 covers the removal of TS-E02's comment
// exemption.
//
// Goal: an error discarded with _ under a trailing comment, a comment on the
// line above, and a bare //nolint is reported, while the same discard in a
// defer call is not.
func TestCheckCommentedDiscardFailsE02(t *testing.T) {
	dir := writeModule(t, sourceFile{"store/store.go", `// Package store closes files.
package store

import "os"

// Trailing discards under a trailing comment.
func Trailing(f *os.File) {
	_ = f.Close() // reason
}

// Above discards under a comment on the line above.
func Above(f *os.File) {
	// reason
	_ = f.Close()
}

// Nolint discards under a bare nolint.
func Nolint(f *os.File) {
	_ = f.Close() //nolint
}

// Deferred discards inside a defer call.
func Deferred(f *os.File) {
	defer func() { _ = f.Close() }()
}
`})

	got := run(t, "check", "-C", dir, "./...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, 3, strings.Count(got.stdout, "TS-E02: this error is discarded with _"))
	assert.Contains(t, got.stdout, "store.go:8:")
	assert.Contains(t, got.stdout, "store.go:14:")
	assert.Contains(t, got.stdout, "store.go:19:")
}

// TestCheckRetiredDirectiveVerbsFailL09 covers the removal of the restrict,
// hot, wire, and owner directive verbs.
//
// Goal: each verb is reported as an unknown directive by TS-L09 and the run
// exits 1.
func TestCheckRetiredDirectiveVerbsFailL09(t *testing.T) {
	for _, test := range []struct{ name, verb, line string }{
		{"Restrict", "restrict", "restrict no-reflect"},
		{"Hot", "hot", "hot"},
		{"Wire", "wire", "wire"},
		{"Owner", "owner", "owner"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := writeModule(t, sourceFile{
				name: "pkg/pkg.go",
				content: "// Package pkg carries a retired directive.\npackage pkg\n\n" +
					"//tiger:" + test.line + "\n// Value is a number.\nvar Value = 1\n",
			})

			got := run(t, "check", "-C", dir, "./...")
			assert.Equal(t, cli.ExitFindings, got.code)
			assert.Empty(t, got.stderr)
			assert.Contains(t, got.stdout,
				"TS-L09: //tiger:"+test.verb+" is not a directive tiger recognizes")
		})
	}
}
