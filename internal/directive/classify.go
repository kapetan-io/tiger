package directive

import (
	"go/ast"
	"strings"
)

type Class int

const (
	Prose Class = iota
	Header
	GoDirective
	Tiger
	Nolint
)

var goDirectivePrefixes = []string{"//go:", "//line ", "//export ", "//extern ", "/*line "}

func Classify(file *ast.File, comment *ast.Comment) Class {
	switch {
	case is(comment.Text):
		return Tiger
	case honoredNolint(comment.Text):
		return Nolint
	case hasAnyPrefix(comment.Text, goDirectivePrefixes):
		return GoDirective
	case comment.Pos() < file.Package:
		return Header
	}
	return Prose
}

func Spelled(text string) bool {
	rest, found := strings.CutPrefix(text, "//nolint")
	return found && (rest == "" || rest[0] == ' ' || rest[0] == ':')
}

func honoredNolint(text string) bool {
	body, found := strings.CutPrefix(text, "//")
	if !found {
		return false
	}
	rest, found := strings.CutPrefix(strings.TrimLeft(body, "/ "), "nolint")
	return found && (rest == "" || rest[0] == ' ' || rest[0] == ':')
}

func hasAnyPrefix(text string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}
