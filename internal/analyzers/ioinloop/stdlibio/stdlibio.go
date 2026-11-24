// Package stdlibio names which standard library calls do IO.
//
// Every exported function and method in os, net, net/http and database/sql
// carries one Effect, keyed by its types.Func full name as an analyzer sees
// it ("os.ReadFile", "(*net/http.Client).Do", "(net.Conn).Write"). A method
// promoted from an unexported embedded type keeps that type's name
// ("(*net.conn).Read" for a *net.TCPConn). A short list of raw system calls
// in syscall is IO too; syscall itself is not listed in full because its API
// differs per platform.
//
// IO means the call reaches a disk, a network peer, a database or a process
// table on every call. NoIO means it works in memory or only reads process
// state, such as net.SplitHostPort, os.Getenv and (*os.File).SetDeadline.
// Unproven means the effect depends on a value the call site cannot see: an
// interface method whose implementer may be in memory (net.Pipe,
// httptest.ResponseRecorder), or a driver-dependent database/sql call. An
// Unproven call is silent, the same as NoIO, because a blocking rule must
// not guess.
//
// A Go release that adds an exported function or method to one of the four
// packages fails this package's completeness test until the new call is
// listed here.
package stdlibio

import (
	"maps"
	"slices"
)

type Effect uint8

const (
	Unlisted Effect = iota
	IO
	NoIO
	Unproven
)

func EffectOf(fullName string) Effect {
	return effects[fullName]
}

func Listed() []string {
	return slices.Sorted(maps.Keys(effects))
}
