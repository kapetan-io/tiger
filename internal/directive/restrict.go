package directive

import (
	"errors"
	"fmt"
	"strings"
)

// Restriction is a parsed //tiger:restrict declaration: the package's
// claimed axes. Each axis is claimed or unclaimed; an unclaimed axis takes
// the specification's default and is never checked (TS-P01). Imports holds
// the module-relative path patterns, sorted and deduplicated; an empty
// Imports means the axis is unclaimed, since imports() with no paths is a
// parse error.
type Restriction struct {
	ClosedDispatch bool
	NoReflect      bool
	Imports        []string
}

// Subtree is the pattern suffix that matches every package under a path.
const Subtree = "/..."

// ParseRestrict reads a restrict directive's argument text into a
// Restriction. Anything outside the three axes is an error naming the
// offending token, so a misspelled axis is never a silently meaningless
// string.
func ParseRestrict(args string) (Restriction, error) {
	trimmed := strings.TrimSpace(args)
	if trimmed == "" {
		return Restriction{}, errors.New("has nothing after it — list at least one of " +
			"closed-dispatch, no-reflect, imports(...)")
	}
	set := Restriction{}
	for _, term := range splitCommas(trimmed) {
		if term == "" {
			return Restriction{}, errors.New(
				"has an empty item between commas — remove the extra comma")
		}
		if err := parseRestrictTerm(&set, term); err != nil {
			return Restriction{}, err
		}
	}
	set.Imports = normalize(set.Imports)
	return set, nil
}

// parseRestrictTerm folds one comma-separated term into set.
func parseRestrictTerm(set *Restriction, term string) error {
	split, err := splitCall(term)
	if err != nil {
		return err
	}
	switch split.name {
	case "closed-dispatch", "no-reflect":
		return markAxis(set, split)
	case "imports":
		if split.inner == nil {
			return errors.New("has a bare imports — name the allowed paths, for example " +
				"imports(internal/domain/...)")
		}
		if len(split.inner) == 0 {
			return errors.New("has an empty imports() — name at least one path, for example " +
				"imports(internal/domain/...)")
		}
		if len(set.Imports) > 0 {
			return errors.New("lists imports(...) twice — merge the paths into one list")
		}
		for _, path := range split.inner {
			if !validImportPattern(path) {
				return fmt.Errorf("has import path %q, which is not a module-relative path — "+
					"write one like internal/domain, or internal/domain/... to allow a subtree",
					path)
			}
		}
		set.Imports = append(set.Imports, split.inner...)
	default:
		return fmt.Errorf("names %q, which is not a restriction tiger knows — use only "+
			"closed-dispatch, no-reflect, imports(...)", split.name)
	}
	return nil
}

// markAxis sets one of the two bare axes, rejecting arguments and repeats.
func markAxis(set *Restriction, split call) error {
	if split.inner != nil {
		return fmt.Errorf("has %s with arguments, but it takes none — write it bare, as %s",
			split.name, split.name)
	}
	claimed := &set.NoReflect
	if split.name == "closed-dispatch" {
		claimed = &set.ClosedDispatch
	}
	if *claimed {
		return fmt.Errorf("lists %s twice — write it once", split.name)
	}
	*claimed = true
	return nil
}

// validImportPattern reports whether text is a clean module-relative
// import path, optionally ending in the subtree suffix: one or more
// non-empty segments with no leading slash, no "." or ".." segment, and no
// whitespace or parentheses.
func validImportPattern(text string) bool {
	trimmed := strings.TrimSuffix(text, Subtree)
	if trimmed == "" || strings.ContainsAny(trimmed, " \t()") {
		return false
	}
	for segment := range strings.SplitSeq(trimmed, "/") {
		if segment == "" || segment == "." || segment == ".." || segment == "..." {
			return false
		}
	}
	return true
}

// FormatRestrict prints a restriction canonically — the axes in the order
// closed-dispatch, no-reflect, imports, with sorted paths. This is the byte
// form a declaration takes.
func FormatRestrict(set Restriction) string {
	terms := []string{}
	if set.ClosedDispatch {
		terms = append(terms, "closed-dispatch")
	}
	if set.NoReflect {
		terms = append(terms, "no-reflect")
	}
	if len(set.Imports) > 0 {
		terms = append(terms, "imports("+strings.Join(normalize(set.Imports), ", ")+")")
	}
	return strings.Join(terms, ", ")
}
