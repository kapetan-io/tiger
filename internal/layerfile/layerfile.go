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

	"golang.org/x/mod/module"
	"gopkg.in/yaml.v3"

	"github.com/kapetan-io/tiger/assert"
	"github.com/kapetan-io/tiger/internal/config"
)

// FileName is the layer file's fixed name in the module root.
const FileName = "tiger.layers.yaml"

// RuleID is the rule the layer file backs.
const RuleID = "TS-X03"

const (
	subtreeTail  = "/..."
	modulePrefix = "./"
	rootKey      = "."
)

// impure are the standard library subtrees that leave the process or are
// nondeterministic. Naming one in a package's list allows its whole subtree
// for that package.
var impure = []string{
	"crypto/rand", "database", "io/ioutil", "math/rand", "net", "os", "plugin", "syscall",
}

// entry is one key of the layer file: a package, or a subtree of packages,
// the names it may import beyond the pure standard library, and the line the
// key sits on.
type entry struct {
	key   string
	names []string
	line  int
}

// File is a loaded, validated layer file bound to the module it was read
// from. The zero value is an empty file.
type File struct {
	module   config.Module
	layout   config.Layout
	entries  []entry
	standard *standardLibrary
	digest   string
}

// Empty reports whether there is no layer file, or it lists nothing.
func (f *File) Empty() bool {
	return f == nil || len(f.entries) == 0
}

// Digest identifies the file's bytes, so a driver that caches results can
// tell one layer file from another; an absent file has the empty digest.
func (f *File) Digest() string {
	if f == nil {
		return ""
	}
	return f.digest
}

// Load reads dir's layer file, the module path from dir's go.mod, and the
// module's layout. An absent file is an empty File, because the layer file
// is opt-in. A key that is not a package, two keys that cover the same
// package, or a name that can never match an import is an error naming the
// entry and its line.
func Load(dir string) (*File, error) {
	raw, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, os.ErrNotExist) {
		return &File{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	modulePath, err := config.ModulePath(dir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	layout, err := config.ReadLayout(dir)
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

// parse decodes raw into entries. It walks yaml nodes rather than decoding
// into a map so each entry keeps its line and a repeated key is still an
// error. The parser can name a line past the end, so every line is clamped to
// the file's last.
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

// lastLine is the number of the file's last line, 1 for an empty file.
func lastLine(raw []byte) int {
	count := bytes.Count(raw, []byte("\n"))
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		count++
	}
	return max(count, 1)
}

// syntaxError rewords a parser error to read like the other load errors,
// "line N: ...", without the parser's "yaml: " prefix.
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

// secondDocumentLine finds the "---" that starts the second document: the
// first document marker after the line the first document starts on. The
// parser can't say when the second document fails to parse.
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

// blank reports whether a document's node is the empty value a document of
// only "---" and comments holds.
func blank(node *yaml.Node) bool {
	return node.Kind == yaml.ScalarNode && node.Tag == "!!null" && node.Value == ""
}

// shapeError is the error for a file that is not a map of keys to lists.
func shapeError(line int) error {
	return fmt.Errorf("line %d: expected entries like \"./store\": [database, ./internal/types]",
		line)
}

// Coverage is how the layer file and a module's packages disagree.
type Coverage struct {
	// Unlisted are packages with no entry.
	Unlisted []config.Package
	// Stale are entries that match no package.
	Stale []Stale
}

// Stale is an entry that matches no package: its key and the line it sits on.
type Stale struct {
	Key  string
	Line int
}

// Coverage compares the entries with the packages the module's layout holds.
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
		if !slices.ContainsFunc(packages, func(pkg config.Package) bool {
			return each.covers(pkg.Dir)
		}) {
			found.Stale = append(found.Stale, Stale{Key: each.key, Line: each.line})
		}
	}
	return found
}

// relative rewrites pkgPath relative to the module, as config.Module.Relative
// does, except that a package of a nested module is outside: it belongs to
// that module and its own layer file.
func (f *File) relative(pkgPath string) (string, bool) {
	rel, inModule := f.module.Relative(pkgPath)
	if !inModule || f.nestedOwner(importPath(pkgPath)) != "" {
		return "", false
	}
	return rel, true
}

// nestedOwner returns the nested module that owns p, or "". The go command
// gives a package to the longest module path that matches it, so only a nested
// module whose path lies inside this module's path can take one of its
// packages; one whose path is a prefix of this module's, like example.com/m
// nested in example.com/m/v2, never does.
func (f *File) nestedOwner(p importPath) config.Module {
	owner := config.Module("")
	for _, nested := range f.layout.Nested {
		inside := strings.HasPrefix(string(nested), string(f.module)+"/")
		if inside && p.within(string(nested)) && len(nested) > len(owner) {
			owner = nested
		}
	}
	return owner
}

// installed is the process-global layer file the current run installed,
// reset the same way as config.Install: every run installs what its
// directory holds, an absent file included.
var installed *File

