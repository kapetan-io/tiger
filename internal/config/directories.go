package config

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
)

type Directories struct {
	Name string
}

var _ flag.Value = Directories{Name: ""}

func (d Directories) String() string { return "" }

func (d Directories) Set(string) error {
	return errors.New(d.Name + " is set only in " + FileName + ", each directory with a reason")
}

func directoryFlag(value flag.Value) bool {
	_, found := value.(Directories)
	return found
}

func (f *File) directory(key Key, entry Entry) (Entry, error) {
	fail := func(problem string) (Entry, error) {
		return Entry{}, &Error{
			Analyzer: key.Analyzer, Key: key.Flag, Value: entry.Value, Problem: problem,
		}
	}
	if len(entry.Packages) > 0 {
		return fail("the entry has packages: — a directory covers its whole subtree, " +
			"so remove packages:")
	}
	value := entry.Value
	if value != "." {
		if strings.Contains(value, "...") || !pattern(value).wellFormed() {
			return fail("the value is not a module-relative directory — write a path like " +
				"internal/legacy, with no ..., .., leading slash or trailing slash, " +
				"or . for the whole module")
		}
		info, err := os.Stat(filepath.Join(f.root, filepath.FromSlash(value)))
		if err != nil || !info.IsDir() {
			return fail("no directory has this path in the module — " +
				"fix the path or remove the entry")
		}
		if nested := f.nestedModule(value); nested != "" {
			return fail("the directory is inside the nested module at " + nested +
				", which this run does not check — remove the entry")
		}
	}
	scope := "..."
	if value != "." {
		scope = value + "/..."
	}
	entry.Packages = []string{scope}
	return entry, nil
}

func (f *File) nestedModule(rel string) string {
	walked := ""
	for _, segment := range strings.Split(rel, "/") {
		walked = filepath.Join(walked, segment)
		if holdsModule(filepath.Join(f.root, walked)) {
			return filepath.ToSlash(walked)
		}
	}
	return ""
}

func holdsModule(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}
