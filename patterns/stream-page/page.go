// Package streampage shows a page loop over a stream the module does not own
// (a bufio.Scanner, standing in for a database cursor opened with a LIMIT),
// and the page that hands the caller no way to ask for the next one.
package streampage

import "bufio"

// pageMax is the most rows one page asks for.
const pageMax = 1_000

// pageMin is the fewest rows one page asks for, so a zero limit still moves.
const pageMin = 1

// A floor above the maximum would leave no page size that is valid.
const _ = uint(pageMax - pageMin)

// Opener starts a read of at most limit rows that sort after the row named
// after, the way a query carries WHERE key > ? LIMIT ?.
type Opener func(after string, limit int) *bufio.Scanner

// Page is one slice of a listing. After is the row to pass to the next call,
// empty when the stream ended before the page filled.
type Page struct {
	Names []string
	After string
}

// clampPage keeps a requested page size in pageMin..pageMax.
func clampPage(requested int) int {
	return max(pageMin, min(requested, pageMax))
}

// ListPage restates the clamped limit in the loop header, so the loop ends at
// the limit whatever the stream does, and returns the row the next call
// resumes after, so a client can walk the whole listing without a rescan.
func ListPage(open Opener, after string, requested int) Page {
	limit := clampPage(requested)
	lines := open(after, limit)
	var names []string
	for count := 0; count < limit && lines.Scan(); count++ {
		names = append(names, lines.Text())
	}
	if len(names) < limit {
		return Page{Names: names}
	}
	return Page{Names: names, After: names[len(names)-1]}
}

// ListFromStart has the same bounded loop and no resume point. Tiger sees a
// limit over an outside stream and accepts it; every call starts at the top,
// so a client paging through the listing reads the first page forever.
func ListFromStart(open Opener, requested int) []string {
	limit := clampPage(requested)
	lines := open("", limit)
	var names []string
	for count := 0; count < limit && lines.Scan(); count++ {
		names = append(names, lines.Text())
	}
	return names
}
