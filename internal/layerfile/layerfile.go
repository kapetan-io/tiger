// Package layerfile owns tiger.layers.yaml: for every package in a module,
// what it may import beyond the pure standard library (TS-X03). The file maps
// a package to a list of names. Load validates it, the drivers Install it,
// and the layers analyzer asks Locate where a package sits and Judge whether
// one of its imports is listed. Coverage compares the file with the packages
// a module really has, which only the tiger CLI reports. Every TS-X03 message
// is worded here.
//
// Naming the impure packages a package may import keeps most IO and all
// randomness behind interfaces passed in. It is not a proof: a few pure
// standard library packages reach the file system or the network on their
// own (path/filepath.Glob, html/template.ParseFiles, log/syslog, crypto/tls).
package layerfile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	gomodule "golang.org/x/mod/module"
	"gopkg.in/yaml.v3"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/module"
)

const FileName = "tiger.layers.yaml"

const RuleID = "TS-X03"

const (
	subtreeTail  = "/..."
	modulePrefix = "./"
	rootKey      = "."
)

var impure = []string{
	"crypto/rand", "database", "io/ioutil", "math/rand", "net", "os", "plugin", "syscall",
}

type entry struct {
	key   string
	names []string
	line  int
}

type File struct {
	module   module.Path
	layout   module.Layout
	entries  []entry
	standard *standardLibrary
	digest   string
}

func (f *File) Empty() bool {
	return f == nil || len(f.entries) == 0
}

func (f *File) Digest() string {
	if f == nil {
		return ""
	}
	return f.digest
}

func Load(dir string) (*File, error) {
	raw, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, os.ErrNotExist) {
		return &File{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	modulePath, err := module.ReadPath(dir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	layout, err := module.ReadLayout(dir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	sum := sha256.Sum256(raw)
	loaded := &File{
		module:   modulePath,
		layout:   layout,
		standard: newStandardLibrary(dir),
		digest:   hex.EncodeToString(sum[:]),
	}
	if err := loaded.parse(raw); err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	return loaded, nil
}

func (f *File) parse(raw []byte) error {
	last := lastLine(raw)
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	var document yaml.Node
	err := decoder.Decode(&document)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return syntaxError(err, last)
	}
	var second yaml.Node
	if err := decoder.Decode(&second); !errors.Is(err, io.EOF) {
		return fmt.Errorf("line %d: a second YAML document starts here; keep every entry in one "+
			"map", min(secondDocumentLine(raw, document.Line), last))
	}
	if len(document.Content) == 0 || blank(document.Content[0]) {
		return nil
	}
	top := document.Content[0]
	if top.Kind != yaml.MappingNode {
		return shapeError(min(top.Line, last))
	}
	for i := 0; i < len(top.Content); i += 2 {
		keyNode, listNode := top.Content[i], top.Content[i+1]
		names := []string{}
		if keyNode.Kind != yaml.ScalarNode || listNode.Kind != yaml.SequenceNode ||
			listNode.Decode(&names) != nil {
			return shapeError(keyNode.Line)
		}
		key := keyNode.Value
		index := slices.IndexFunc(f.entries, func(e entry) bool { return e.key == key })
		if index >= 0 {
			return fmt.Errorf("line %d: %q is already defined at line %d", keyNode.Line, key,
				f.entries[index].line)
		}
		if err := f.validateEntry(key, names); err != nil {
			return fmt.Errorf("line %d: %w", keyNode.Line, err)
		}
		f.entries = append(f.entries, entry{
			key:   key,
			names: slices.Sorted(slices.Values(names)),
			line:  keyNode.Line,
		})
	}
	slices.SortFunc(f.entries, func(a, b entry) int { return strings.Compare(a.key, b.key) })
	return validateDistinct(f.entries)
}

func lastLine(raw []byte) int {
	count := bytes.Count(raw, []byte("\n"))
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		count++
	}
	return max(count, 1)
}

func syntaxError(err error, last int) error {
	text := strings.TrimPrefix(strings.Join(strings.Fields(err.Error()), " "), "yaml: ")
	rest, found := strings.CutPrefix(text, "line ")
	number, message, _ := strings.Cut(rest, ": ")
	line, convErr := strconv.Atoi(number)
	if !found || convErr != nil {
		return errors.New(text)
	}
	return fmt.Errorf("line %d: %s", min(line, last), message)
}

func secondDocumentLine(raw []byte, start int) int {
	for i, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimRight(line, "\r")
		marker := line == "---" || strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "---\t")
		if i+1 > start && marker {
			return i + 1
		}
	}
	return start
}

