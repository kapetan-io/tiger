// Package budget owns tiger.budget.yaml: what a repository admits it owes,
// as one non-negative integer per (module-relative package path, advisory
// rule code). The driver supplies counts; this package never runs analysis.
//
// Its contract is the ratchet's lowering-only half (ADR-0011): Lower takes
// the loaded file and the current counts and produces rows where every
// number is min(existing, count) for a row that existed and count for one
// that did not, deleting rows at 0 and packages with no rows left. No API
// sets a row to an arbitrary value, so a written number is never greater
// than the one it replaced. The comparison half — count against budget —
// is Compare; the "never raised without a commit that says so" half is
// review's.
package budget

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kapetan-io/tiger/assert"
)

// FileName is the budget file's fixed name in the run directory.
const FileName = "tiger.budget.yaml"

// Key identifies one row: a module-relative package path and the
// user-facing code of an advisory rule.
type Key struct {
	Package string
	Code    string
}

// compare orders keys by package then code.
func (k Key) compare(other Key) int {
	return cmp.Or(cmp.Compare(k.Package, other.Package), cmp.Compare(k.Code, other.Code))
}

// row is one budget number with the line it was read from, for positions.
type row struct {
	budget int
	line   int
}

// File is a loaded, validated budget file.
type File struct {
	rows map[Key]row
}

// Counts is the current count of counted findings per row key, over the
// analyzed packages only.
type Counts map[Key]int

// Run names what one analysis covered: the package keys analyzed and the
// rule codes counted, so Lower can tell a row at 0 from a row outside the
// run.
type Run struct {
	Packages []string
	Codes    []string
}

// Overrun is one row whose count exceeds its budget. Line is the row's
// line in the file, or 0 when the file has no row, in which case NoRow is
// set.
type Overrun struct {
	Key    Key
	Count  int
	Budget int
	Line   int
	NoRow  bool
}

// nodePair is one key/value pair of a YAML mapping node.
type nodePair struct {
	key   *yaml.Node
	value *yaml.Node
}

// pairs splits a mapping node's content into key/value pairs.
func pairs(mapping *yaml.Node) []nodePair {
	listed := []nodePair{}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		listed = append(listed, nodePair{key: mapping.Content[i], value: mapping.Content[i+1]})
	}
	return listed
}

// Load reads dir's budget file, validating every row against allowed —
// the set of rule codes registered at advisory severity. An absent file
// is an empty budget. A row with a package key that is not a
// module-relative path, a code outside allowed, or a value that is not a
// non-negative integer is an error naming the row.
func Load(dir string, allowed map[string]bool) (*File, error) {
	loaded := &File{rows: map[Key]row{}}
	raw, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, os.ErrNotExist) {
		return loaded, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("%s: %s", FileName, strings.Join(strings.Fields(err.Error()), " "))
	}
	if len(root.Content) == 0 {
		return loaded, nil
	}
	top := root.Content[0]
	if top.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: the file must be a map from package path to rule rows",
			FileName)
	}
	for _, block := range pairs(top) {
		if err := loaded.addPackage(block, allowed); err != nil {
			return nil, err
		}
	}
	return loaded, nil
}

// addPackage validates and records one package block.
func (f *File) addPackage(block nodePair, allowed map[string]bool) error {
	pkg := block.key.Value
	if !wellFormedPackage(pkg) {
		return fmt.Errorf("%s:%d: %q is not a module-relative package path — write a path "+
			"like internal/cli, or . for the module root", FileName, block.key.Line, pkg)
	}
	if block.value.Kind != yaml.MappingNode {
		return fmt.Errorf("%s:%d: %s must map rule codes to numbers", FileName,
			block.key.Line, pkg)
	}
	for _, entry := range pairs(block.value) {
		code := entry.key.Value
		if !allowed[code] {
			return fmt.Errorf("%s:%d: %s: %s is not a rule code counted against budgets — "+
				"remove the row", FileName, entry.key.Line, pkg, code)
		}
		number, err := strconv.Atoi(entry.value.Value)
		if err != nil || number < 0 || entry.value.Kind != yaml.ScalarNode {
			return fmt.Errorf("%s:%d: %s: %s must be a whole number of 0 or more, not %q",
				FileName, entry.value.Line, pkg, code, entry.value.Value)
		}
		f.rows[Key{Package: pkg, Code: code}] = row{budget: number, line: entry.value.Line}
	}
	return nil
}

