// Package dotfs has an empty entry and reaches the file system through a dot
// import and through method forms.
package dotfs

import (
	"html/template"
	. "path/filepath"
)

func Reads() (string, error) {
	return Abs(".") // want `can't use path/filepath\.Abs `
}

func Methods() {
	parse := (*template.Template).ParseGlob // want `can't use html/template\.Template\.ParseGlob `
	_ = parse
	method := template.New("a").ParseFiles // want `can't use html/template\.Template\.ParseFiles `
	_ = method
}
