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

// Classify is the one answer to what a comment means to tiger. Text decides
// every directive class first, so a directive means the same thing above the
// package clause as below it. A comment above the package clause that is no
// directive is Header, and one below it is Prose.
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

// Spelled reports whether a Nolint comment is spelled //nolint, the one form
// that survives the comment ban.
func Spelled(text string) bool {
	rest, found := strings.CutPrefix(text, "//nolint")
	return found && (rest == "" || rest[0] == ' ' || rest[0] == ':')
}

// honoredNolint matches the way golangci-lint 2.11 honors //nolint: slashes
// and spaces after the opening slashes are skipped, then nolint must stand
// alone or be followed by a space or colon. Block comments and tabs are not
// honored there.
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