func blank(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && node.Tag == "!!null" && node.Value == ""
}

func shapeError(line int) error {
	return fmt.Errorf("line %d: expected entries like \"./store\": [database, ./internal/types]",
		line)
}

type Coverage struct {
	Unlisted []module.Package
	Stale    []Stale
}

type Stale struct {
	Key  string
	Line int
}

func (f *File) Coverage() Coverage {
	var found Coverage
	if f.Empty() {
		return found
	}
	packages := f.layout.Packages
	for _, pkg := range packages {
		if !slices.ContainsFunc(f.entries, func(e entry) bool { return e.covers(pkg.Dir) }) {
			found.Unlisted = append(found.Unlisted, pkg)
		}
	}
	for _, each := range f.entries {
		if !slices.ContainsFunc(packages, func(pkg module.Package) bool {
			return each.covers(pkg.Dir)
		}) {
			found.Stale = append(found.Stale, Stale{Key: each.key, Line: each.line})
		}
	}
	return found
}

func (f *File) relative(pkgPath string) (string, bool) {
	rel, inModule := f.module.Relative(pkgPath)
	if !inModule || f.nestedOwner(importPath(pkgPath)) != "" {
		return "", false
	}
	return rel, true
}

func (f *File) nestedOwner(p importPath) module.Path {
	owner := module.Path("")
	for _, nested := range f.layout.Nested {
		inside := strings.HasPrefix(string(nested), string(f.module)+"/")
		if inside && p.within(string(nested)) && len(nested) > len(owner) {
			owner = nested
		}
	}
	return owner
}

var installed *File

func Install(file *File) {
	installed = file
}

func Installed() bool {
	return !installed.Empty()
}

type Importer struct {
	file  *File
	dir   string
	entry *entry
}

func Locate(pkgPath string) Importer {
	if installed.Empty() {
		return Importer{}
	}
	rel, inModule := installed.relative(pkgPath)
	if !inModule {
		return Importer{}
	}
	located := Importer{file: installed, dir: rel}
	for i := range installed.entries {
		if installed.entries[i].covers(rel) {
			located.entry = &installed.entries[i]
			break
		}
	}
	return located
}

func (i Importer) InModule() bool {
	return i.dir != ""
}

func (i Importer) Listed() bool {
	return i.entry != nil
}

func (i Importer) Dir() string {
	return i.dir
}

type breach int

const (
	noBreach breach = iota
	unlistedPackage
	unlistedModule
	unlistedImpure
)

type Verdict struct {
	breach   breach
	entryKey string
	imported string
	name     string
}

func (v Verdict) Allowed() bool {
	return v.breach == noBreach
}

func (i Importer) Judge(imported string) Verdict {
	assert.Ok(i.entry != nil, "only a package with an entry is judged import by import")
	names := i.entry.names
	verdict := Verdict{entryKey: i.entry.key, imported: imported}
	if imported == "C" || imported == "unsafe" {
		return Verdict{}
	}
	if rel, inModule := i.file.relative(imported); inModule {
		verdict.name = packageKey(rel)
		if !slices.Contains(names, verdict.name) {
			verdict.breach = unlistedPackage
		}
		return verdict
	}
	if i.file.standard.contains(imported) {
		verdict.name = impureSubtree(imported)
		if verdict.name != "" && !slices.Contains(names, verdict.name) {
			verdict.breach = unlistedImpure
		}
		return verdict
	}
	verdict.name = imported
	if !slices.ContainsFunc(names, importPath(imported).within) {
		verdict.breach = unlistedModule
	}
	return verdict
}

func impureSubtree(imported string) string {
	for _, root := range impure {
		if importPath(imported).within(root) {
			return root
		}
	}
	return ""
}

type importPath string

func (p importPath) within(root string) bool {
	return string(p) == root || strings.HasPrefix(string(p), root+"/")
}

func packageKey(dir string) string {
	if dir == rootKey {
		return dir
	}
	return modulePrefix + dir
}

func keyDir(key string) (string, bool) {
	if key == rootKey {
		return rootKey, false
	}
	rest := strings.TrimPrefix(key, modulePrefix)
	return strings.CutSuffix(rest, subtreeTail)
}