// Install makes file the layer file the layers analyzer queries for the
// rest of the process, or clears it when file is nil or empty.
func Install(file *File) {
	installed = file
}

// Installed reports whether a non-empty layer file is installed.
func Installed() bool {
	return !installed.Empty()
}

// Importer is one package as the installed layer file sees it: whether it is
// in the module, its directory, and the entry covering it, if any.
type Importer struct {
	file  *File
	dir   string
	entry *entry
}

// Locate places the package at pkgPath against the installed layer file.
// Nothing installed places every package outside the module, and so does a
// nested module.
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
		// Keys never overlap (validated at load), so at most one entry covers
		// a package.
		if installed.entries[i].covers(rel) {
			located.entry = &installed.entries[i]
			break
		}
	}
	return located
}

// InModule reports whether the package belongs to the layered module.
func (i Importer) InModule() bool {
	return i.dir != ""
}

// Listed reports whether an entry covers the package.
func (i Importer) Listed() bool {
	return i.entry != nil
}

// Dir is the package's directory relative to the module root: "." or
// "internal/store"; empty outside the module.
func (i Importer) Dir() string {
	return i.dir
}

// breach is the kind of import an entry does not allow; the zero value is
// none.
type breach int

// The breaches: a package of this module the entry does not name, a
// third-party package no named module path covers, and a standard library
// package in an impure subtree the entry does not name.
const (
	noBreach breach = iota
	unlistedPackage
	unlistedModule
	unlistedImpure
)

// Verdict is the judgment on one import: allowed, or what the entry lacks.
// The zero value allows the import.
type Verdict struct {
	breach   breach
	entryKey string
	imported string
	name     string
}

// Allowed reports whether the entry allows the import.
func (v Verdict) Allowed() bool {
	return v.breach == noBreach
}

// Judge decides whether the package's entry allows importing imported.
// Matching is exact: an in-module name is one package, a module path covers
// itself and the paths below it on a "/" boundary, and an impure name covers
// its subtree the same way. The package must be Listed.
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

// impureSubtree returns the impure subtree a standard library path sits in,
// or "".
func impureSubtree(imported string) string {
	for _, root := range impure {
		if importPath(imported).within(root) {
			return root
		}
	}
	return ""
}

// importPath is a package import path.
type importPath string

// within reports whether the path is root or a path below it on a "/"
// boundary, so "a/b" holds "a/b/c" but not "a/b-c".
func (p importPath) within(root string) bool {
	return string(p) == root || strings.HasPrefix(string(p), root+"/")
}

// packageKey spells a package directory, "." or "internal/store", the way
// the layer file names a package: "." for the root and "./dir" for the rest.
func packageKey(dir string) string {
	if dir == rootKey {
		return dir
	}
	return modulePrefix + dir
}

// keyDir splits a key into its directory, "." for the root, and whether it
// names a subtree. The key is already validated.
func keyDir(key string) (string, bool) {
	if key == rootKey {
		return rootKey, false
	}
	rest := strings.TrimPrefix(key, modulePrefix)
	return strings.CutSuffix(rest, subtreeTail)
}

// covers reports whether the entry covers the package in dir, "." or
// "internal/store".
func (e entry) covers(dir string) bool {
	keyed, subtree := keyDir(e.key)
	if !subtree {
		return keyed == dir
	}
	return dir == keyed || strings.HasPrefix(dir, keyed+"/")
}

// validateEntry checks one key and its names. A name spelled as this
// module's import path is refused, because Judge matches in-module imports
// against the "./dir" spelling only, and so is a "./dir" name inside a nested
// module, which Judge matches by that module's path.
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

// nestedModule returns the nested module an in-module name, "./dir", sits in,
// or "".
func (f *File) nestedModule(name string) config.Module {
	dir, found := strings.CutPrefix(name, modulePrefix)
	if !found {
		return ""
	}
	return f.nestedOwner(importPath(string(f.module) + "/" + dir))
}

// validateKey accepts ".", "./dir" and "./dir/...".
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

// cleanDir reports whether dir is a clean, relative, non-empty directory
// inside the module with no wildcard element.
func cleanDir(dir string) bool {
	return dir != "" && dir != "." && path.Clean(dir) == dir && !strings.HasPrefix(dir, "/") &&
		dir != ".." && !strings.HasPrefix(dir, "../") &&
		!slices.Contains(strings.Split(dir, "/"), "...")
}

// validateName accepts an in-module package, an impure subtree, or a module
// path, and refuses a name no import can match with what to write instead.
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
	if err := module.CheckImportPath(name); err != nil {
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

// validateModuleName checks a "./dir" name: one clean package of this
// module, never a subtree, because an in-module name matches one package.
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

// validateDistinct refuses two keys covering the same package. Two keys
// overlap only when one covers the other, so the broader one is a subtree.
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
