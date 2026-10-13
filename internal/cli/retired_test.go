package cli_test

import (
	"os"
	"path/filepath"
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