func (e entry) covers(dir string) bool {
	keyed, subtree := keyDir(e.key)
	if !subtree {
		return keyed == dir
	}
	return dir == keyed || strings.HasPrefix(dir, keyed+"/")
}

func (f *File) validateEntry(key string, names []string) error {
	if err := validateKey(key); err != nil {
		return err
	}
	for i, name := range names {
		if rel, inModule := f.relative(name); inModule {
			return fmt.Errorf("%q: %q is this module; write it as %q", key, name,
				packageKey(rel))
		}
		if err := f.validateName(name); err != nil {
			return fmt.Errorf("%q: %w", key, err)
		}
		if nested := f.nestedModule(name); nested != "" {
			return fmt.Errorf("%q: %q is in the nested module %s; name that module path", key,
				name, nested)
		}
		if slices.Contains(names[:i], name) {
			return fmt.Errorf("%q: %q is listed twice", key, name)
		}
	}
	return nil
}

func (f *File) nestedModule(name string) module.Path {
	dir, found := strings.CutPrefix(name, modulePrefix)
	if !found {
		return ""
	}
	return f.nestedOwner(importPath(string(f.module) + "/" + dir))
}

func validateKey(key string) error {
	if key == rootKey {
		return nil
	}
	rest, found := strings.CutPrefix(key, modulePrefix)
	dir := strings.TrimSuffix(rest, subtreeTail)
	if !found || !cleanDir(dir) {
		return fmt.Errorf("%q isn't a package; use \".\", \"./dir\" or \"./dir/...\"", key)
	}
	return nil
}

func cleanDir(dir string) bool {
	return dir != "" && dir != "." && path.Clean(dir) == dir && !strings.HasPrefix(dir, "/") &&
		dir != ".." && !strings.HasPrefix(dir, "../") &&
		!slices.Contains(strings.Split(dir, "/"), "...")
}

func (f *File) validateName(name string) error {
	if name == rootKey || slices.Contains(impure, name) {
		return nil
	}
	if strings.HasPrefix(name, modulePrefix) {
		return validateModuleName(name)
	}
	if trimmed, found := strings.CutSuffix(name, subtreeTail); found {
		return fmt.Errorf("%q: a module path already covers every path below it; write %q",
			name, trimmed)
	}
	if strings.HasSuffix(name, "/") {
		return fmt.Errorf("%q ends in a slash; write %q", name, strings.TrimRight(name, "/"))
	}
	if name == "C" {
		return fmt.Errorf("%q is always allowed; remove it", name)
	}
	if err := gomodule.CheckImportPath(name); err != nil {
		return fmt.Errorf("%q isn't an import path; write a module path like \"example.com/lib\"",
			name)
	}
	if !f.standard.contains(name) {
		return nil
	}
	if root := impureSubtree(name); root != "" {
		return fmt.Errorf("%q is under %s; name %q to allow it", name, root, root)
	}
	return fmt.Errorf("%q is always allowed; remove it", name)
}

func validateModuleName(name string) error {
	rest := strings.TrimPrefix(name, modulePrefix)
	if dir, found := strings.CutSuffix(rest, "..."); found {
		example := "./store"
		if dir = strings.TrimSuffix(dir, "/"); cleanDir(dir) {
			example = modulePrefix + dir
		}
		return fmt.Errorf("%q names a subtree, but a name is one package; list each package, "+
			"like %q", name, example)
	}
	if cleanDir(rest) {
		return nil
	}
	example := "./store"
	if cleaned := path.Clean(strings.TrimLeft(rest, "/")); cleanDir(cleaned) {
		example = modulePrefix + cleaned
	}
	return fmt.Errorf("%q isn't a package of this module; write it like %q", name, example)
}

func validateDistinct(entries []entry) error {
	for i, first := range entries {
		for _, second := range entries[:i] {
			broad, narrow := second, first
			if secondDir, _ := keyDir(second.key); first.covers(secondDir) {
				broad, narrow = first, second
			}
			dir, _ := keyDir(narrow.key)
			if !broad.covers(dir) {
				continue
			}
			return fmt.Errorf("line %d: %q and %q both cover %s — remove %q, or replace %q with "+
				"one key per package", narrow.line, broad.key, narrow.key, packageKey(dir),
				narrow.key, broad.key)
		}
	}
	return nil
}
