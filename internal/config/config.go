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

	"github.com/kapetan-io/tiger/internal/module"
)

const FileName = "tiger.yaml"

const version = 1

type Entry struct {
	Value    string   `yaml:"value"`
	Reason   string   `yaml:"reason"`
	Packages []string `yaml:"packages"`
}

type Key struct {
	Analyzer string
	Flag     string
}

type File struct {
	module  module.Path
	root    string
	entries map[Key][]Entry
	nolint  bool
}

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

type document struct {
	Version   int                           `yaml:"version"`
	Nolint    yaml.Node                     `yaml:"nolint"`
	Analyzers map[string]map[string][]Entry `yaml:",inline"`
}

type nolintBlock struct {
	Forbid bool   `yaml:"forbid"`
	Reason string `yaml:"reason"`
}

func Load(dir string, analyzers []*analysis.Analyzer) (*File, error) {
	raw, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, &Error{Problem: err.Error()}
	}
	modulePath, err := module.ReadPath(dir)
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
	loaded := &File{module: modulePath, root: dir, entries: map[Key][]Entry{}}
	forbid, err := parseNolint(&parsed.Nolint)
	if err != nil {
		return nil, err
	}
	loaded.nolint = forbid
	for _, name := range slices.Sorted(maps.Keys(parsed.Analyzers)) {
		if err := loaded.add(name, parsed.Analyzers[name], analyzers); err != nil {
			return nil, err
		}
	}
	return loaded, nil
}

func parseNolint(node *yaml.Node) (bool, error) {
	if node.IsZero() {
		return false, nil
	}
	if node.Kind != yaml.MappingNode {
		return false, &Error{Analyzer: "nolint", Problem: "the block must hold forbid: and reason:"}
	}
	for i := 0; i < len(node.Content); i += 2 {
		if name := node.Content[i].Value; name != "forbid" && name != "reason" {
			return false, &Error{
				Analyzer: "nolint", Key: name,
				Problem: "no setting has this name — the block takes forbid: and reason:",
			}
		}
	}
	var block nolintBlock
	if err := node.Decode(&block); err != nil {
		return false, &Error{Analyzer: "nolint", Problem: oneLine(err)}
	}
	if block.Forbid && strings.TrimSpace(block.Reason) == "" {
		return false, &Error{Analyzer: "nolint", Key: "forbid", Problem: "the entry has no " +
			"reason — add reason: with why a reviewer accepted it"}
	}
	return block.Forbid, nil
}

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
			if directoryFlag(registered.Flags.Lookup(flagName).Value) {
				rewritten, err := f.directory(key, entry)
				if err != nil {
					return err
				}
				entry = rewritten
			}
			f.entries[key] = append(f.entries[key], entry)
		}
	}
	return nil
}

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

type pattern string

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

func (p pattern) matches(rel string) bool {
	if p == "..." {
		return true
	}
	if prefix, found := strings.CutSuffix(string(p), "/..."); found {
		return rel == prefix || strings.HasPrefix(rel, prefix+"/")
	}
	return string(p) == rel
}

func byName(analyzers []*analysis.Analyzer, name string) *analysis.Analyzer {
	for _, analyzer := range analyzers {
		if analyzer.Name == name {
			return analyzer
		}
	}
	return nil
}

func oneLine(err error) string {
	return strings.Join(strings.Fields(err.Error()), " ")
}

var installed *File

func Install(file *File) {
	installed = file
}

func Installed() bool {
	return installed != nil
}

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

func NolintForbidden() bool {
	return installed != nil && installed.nolint
}