// wellFormedPackage reports whether key is "." or a relative path with no
// empty, current, parent, or wildcard segments.
func wellFormedPackage(key string) bool {
	if key == "." {
		return true
	}
	if key == "" || strings.HasPrefix(key, "/") {
		return false
	}
	for _, segment := range strings.Split(key, "/") {
		if segment == "" || segment == "." || segment == ".." || segment == "..." {
			return false
		}
	}
	return true
}

// Compare returns every key in counts whose count exceeds its budget, a
// missing row counting as 0, sorted by package then code.
func (f *File) Compare(counts Counts) []Overrun {
	overruns := []Overrun{}
	for _, key := range slices.SortedFunc(maps.Keys(counts), Key.compare) {
		count := counts[key]
		existing, found := f.rows[key]
		if found && count <= existing.budget {
			continue
		}
		if !found && count == 0 {
			continue
		}
		overruns = append(overruns, Overrun{
			Key: key, Count: count, Budget: existing.budget, Line: existing.line, NoRow: !found,
		})
	}
	return overruns
}

// Lower returns the file Write would store: for every analyzed package and
// every counted code, min(existing, count) for an existing row and count
// for a new one, dropping rows at 0 and packages left empty. Packages
// outside the run keep their rows untouched. Rows that existed keep their
// line numbers; new rows have none.
func (f *File) Lower(counts Counts, run Run) *File {
	lowered := &File{rows: map[Key]row{}}
	for _, key := range slices.SortedFunc(maps.Keys(f.rows), Key.compare) {
		lowered.put(key, f.rows[key])
	}
	for _, pkg := range run.Packages {
		for _, code := range run.Codes {
			key := Key{Package: pkg, Code: code}
			next := row{budget: counts[key]}
			// A row that existed keeps its line, so an overrun that survives
			// the write is still positioned at the row.
			if existing, found := f.rows[key]; found {
				next = row{budget: min(existing.budget, next.budget), line: existing.line}
			}
			lowered.put(key, next)
		}
	}
	return lowered
}

// put sets one row, deleting it at 0.
func (f *File) put(key Key, next row) {
	if next.budget == 0 {
		delete(f.rows, key)
		return
	}
	f.rows[key] = next
}

// Rows returns every row's number by key.
func (f *File) Rows() map[Key]int {
	rows := map[Key]int{}
	for _, key := range slices.SortedFunc(maps.Keys(f.rows), Key.compare) {
		rows[key] = f.rows[key].budget
	}
	return rows
}

// Encode renders the file's rows as bytes: keys sorted, two-space indent,
// no comments, trailing newline — byte-deterministic for a given set of
// rows. No rows encode to an empty file.
func (f *File) Encode() []byte {
	if len(f.rows) == 0 {
		return []byte{}
	}
	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	var block *yaml.Node
	previous := ""
	for _, key := range slices.SortedFunc(maps.Keys(f.rows), Key.compare) {
		if block == nil || key.Package != previous {
			block = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			root.Content = append(root.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key.Package}, block)
			previous = key.Package
		}
		block.Content = append(block.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key.Code},
			&yaml.Node{
				Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(f.rows[key].budget),
			})
	}
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	assert.Ok(encoder.Encode(root) == nil, "a node tree of plain scalars encodes")
	assert.Ok(encoder.Close() == nil, "an in-memory buffer has nothing to flush")
	return out.Bytes()
}

// Write stores the file in dir atomically: a temp file in the same
// directory, then a rename, so a crash never leaves a half-written file.
func (f *File) Write(dir string) error {
	temp, err := os.CreateTemp(dir, FileName+".*.tmp")
	if err != nil {
		return fmt.Errorf("writing %s: %w", FileName, err)
	}
	if err := fill(temp, f.Encode()); err != nil {
		return errors.Join(fmt.Errorf("writing %s: %w", FileName, err), os.Remove(temp.Name()))
	}
	if err := os.Rename(temp.Name(), filepath.Join(dir, FileName)); err != nil {
		return errors.Join(fmt.Errorf("writing %s: %w", FileName, err), os.Remove(temp.Name()))
	}
	return nil
}

// fill writes content to temp with the mode a hand-edited file has, and
// closes it.
func fill(temp *os.File, content []byte) error {
	if _, err := temp.Write(content); err != nil {
		return errors.Join(err, temp.Close())
	}
	if err := temp.Chmod(0o644); err != nil {
		return errors.Join(err, temp.Close())
	}
	return temp.Close()
}
