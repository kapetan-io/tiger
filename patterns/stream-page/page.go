// Package streampage shows a page loop over a stream the module does not own
// (a bufio.Scanner, standing in for a database cursor opened with a LIMIT),
// and the page that hands the caller no way to ask for the next one.
package streampage

import "bufio"

const pageMax = 1_000

const pageMin = 1

const _ = uint(pageMax - pageMin)

type Opener func(after string, limit int) *bufio.Scanner

type Page struct {
	Names []string
	After string
}

func clampPage(requested int) int {
	return max(pageMin, min(requested, pageMax))
}

func ListPage(open Opener, after string, requested int) (Page, error) {
	limit := clampPage(requested)
	lines := open(after, limit)
	var names []string
	for count := 0; count < limit && lines.Scan(); count++ {
		names = append(names, lines.Text())
	}
	if err := lines.Err(); err != nil {
		return Page{}, err
	}
	if len(names) < limit {
		return Page{Names: names}, nil
	}
	return Page{Names: names, After: names[len(names)-1]}, nil
}

func ListFromStart(open Opener, requested int) []string {
	limit := clampPage(requested)
	lines := open("", limit)
	var names []string
	for count := 0; count < limit && lines.Scan(); count++ {
		names = append(names, lines.Text())
	}
	return names
}
