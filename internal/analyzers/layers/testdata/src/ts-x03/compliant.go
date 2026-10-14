package layered

// Each import is listed or is standard library outside the impure subtrees: os
// and its subtree are named, ./store is named exactly, and example.com/lib
// allows the module and every package below it.
import (
	_ "crypto/sha256"
	_ "os"
	_ "os/exec"
	_ "strings"
	_ "unsafe"

	_ "example.com/lib"
	_ "example.com/lib/sub"

	_ "ts-x03/store"
)
