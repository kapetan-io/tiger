// Package config owns tiger.yaml: the facts a repository tells the analyzers
// about its world. Every entry is a value, a reason, and an optional package
// scope; an entry widens what an analyzer detects or corrects a dictionary,
// and no entry names a function, file, or site to exempt (ADR-0003/0005).
//
// The loader validates shape only — it knows nothing about what a value
// means; the analyzer does. Both drivers load the file and install it before
// running; analyzers ask Values for the entries that apply to the package
// they are analyzing and never read files themselves (ADR-0002). With
// nothing installed, Values returns nothing, so analysistest corpora run
// exactly as they did before the file existed.
package config

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
	"gopkg.in/yaml.v3"
)

// FileName is the config file's fixed name in the run directory.
const FileName = "tiger.yaml"

// version is the only schema version this loader accepts.
const version = 1

// Entry is one config value with the reason a reviewer accepted it and the
// module-relative package patterns it applies to; empty Packages means the
// whole module.
type Entry struct {
	Value    string   `yaml:"value"`
	Reason   string   `yaml:"reason"`
	Packages []string `yaml:"packages"`
}

// Module is a module path, as read from go.mod.
type Module string

// Relative rewrites pkgPath relative to the module: the module root is
// ".", a package below it loses the module prefix, and a package outside
// the module returns ok false.
func (m Module) Relative(pkgPath string) (string, bool) {
	if pkgPath == string(m) {
		return ".", true
	}
	if rest, found := strings.CutPrefix(pkgPath, string(m)+"/"); found {
		return rest, true
	}
	return "", false
}

// Key names one analyzer setting: the analyzer and its flag name, the
// two levels a tiger.yaml entry sits under.
type Key struct {
	Analyzer string
	Flag     string
}

// File is a loaded, validated tiger.yaml bound to the module it was read
// from.
type File struct {
	module  Module
	entries map[Key][]Entry
}

// Error is a load failure carrying the analyzer, key, and value the
// one-line message names.
type Error struct {
	Analyzer string
	Key      string
	Value    string
	Problem  string
}

func (e *Error) Error() string {
	parts := []string{FileName}
	if e.Analyzer != "" {
		parts = append(parts, e.Analyzer)
	}
	if e.Key != "" {
		parts = append(parts, e.Key)
	}
	if e.Value != "" {
		parts = append(parts, e.Value)
	}
	return strings.Join(parts, ": ") + ": " + e.Problem
}

// document is the file's decoded shape: version, then one map per analyzer
// name of key to entries.
type document struct {
	Version   int                           `yaml:"version"`
	Analyzers map[string]map[string][]Entry `yaml:",inline"`
}

// Load reads dir's tiger.yaml and validates it against analyzers: every
// top-level key names a registered analyzer and every key under it names
// one of that analyzer's flags. An absent file returns (nil, nil). The
// module path comes from dir's go.mod, which the file requires so package
// patterns can be resolved.
func Load(dir string, analyzers []*analysis.Analyzer) (*File, error) {
	raw, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, &Error{Problem: err.Error()}
	}
	module, err := ModulePath(dir)
	if err != nil {
		return nil, &Error{Problem: err.Error()}
	}
	var parsed document
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return nil, &Error{Problem: oneLine(err)}
	}
	if parsed.Version != version {
		return nil, &Error{Key: "version", Problem: fmt.Sprintf(
			"version %d is not supported — write version: %d", parsed.Version, version)}
	}
	loaded := &File{module: Module(module), entries: map[Key][]Entry{}}
	for _, name := range slices.Sorted(maps.Keys(parsed.Analyzers)) {
		if err := loaded.add(name, parsed.Analyzers[name], analyzers); err != nil {
			return nil, err
		}
	}
	return loaded, nil
}

// add validates one analyzer's block and records its entries.
func (f *File) add(name string, flags map[string][]Entry, analyzers []*analysis.Analyzer) error {
	registered := byName(analyzers, name)
	if registered == nil {
		return &Error{Analyzer: name, Problem: "no analyzer has this name — remove the block"}
	}
	for _, flagName := range slices.Sorted(maps.Keys(flags)) {
		key := Key{Analyzer: name, Flag: flagName}
		if registered.Flags.Lookup(flagName) == nil {
			return &Error{
				Analyzer: name, Key: flagName,
				Problem: "the analyzer has no setting with this name — remove the list",
			}
		}
		for _, entry := range flags[flagName] {
			if err := validate(key, entry); err != nil {
				return err
			}
		}
		f.entries[key] = append(f.entries[key], flags[flagName]...)
	}
	return nil
}

