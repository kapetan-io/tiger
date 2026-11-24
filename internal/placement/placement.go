// Package placement locates well-formed pin directives and binds each to the
// declaration or statement it pins.
package placement

import (
	"go/ast"
	"go/token"

	"github.com/kapetan-io/tiger/internal/directive"
)

type Pin struct {
	Directive directive.Directive
	Pos       token.Pos
}

type lineKey struct {
	file string
	line int
}

type Set struct {
	fset     *token.FileSet
	attached map[lineKey][]Pin
}

func Collect(fset *token.FileSet, files []*ast.File) Set {
	collected := Set{fset: fset, attached: map[lineKey][]Pin{}}
	for _, file := range files {
		for _, group := range file.Comments {
			end := fset.Position(group.End())
			key := lineKey{file: end.Filename, line: end.Line + 1}
			for _, comment := range group.List {
				collected.add(key, file, comment)
			}
		}
	}
	return collected
}

func (s Set) add(key lineKey, file *ast.File, comment *ast.Comment) {
	if directive.Classify(file, comment) != directive.Tiger {
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
