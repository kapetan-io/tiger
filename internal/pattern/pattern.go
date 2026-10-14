// Package pattern reads the pattern collection: for each directory, the
// PATTERN.md card (the rules it backs, the gap, the bug behind it, the
// reviewer checklist) and the Go files that show the right shape and prove
// both shapes. The tiger binary prints these and the registry meta-tests
// check them against the rules.
package pattern

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// CardFile names the card inside each pattern directory.
const CardFile = "PATTERN.md"

const (
	rulesHeading     = "Rules"
	gapHeading       = "Gap"
	bugHeading       = "Bug"
	checklistHeading = "Checklist"
)

var ruleCode = regexp.MustCompile(`^TS-[A-Z]+[0-9]+$`)

// File is one Go source file of a pattern, path relative to the pattern.
type File struct {
	Path   string
	Source string
	// Test is true for _test.go files.
	Test bool
}

// Pattern is one entry of the collection.
type Pattern struct {
	// Name is the directory name, the argument to tiger pattern.
	Name string
	// Summary is the card's opening paragraph.
	Summary string
	// Rules are the rule codes the pattern backs.
	Rules []string
	// Gap is the part of the correct shape those rules cannot check.
	Gap string
	// Bug is the real bug that lived in the gap.
	Bug string
	// Checklist holds the reviewer's yes-or-no questions.
	Checklist []string
	// Files are the pattern's Go files, sorted by path.
	Files []File
}

// Load reads every pattern directory under the root of fsys. A directory
// without a card, or with a malformed card, is an error: a pattern that the
// binary cannot print must not ship.
func Load(fsys fs.FS) ([]Pattern, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	loaded := []Pattern{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		each, err := read(fsys, entry.Name())
		if err != nil {
			return nil, err
		}
		loaded = append(loaded, each)
	}
	return loaded, nil
}

// Find returns the pattern with the given name.
func Find(all []Pattern, name string) (Pattern, bool) {
	for _, each := range all {
		if each.Name == name {
			return each, true
		}
	}
	return Pattern{}, false
}

// Behind returns the patterns that back the rule code, ordered by name.
func Behind(all []Pattern, code string) []Pattern {
	found := []Pattern{}
	for _, each := range all {
		for _, backed := range each.Rules {
			if backed == code {
				found = append(found, each)
			}
		}
	}
	return found
}

func read(fsys fs.FS, name string) (Pattern, error) {
	card, err := fs.ReadFile(fsys, path.Join(name, CardFile))
	if err != nil {
		return Pattern{}, fmt.Errorf("pattern %s: %w", name, err)
	}
	parsed, err := parseCard(string(card))
	if err != nil {
		return Pattern{}, fmt.Errorf("pattern %s: %s: %w", name, CardFile, err)
	}
	parsed.Name = name
	files, err := goFiles(fsys, name)
	if err != nil {
		return Pattern{}, fmt.Errorf("pattern %s: %w", name, err)
	}
	if len(files) == 0 {
		return Pattern{}, fmt.Errorf("pattern %s: no Go files", name)
	}
	parsed.Files = files
	return parsed, nil
}

func goFiles(fsys fs.FS, name string) ([]File, error) {
	files := []File{}
	err := fs.WalkDir(fsys, name, func(each string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(each, ".go") {
			return err
		}
		source, err := fs.ReadFile(fsys, each)
		if err != nil {
			return err
		}
		files = append(files, File{
			Path:   strings.TrimPrefix(each, name+"/"),
			Source: string(source),
			Test:   strings.HasSuffix(each, "_test.go"),
		})
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, err
}

// parseCard splits the card into its title paragraph and its four
// sections.
func parseCard(card string) (Pattern, error) {
	sections := map[string][]string{}
	current := ""
	summary := []string{}
	for _, line := range strings.Split(card, "\n") {
		heading, isHeading := strings.CutPrefix(line, "## ")
		switch {
		case isHeading:
			current = strings.TrimSpace(heading)
			sections[current] = []string{}
		case strings.HasPrefix(line, "# "):
		case current == "":
			summary = append(summary, line)
		default:
			sections[current] = append(sections[current], line)
		}
	}
	parsed := Pattern{Summary: strings.TrimSpace(strings.Join(summary, "\n"))}
	if parsed.Summary == "" {
		return Pattern{}, fmt.Errorf("no summary paragraph under the title")
	}
	for _, heading := range []string{rulesHeading, gapHeading, bugHeading, checklistHeading} {
		if strings.TrimSpace(strings.Join(sections[heading], "")) == "" {
			return Pattern{}, fmt.Errorf("section %q is missing or empty", heading)
		}
	}
	parsed.Gap = strings.TrimSpace(strings.Join(sections[gapHeading], "\n"))
	parsed.Bug = strings.TrimSpace(strings.Join(sections[bugHeading], "\n"))
	rulesText := strings.Join(sections[rulesHeading], " ")
	for _, field := range strings.FieldsFunc(rulesText, isSeparator) {
		if !ruleCode.MatchString(field) {
			return Pattern{}, fmt.Errorf("%q in the Rules section is not a rule code", field)
		}
		if slices.Contains(parsed.Rules, field) {
			return Pattern{}, fmt.Errorf("%q is listed twice in the Rules section", field)
		}
		parsed.Rules = append(parsed.Rules, field)
	}
	for _, line := range sections[checklistHeading] {
		trimmed := strings.TrimSpace(line)
		if item, isItem := strings.CutPrefix(trimmed, "- "); isItem {
			parsed.Checklist = append(parsed.Checklist, item)
		} else if trimmed != "" && len(parsed.Checklist) > 0 {
			last := len(parsed.Checklist) - 1
			parsed.Checklist[last] += " " + trimmed
		}
	}
	for _, item := range parsed.Checklist {
		if !strings.HasSuffix(item, "?") {
			return Pattern{}, fmt.Errorf("checklist item %q is not a question", item)
		}
	}
	if len(parsed.Checklist) == 0 {
		return Pattern{}, fmt.Errorf("the Checklist section has no \"- \" items")
	}
	return parsed, nil
}

func isSeparator(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == '\t' }