// validate checks one entry: a value, a reason, and well-formed patterns.
func validate(key Key, entry Entry) error {
	if strings.TrimSpace(entry.Value) == "" {
		return &Error{Analyzer: key.Analyzer, Key: key.Flag, Problem: "an entry has an " +
			"empty value — give it the value the setting takes, or remove it"}
	}
	if strings.TrimSpace(entry.Reason) == "" {
		return &Error{
			Analyzer: key.Analyzer, Key: key.Flag, Value: entry.Value,
			Problem: "the entry has no reason — add reason: with why a reviewer accepted it",
		}
	}
	for _, each := range entry.Packages {
		if !pattern(each).wellFormed() {
			return &Error{
				Analyzer: key.Analyzer, Key: key.Flag, Value: entry.Value,
				Problem: fmt.Sprintf("package pattern %q is not a module-relative package "+
					"path — write a path like internal/auth, internal/auth/..., or .", each),
			}
		}
	}
	return nil
}

// pattern is a module-relative Go package pattern: ".", "...", "a/b", or
// "a/b/...".
type pattern string

// wellFormed reports whether the pattern has no absolute, parent, or
// current-directory segments and no wildcard except a trailing one.
func (p pattern) wellFormed() bool {
	if p == "." || p == "..." {
		return true
	}
	text := string(p)
	if text == "" || strings.HasPrefix(text, "/") || strings.HasSuffix(text, "/") {
		return false
	}
	for _, segment := range strings.Split(strings.TrimSuffix(text, "/..."), "/") {
		if segment == "" || segment == "." || segment == ".." || segment == "..." {
			return false
		}
	}
	return true
}

// matches reports whether the module-relative package path rel is covered
// by the pattern.
func (p pattern) matches(rel string) bool {
	if p == "..." {
		return true
	}
	if prefix, found := strings.CutSuffix(string(p), "/..."); found {
		return rel == prefix || strings.HasPrefix(rel, prefix+"/")
	}
	return string(p) == rel
}

// byName finds one analyzer by name.
func byName(analyzers []*analysis.Analyzer, name string) *analysis.Analyzer {
	for _, analyzer := range analyzers {
		if analyzer.Name == name {
			return analyzer
		}
	}
	return nil
}

// oneLine collapses a multi-line yaml error into one line (ADR-0009).
func oneLine(err error) string {
	return strings.Join(strings.Fields(err.Error()), " ")
}

// ModulePath reads the module path from dir's go.mod.
func ModulePath(dir string) (Module, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("reading module path: %w", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if module, found := strings.CutPrefix(strings.TrimSpace(line), "module "); found {
			return Module(strings.TrimSpace(module)), nil
		}
	}
	return "", errors.New("go.mod has no module line")
}

// installed is the process-global config the current run installed — the
// same shape as analyzer flags, and reset the same way: every run installs
// what its directory holds, an absent file included.
var installed *File

// Install makes file the config every analyzer queries for the rest of
// the process, or clears it when file is nil.
func Install(file *File) {
	installed = file
}

// Installed reports whether a config file is currently installed.
func Installed() bool {
	return installed != nil
}

// Values returns the values of key that apply to the package at pkgPath:
// entries with no scope, plus entries whose scope covers the package's
// module-relative path. Nothing installed means nothing returned.
func Values(pkgPath string, key Key) []string {
	if installed == nil {
		return nil
	}
	rel, inModule := installed.module.Relative(pkgPath)
	values := []string{}
	for _, entry := range installed.entries[key] {
		if len(entry.Packages) == 0 {
			values = append(values, entry.Value)
			continue
		}
		if !inModule {
			continue
		}
		if slices.ContainsFunc(entry.Packages, func(each string) bool {
			return pattern(each).matches(rel)
		}) {
			values = append(values, entry.Value)
		}
	}
	return values
}
