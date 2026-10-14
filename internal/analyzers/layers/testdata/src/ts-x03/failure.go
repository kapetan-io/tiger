package layered

// Each import sits outside the root entry: net is not named, math/rand/v2 is
// under math/rand, which is not named, ./domain is not named, example.com/lib
// does not allow example.com/lib-c, whose path only starts the same way, and
// dotless/lib is a third-party module though its path has no dot.
import (
	_ "math/rand/v2" // want `the "\." entry in tiger\.layers\.yaml doesn't list math/rand, so this package can't import math/rand/v2 — add "math/rand" to it`
	_ "net/http"     // want `doesn't list net, so this package can't import net/http — add "net" to it, or pass it in through an interface`

	_ "dotless/lib"       // want `the "\." entry in tiger\.layers\.yaml doesn't list dotless/lib — add it, or its module path, to the entry`
	_ "example.com/lib-c" // want `the "\." entry in tiger\.layers\.yaml doesn't list example\.com/lib-c — add it, or its module path, to the entry`

	_ "ts-x03/domain" // want `the "\." entry in tiger\.layers\.yaml doesn't list \./domain — add it, or pass what you need in through an interface`
)
