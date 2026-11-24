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

// listedPackages are the packages whose whole exported API has a Class. The
// syscall package is left out because its API differs per platform.
var listedPackages = []string{"os", "net", "net/http", "database/sql"}

// These tests call stdlibio directly rather than through the analyzer,
// because no corpus file can contain a call that a future Go release adds.
//
// TestEveryExportedCallIsListed fails when the Go release in go.mod has an
// exported function or method in os, net, net/http or database/sql that
// stdlibio does not list. Adopters never run this test. A Go release newer
// than go.mod's fails it too, once that release adds a call, so CI's job on
// the next Go release leaves this test out with -skip.
//
// Goal: every exported function, and every exported method in the method set
// of an exported type (value and pointer, promoted methods included), has an
// Class other than Unlisted. The failure names each new call so the fix is
// one line in calls.go.
func TestEveryExportedCallIsListed(t *testing.T) {
	var unlisted []string
	for _, name := range exportedCalls(t, listedPackages...) {
		if stdlibio.Classify(name) == stdlibio.Unlisted {
			unlisted = append(unlisted, name)
		}
	}
	assert.Empty(t, unlisted, "classify each call in calls.go as IO, NoIO or Unproven, using the "+
		"definitions in stdlibio.go's package doc, in its class's sorted group; "+
		"use Unproven when unsure")
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
	for _, name := range stdlibio.Names() {
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
	for _, name := range stdlibio.Names() {
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
