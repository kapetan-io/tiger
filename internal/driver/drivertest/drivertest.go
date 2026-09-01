// Package drivertest runs a whole-program rule's module corpus through the
// tiger driver and checks its // want expectations.
//
// analysistest cannot run a finish step, so each whole-program rule's
// corpus is a small module — its own go.mod, two or more packages — that
// only driver.Check can exercise end to end. Expectations use the same
// comment form analysistest reads: // want "regexp" on the line the
// finding lands, one quoted pattern per expected diagnostic.
package drivertest

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/kapetan-io/tiger/internal/driver"
	"github.com/kapetan-io/tiger/internal/finish"
)

// Reporter is the subset of testing.TB the harness reports through, so a
// caller that only wants the findings can pass a silent implementation.
type Reporter interface {
	Errorf(format string, args ...any)
}

// expectation is one // want pattern: where it sits and what it matches.
type expectation struct {
	file    string
	line    int
	pattern *regexp.Regexp
	matched bool
}

// wantPattern finds the expectation list after "want" in a comment.
var wantPattern = regexp.MustCompile(`\bwant\s+(.*)$`)

// Run runs finisher's analyzer — and its finish function, when set — over
// the module at root through driver.Check, reports every mismatch between
// findings and // want expectations through t, and returns every finding
// so a caller can check messages against the registry and the style
// guide. A driver error is reported through t and yields no findings.
func Run(t Reporter, root string, finisher finish.Finisher) []driver.Finding {
	finishers := []finish.Finisher{}
	if finisher.Run != nil {
		finishers = append(finishers, finisher)
	}
	findings, err := driver.Check(
		root, []string{"./..."}, []*analysis.Analyzer{finisher.Analyzer}, finishers,
	)
	if err != nil {
		t.Errorf("drivertest: %s: %v", root, err)
		return nil
	}
	expectations, err := collect(root)
	if err != nil {
		t.Errorf("drivertest: %s: %v", root, err)
		return findings
	}
	for _, finding := range findings {
		if !match(expectations, finding) {
			t.Errorf("drivertest: %s: unexpected diagnostic at %s: %s",
				root, finding.Position, finding.Message)
		}
	}
	for _, want := range expectations {
		if !want.matched {
			t.Errorf("drivertest: %s: missing diagnostic at %s:%d matching %q",
				root, want.file, want.line, want.pattern)
		}
	}
	return findings
}

// match marks the first unmatched expectation on finding's line whose
// pattern matches its message.
func match(expectations []*expectation, finding driver.Finding) bool {
	for _, want := range expectations {
		if want.matched || want.file != finding.Position.Filename ||
			want.line != finding.Position.Line {
			continue
		}
		if want.pattern.MatchString(finding.Message) {
			want.matched = true
			return true
		}
	}
	return false
}

// collect parses every Go file under root for // want comments.
func collect(root string) ([]*expectation, error) {
	expectations := []*expectation{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return walkErr
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		for _, group := range file.Comments {
			for _, comment := range group.List {
				found, err := parseWant(comment.Text)
				if err != nil {
					return err
				}
				for _, pattern := range found {
					expectations = append(expectations, &expectation{
						file:    filepath.ToSlash(relative),
						line:    fset.Position(comment.Pos()).Line,
						pattern: pattern,
					})
				}
			}
		}
		return nil
	})
	return expectations, err
}

// parseWant reads the quoted patterns after "want" in one comment, if any.
func parseWant(text string) ([]*regexp.Regexp, error) {
	located := wantPattern.FindStringSubmatch(text)
	if located == nil {
		return nil, nil
	}
	patterns := []*regexp.Regexp{}
	rest := strings.TrimSpace(located[1])
	// Every pass consumes at least one byte of rest, so the comment's
	// length bounds the loop.
	for range len(rest) {
		if rest == "" {
			break
		}
		end, err := quotedEnd(rest)
		if err != nil {
			return nil, err
		}
		quoted, err := strconv.Unquote(rest[:end])
		if err != nil {
			return nil, err
		}
		pattern, err := regexp.Compile(quoted)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, pattern)
		rest = strings.TrimSpace(rest[end:])
	}
	return patterns, nil
}

// quotedEnd returns the index just past the Go string literal at the front
// of text.
func quotedEnd(text string) (int, error) {
	switch text[0] {
	case '`':
		end := strings.IndexByte(text[1:], '`')
		if end < 0 {
			return 0, &parseError{text}
		}
		return end + 2, nil
	case '"':
		escaped := false
		for i := 1; i < len(text); i++ {
			if escaped {
				escaped = false
				continue
			}
			if text[i] == '\\' {
				escaped = true
				continue
			}
			if text[i] == '"' {
				return i + 1, nil
			}
		}
		return 0, &parseError{text}
	default:
		return 0, &parseError{text}
	}
}

// parseError reports a // want comment whose patterns are not quoted.
type parseError struct {
	text string
}

func (e *parseError) Error() string {
	return "want expectation is not a quoted pattern: " + e.text
}
