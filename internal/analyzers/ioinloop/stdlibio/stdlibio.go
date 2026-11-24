// Package stdlibio is TS-M10's IO list: which standard library calls do IO.
//
// Every exported function and method in os, net, net/http and database/sql
// has one Class, keyed by its types.Func full name as an analyzer sees it
// ("os.ReadFile", "(*net/http.Client).Do", "(net.Conn).Write"). A method
// promoted from an unexported embedded type keeps that type's name
// ("(*net.conn).Read" for a *net.TCPConn). A short list of raw system calls
// in syscall is IO too. The rest of syscall is unlisted, because its API
// differs per platform.
//
// IO means the call makes a system call that touches a file, a socket, a
// database connection or another process every time it runs, such as
// os.ReadFile, os.Getwd and (*sql.DB).Query. NoIO means it computes in
// memory or returns state the process already holds, such as
// net.SplitHostPort, os.Getenv, os.Getpid and (*os.File).SetDeadline.
// Unproven means the call site cannot tell which: an interface method whose
// implementer may be in memory (net.Pipe, httptest.ResponseRecorder), a
// database/sql call whose cost depends on the driver, a call that skips the
// lookup for some arguments (net.ResolveTCPAddr on an IP literal,
// net.LookupPort on a port number), a call that touches the system only on
// its first use (http.ParseTime loading the local time zone), or a call that
// signals or waits on another process
// ((*os.Process).Kill, Signal, Wait, Release and os.FindProcess). Unproven
// and Unlisted calls are silent, the same as NoIO, because a blocking rule
// must not guess. When a new call sits between IO and Unproven, choose
// Unproven.
//
// Two calls are IO although the definitions make them Unproven, because
// TS-M10's corpus requires the finding. (net.Conn).Write is the one interface
// method listed as IO, although net.Pipe implements it, because a net.Conn
// outside tests is almost always a socket. (*sql.Rows).Close depends on the
// driver, but it usually releases a connection the query still holds.
//
// calls.go lists IO calls first, then Unproven, then NoIO, each group sorted
// by name. When go.mod moves to a Go release that adds an exported function
// or method to one of the four packages, TestEveryExportedCallIsListed fails
// and names the call, and the fix is one line in its group.
//
// Unlike the effect table ADR-0026 removed, this list classifies every
// exported call in its four packages, enforced by a test, and decides one
// rule's verdict on direct calls only, never effects along a call chain. A
// repo adds a third-party client, such as a database driver, by package
// through ioinloop.packages.
package stdlibio

import (
	"maps"
	"slices"
)

type Class uint8

const (
	Unlisted Class = iota
	IO
	NoIO
	Unproven
)

func Classify(fullName string) Class {
	return classes[fullName]
}

func Names() []string {
	return slices.Sorted(maps.Keys(classes))
}
