package stdlibio_test

import (
	"go/types"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"

	"github.com/kapetan-io/tiger/internal/analyzers/ioinloop/stdlibio"
)

// listedPackages are the packages whose whole exported API carries an
// Effect. The syscall package is left out because its API differs per
// platform.
var listedPackages = []string{"os", "net", "net/http", "database/sql"}

// A package test: a Go release adding a call is unreachable from the
// analyzer's corpus.
//
// TestEveryExportedCallIsListed fails when the Go toolchain running the tests
// (the version in go.mod, in CI) has an exported function or method in os,
// net, net/http or database/sql that stdlibio does not list. Adopters never
// run this test, so a new Go release fails only tiger's own CI.
//
// Goal: every exported function, and every exported method in the method set
// of an exported type (value and pointer, promoted methods included), has an
// Effect other than Unlisted. The failure names each new call so the fix is
// one line in effects.go.
func TestEveryExportedCallIsListed(t *testing.T) {
	var unlisted []string
	for _, name := range exportedCalls(t, listedPackages...) {
		if stdlibio.EffectOf(name) == stdlibio.Unlisted {
			unlisted = append(unlisted, name)
		}
	}
	assert.Empty(t, unlisted, "add each call to effects.go as IO, NoIO or Unproven")
}

// TestEveryListedCallExists fails on an entry that names nothing, such as a
// typo or a call a Go release removed, so the list never carries dead names.
//
// Goal: every listed name outside syscall is an exported call of the four
// packages, and every syscall name exists on this platform. Windows lacks
// several of the listed Unix system calls, so the syscall half runs on Unix
// only.
func TestEveryListedCallExists(t *testing.T) {
	known := exportedCalls(t, listedPackages...)
	var stale []string
	for _, name := range stdlibio.Listed() {
		if strings.HasPrefix(name, "syscall.") {
			continue
		}
		if _, found := slices.BinarySearch(known, name); !found {
			stale = append(stale, name)
		}
	}
	assert.Empty(t, stale)

	if runtime.GOOS == "windows" {
		return
	}
	known = exportedCalls(t, "syscall")
	stale = nil
	for _, name := range stdlibio.Listed() {
		if !strings.HasPrefix(name, "syscall.") {
			continue
		}
		if _, found := slices.BinarySearch(known, name); !found {
			stale = append(stale, name)
		}
	}
	assert.Empty(t, stale)
}

// exportedCalls returns the sorted full names of every exported function and
// exported method declared in the named packages, the same names
// types.Func.FullName gives the analyzer.
func exportedCalls(t *testing.T, paths ...string) []string {
	t.Helper()
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedTypes,
	}, paths...)
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, pkg := range pkgs {
		require.Empty(t, pkg.Errors)
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			switch obj := scope.Lookup(name).(type) {
			case *types.Func:
				if obj.Exported() {
					seen[obj.FullName()] = true
				}
			case *types.TypeName:
				if !obj.Exported() {
					continue
				}
				for _, typ := range []types.Type{obj.Type(), types.NewPointer(obj.Type())} {
					methods := types.NewMethodSet(typ)
					for i := range methods.Len() {
						fn, ok := methods.At(i).Obj().(*types.Func)
						owned := ok && fn.Pkg() != nil && fn.Pkg().Path() == pkg.PkgPath
						if owned && fn.Exported() {
							seen[fn.FullName()] = true
						}
					}
				}
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
