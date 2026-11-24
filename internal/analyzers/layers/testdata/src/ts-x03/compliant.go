package layered

// Each import is listed or is standard library outside the impure subtrees: os
// and its subtree are named, ./store is named exactly, and example.com/lib
// allows the module and every package below it.
import (
	_ "crypto/sha256"
	"html/template"
	_ "os"
	_ "os/exec"
	"path/filepath"
	_ "strings"
	text "text/template"
	_ "unsafe"

	_ "example.com/lib"
	_ "example.com/lib/sub"

	_ "ts-x03/store"
)

// The root entry names os, so the functions that read outside the process are
// allowed.
func Reads() {
	_, _ = filepath.Glob("*")
	_ = filepath.Walk(".", nil)
	_ = filepath.WalkDir(".", nil)
	_, _ = filepath.EvalSymlinks(".")
	_, _ = template.ParseFiles("a")
	_, _ = text.ParseGlob("*")
}
