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

const FileName = "tiger.budget.yaml"

type Key struct {
	Package string
	Code    string
}

func (k Key) compare(other Key) int {
	return cmp.Or(cmp.Compare(k.Package, other.Package), cmp.Compare(k.Code, other.Code))
}

type row struct {
	budget int
	line   int
}

type File struct {
	rows map[Key]row
}

type Counts map[Key]int

type Run struct {
	Packages []string
	Codes    []string
}

type Overrun struct {
	Key    Key
	Count  int
	Budget int
	Line   int
	NoRow  bool
}

type nodePair struct {
	key   *yaml.Node
	value *yaml.Node
}

func pairs(mapping *yaml.Node) []nodePair {
	listed := []nodePair{}
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		listed = append(listed, nodePair{key: mapping.Content[i], value: mapping.Content[i+1]})
	}
	return listed
}

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

func (f *File) Lower(counts Counts, run Run) *File {
	lowered := &File{rows: map[Key]row{}}
	for _, key := range slices.SortedFunc(maps.Keys(f.rows), Key.compare) {
		lowered.put(key, f.rows[key])
	}
	for _, pkg := range run.Packages {
		for _, code := range run.Codes {
			key := Key{Package: pkg, Code: code}
			next := row{budget: counts[key]}
			if existing, found := f.rows[key]; found {
				next = row{budget: min(existing.budget, next.budget), line: existing.line}
			}
			lowered.put(key, next)
		}
	}
	return lowered
}

func (f *File) put(key Key, next row) {
	if next.budget == 0 {
		delete(f.rows, key)
		return
	}
	f.rows[key] = next
}

func (f *File) Rows() map[Key]int {
	rows := map[Key]int{}
	for _, key := range slices.SortedFunc(maps.Keys(f.rows), Key.compare) {
		rows[key] = f.rows[key].budget
	}
	return rows
}

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

func fill(temp *os.File, content []byte) error {
	if _, err := temp.Write(content); err != nil {
		return errors.Join(err, temp.Close())
	}
	if err := temp.Chmod(0o644); err != nil {
		return errors.Join(err, temp.Close())
	}
	return temp.Close()
}
