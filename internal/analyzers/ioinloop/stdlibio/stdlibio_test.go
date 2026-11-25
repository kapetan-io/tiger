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

// How calls.go classifies a call. Every exported function and method in os,
// net, net/http and database/sql has one Class, keyed by its types.Func full
// name as an analyzer sees it ("os.ReadFile", "(*net/http.Client).Do",
// "(net.Conn).Write"). A method promoted from an unexported embedded type keeps
// that type's name ("(*net.conn).Read" for a *net.TCPConn). A short list of
// raw system calls in syscall is IO too. The rest of syscall is unlisted,
// because its API differs per platform.
//
// IO means the call makes a system call that touches a file, a socket, a
// database connection or another process every time it runs, such as
// os.ReadFile, os.Getwd and (*sql.DB).Query. NoIO means it computes in memory
// or returns state the process already holds, such as net.SplitHostPort,
// os.Getenv, os.Getpid and (*os.File).SetDeadline. Unproven means the call
// site cannot tell which: an interface method whose implementer may be in
// memory (net.Pipe, httptest.ResponseRecorder), a database/sql call whose cost
// depends on the driver, a call that skips the lookup for some arguments
// (net.ResolveTCPAddr on an IP literal, net.LookupPort on a port number), a
// call that touches the system only on its first use (http.ParseTime loading
// the local time zone), or a call that signals or waits on another process
// ((*os.Process).Kill, Signal, Wait, Release and os.FindProcess). Those return
// early on a released or finished process, and a loop that waits on or
// signals each of its children has no batched form to move to. Unproven and
// Unlisted calls are silent, the same as NoIO, because a blocking rule must
// not guess. When a new call sits between IO and Unproven, choose Unproven.
//
// Two calls are IO although the definitions make them Unproven, because
// TS-M10's corpus requires the finding. (net.Conn).Write is the one interface
// method listed as IO, although net.Pipe implements it, because a net.Conn
// outside tests is almost always a socket. (*sql.Rows).Close depends on the
// driver, but it usually releases a connection the query still holds.
//
// calls.go lists IO calls first, then Unproven, then NoIO, each group sorted
// by name. Unlike the effect table ADR-0026 removed, this list classifies
// every exported call in its four packages, enforced by these tests, and
// decides one rule's verdict on direct calls only, never effects along a call
// chain. A repo adds a third-party client, such as a database driver, by
// package through ioinloop.packages.

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
		"use Unproven when unsure. The test lists the standard library of the go running it, so a "+
		"toolchain newer than go.mod's adds calls; run it with go.mod's Go, or classify them when "+
		"go.mod moves to that release")
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
