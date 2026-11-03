// Package sortedmapoutput prints notes attached to syntax nodes in a fixed
// order, the way go/ast's CommentMap.String needed to.
package sortedmapoutput

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

type Node interface {
	Pos() int
	End() int
}

type Statement struct{ Start, Stop int }

func (s *Statement) Pos() int { return s.Start }
func (s *Statement) End() int { return s.Stop }

type Call struct{ Start, Stop int }

func (c *Call) Pos() int { return c.Start }
func (c *Call) End() int { return c.Stop }

type Notes map[Node]string

type Note struct {
	Node Node
	Text string
}

type Document struct {
	notes map[int]Note
}

func NewDocument() *Document { return &Document{notes: map[int]Note{}} }

func (d *Document) Attach(node Node, text string) int {
	sequence := len(d.notes)
	d.notes[sequence] = Note{Node: node, Text: text}
	return sequence
}

func (d *Document) Render() string {
	var out strings.Builder
	for _, sequence := range slices.Sorted(maps.Keys(d.notes)) {
		note := d.notes[sequence]
		fmt.Fprintf(&out, "%d-%d: %s\n", note.Node.Pos(), note.Node.End(), note.Text)
	}
	return out.String()
}

func IndexByPosition(notes Notes) map[int]string {
	index := map[int]string{}
	for node, text := range notes {
		index[node.Pos()] = text
	}
	return index
}

func RenderByPosition(notes Notes) string {
	index := IndexByPosition(notes)
	var out strings.Builder
	for _, position := range slices.Sorted(maps.Keys(index)) {
		fmt.Fprintf(&out, "%d: %s\n", position, index[position])
	}
	return out.String()
}
