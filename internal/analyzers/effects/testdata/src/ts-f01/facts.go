// Unpinned exported functions carry no contract, but their effect set is
// still computed and printed as a reported fact — TS-F01-facts, in exact
// pin syntax, on request.
package ts01

import "os"

// Env reads an environment variable — the reported fact should read
// io(env), the stdlib table's entry for os.Getenv.
//
// want +1 `TS-F01: computed effects for Env — //tiger:effects io\(env\)` Env:`io\(env\)`
func Env() string {
	return os.Getenv("HOME")
}

// Silent does nothing effectful — the reported fact should read none.
//
// want +1 `TS-F01: computed effects for Silent — //tiger:effects none` Silent:`none`
func Silent() int {
	return 7
}
