package ts01

import "os"

// launder calls fn indirectly through a variable; StaticCalls resolves
// only statically known callees, so whatever fn does never reaches the
// composed effect set — known-miss: an effect reached only through a
// function value is invisible to static-call composition. Everything
// below is unexported, so no reported fact fires either way.
func launder(fn func()) {
	fn()
}

func drive() {
	launder(readEnv)
}

func readEnv() {
	os.Getenv("HOME")
}
