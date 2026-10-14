// Package sortedmapoutput prints notes attached to syntax nodes in a fixed
// order, the way go/ast's CommentMap.String needed to.
package sortedmapoutput

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Node is anything a note can be attached to. Nodes are pointers, so a map
// keyed by Node has no order of its own and nothing about it can be sorted.
type Node interface {
	Pos() int
	End() int
}

// Statement wraps a Call and covers the same source span.
type Statement struct{ Start, Stop int }

func (s *Statement) Pos() int { return s.Start }
func (s *Statement) End() int { return s.Stop }

// Call is the expression inside a Statement.
type Call struct{ Start, Stop int }

func (c *Call) Pos() int { return c.Start }
func (c *Call) End() int { return c.Stop }

// Notes maps each node to the note attached to it.
type Notes map[Node]string

// Note is one attached note.
type Note struct {
	Node Node
	Text string
}

// Document keeps notes under a sequence number handed out in attach order.
// The sequence is the sort key: it is unique per note, so no two notes tie.
type Document struct {
	notes map[int]Note
}

// NewDocument returns an empty Document.
func NewDocument() *Document { return &Document{notes: map[int]Note{}} }

// Attach records a note and returns its sequence number.
func (d *Document) Attach(node Node, text string) int {
	sequence := len(d.notes)
	d.notes[sequence] = Note{Node: node, Text: text}
	return sequence
}

// Render prints one line per note in attach order. Ranging over the sorted
// sequence numbers means map order never reaches the output.
func (d *Document) Render() string {
	var out strings.Builder
	for _, sequence := range slices.Sorted(maps.Keys(d.notes)) {
		note := d.notes[sequence]
		fmt.Fprintf(&out, "%d-%d: %s\n", note.Node.Pos(), note.Node.End(), note.Text)
	}
	return out.String()
}

// IndexByPosition keys each note by its node's start position. A Statement
// and the Call it wraps share a position, so their notes land in the same
// slot and the one visited last wins. Tiger passes this: the sorted output
// below ranges over int keys, and it does not check that two map entries can
// write one slot (the invert known miss of TS-T02).
func IndexByPosition(notes Notes) map[int]string {
	index := map[int]string{}
	for node, text := range notes {
		index[node.Pos()] = text
	}
	return index
}

// RenderByPosition prints the index in position order. The positions are
// sorted, but which note each one holds was already decided by map order.
func RenderByPosition(notes Notes) string {
	index := IndexByPosition(notes)
	var out strings.Builder
	for _, position := range slices.Sorted(maps.Keys(index)) {
		fmt.Fprintf(&out, "%d: %s\n", position, index[position])
	}
	return out.String()
}
