// Package pins locates well-formed pin directives and binds each to the
// declaration or statement it pins.
//
// This is shared infrastructure for the pin-bearing analyzers (effects,
// frames, variant, contracts): each needs the same attachment rule — a pin
// binds to the node starting on the line after its comment group — so a pin
// can never bind to one node for one analyzer and another for the next.
// Malformed pins are skipped here; the directives analyzer owns their
// TS-L09 findings.
package pins

import (
	"go/ast"
	"go/token"

	"github.com/kapetan-io/tiger/internal/directive"
)

// Pin is one well-formed pin directive bound to a node.
type Pin struct {
	// Directive is the parsed pin; its Args parse cleanly under the verb's
	// grammar, so consumers can re-parse without error handling.
	Directive directive.Directive
	// Pos locates the directive comment, for findings that point at the pin.
	Pos token.Pos
}

// lineKey addresses the source line a comment group attaches to.
type lineKey struct {
	file string
	line int
}

// Set holds every collected pin, addressable by the node it binds to.
type Set struct {
	fset     *token.FileSet
	attached map[lineKey][]Pin
}

// Collect gathers the well-formed pin directives in files. Every directive
// in a comment group attaches to the line after the group's end — the rule
// that makes doc-comment pins and free-standing pins above a loop behave
// identically.
func Collect(fset *token.FileSet, files []*ast.File) Set {
	collected := Set{fset: fset, attached: map[lineKey][]Pin{}}
	for _, file := range files {
		for _, group := range file.Comments {
			end := fset.Position(group.End())
			key := lineKey{file: end.Filename, line: end.Line + 1}
			for _, comment := range group.List {
				collected.add(key, comment)
			}
		}
	}
	return collected
}

// add appends one comment's pin to key, if it is a well-formed pin.
func (s Set) add(key lineKey, comment *ast.Comment) {
	if !directive.Is(comment.Text) {
		return
	}
	parsed, err := directive.Parse(comment.Text)
	if err != nil {
		return
	}
	spec, ok := directive.Lookup(parsed.Verb)
	if !ok {
		return
	}
	if spec.Kind != directive.KindPin {
		return
	}
	s.attached[key] = append(s.attached[key], Pin{Directive: parsed, Pos: comment.Pos()})
}

// At returns the pins with the given verb bound to node, in source order.
func (s Set) At(node ast.Node, verb string) []Pin {
	start := s.fset.Position(node.Pos())
	matched := []Pin{}
	for _, pin := range s.attached[lineKey{file: start.Filename, line: start.Line}] {
		if pin.Directive.Verb == verb {
			matched = append(matched, pin)
		}
	}
	return matched
}
