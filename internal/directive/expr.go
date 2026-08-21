package directive

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/kapetan-io/tiger/assert"
)

// AtomKind classifies one operand of the variant and contracts languages.
type AtomKind int

const (
	// AtomNumber is an integer literal.
	AtomNumber AtomKind = iota
	// AtomRef is a dot-separated path over locals, parameters, or results.
	AtomRef
	// AtomLen is len(path), the one call the languages admit.
	AtomLen
	// AtomNil is the nil keyword; contracts only.
	AtomNil
)

// Atom is one operand: an integer, a path, len(path), or nil.
type Atom struct {
	Kind AtomKind
	Int  int64
	Ref  string
}

// VariantTerm is one term of a variant expression with its leading sign;
// the first term of a variant is always positive.
type VariantTerm struct {
	Minus bool
	Atom  Atom
}

// Variant is a linear integer ranking expression: a signed sum of integer
// literals, paths, and len() calls. Term order is preserved — it is
// semantic, not presentational.
type Variant struct {
	Terms []VariantTerm
}

// Predicate is one contracts predicate: either a comparison of two atoms or
// a bare invariant ID reference.
type Predicate struct {
	Invariant string
	Op        string
	Left      Atom
	Right     Atom
}

// comparisonOps lists the admitted operators, two-character forms first so
// the scanner never splits <= into < and a stray =.
var comparisonOps = []string{"==", "!=", "<=", ">=", "<", ">"}

// ParseVariant reads a variant pin's argument text into its term list.
func ParseVariant(args string) (Variant, error) {
	trimmed := strings.TrimSpace(args)
	if trimmed == "" {
		return Variant{}, errors.New(
			"a variant pin states a fact — write the loop's ranking expression")
	}
	parsed := Variant{}
	minus := false
	// splitTerms alternates atom text and operator: even positions are
	// atoms, odd positions the sign that precedes the next one.
	for i, segment := range splitTerms(trimmed) {
		if i%2 == 1 {
			minus = segment == "-"
			continue
		}
		if segment == "" {
			return Variant{}, errors.New("the variant ends in an operator or starts with one")
		}
		atom, err := parseAtom(segment, variantLanguage)
		if err != nil {
			return Variant{}, err
		}
		parsed.Terms = append(parsed.Terms, VariantTerm{Minus: minus, Atom: atom})
	}
	return parsed, nil
}

// splitTerms splits a variant expression on top-level + and -, returning
// alternating atoms and the operators between them.
func splitTerms(text string) []string {
	segments := []string{}
	depth, start := 0, 0
	for i, r := range text {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case '+', '-':
			if depth == 0 {
				segments = append(segments, strings.TrimSpace(text[start:i]), string(r))
				start = i + 1
			}
		}
	}
	return append(segments, strings.TrimSpace(text[start:]))
}

// language names which atom set a parse admits, for error text.
type language struct {
	name     string
	allowNil bool
	atoms    string
}

var (
	variantLanguage = language{
		name:  "variant",
		atoms: "an integer, a path, or len(path)",
	}
	predicateLanguage = language{
		name:     "predicate",
		allowNil: true,
		atoms:    "an integer, nil, a path, or len(path)",
	}
)

// parseAtom reads one operand.
func parseAtom(text string, lang language) (Atom, error) {
	outside := fmt.Errorf("atom %q is outside the %s language (%s)", text, lang.name, lang.atoms)
	if text == "nil" {
		if !lang.allowNil {
			return Atom{}, outside
		}
		return Atom{Kind: AtomNil}, nil
	}
	if number, err := strconv.ParseInt(text, 10, 64); err == nil {
		return Atom{Kind: AtomNumber, Int: number}, nil
	}
	split, err := splitCall(text)
	if err != nil {
		return Atom{}, err
	}
	if split.inner != nil {
		if split.name != "len" || len(split.inner) != 1 {
			return Atom{}, outside
		}
		if !validPath(split.inner[0]) {
			return Atom{}, fmt.Errorf("len path %q is not a parameter-rooted path", split.inner[0])
		}
		return Atom{Kind: AtomLen, Ref: split.inner[0]}, nil
	}
	if !validPath(text) {
		return Atom{}, outside
	}
	return Atom{Kind: AtomRef, Ref: text}, nil
}

// formatAtom prints one operand canonically.
func formatAtom(atom Atom) string {
	printed := ""
	switch atom.Kind {
	case AtomNumber:
		printed = strconv.FormatInt(atom.Int, 10)
	case AtomRef:
		printed = atom.Ref
	case AtomLen:
		printed = "len(" + atom.Ref + ")"
	case AtomNil:
		printed = "nil"
	default:
		assert.Unreachable("AtomKind: unhandled value")
	}
	return printed
}

// FormatVariant prints a variant canonically: terms joined by their signs
// with single spaces.
func FormatVariant(parsed Variant) string {
	printed := &strings.Builder{}
	for i, term := range parsed.Terms {
		if i > 0 {
			if term.Minus {
				printed.WriteString(" - ")
			} else {
				printed.WriteString(" + ")
			}
		}
		printed.WriteString(formatAtom(term.Atom))
	}
	return printed.String()
}

// ParsePredicate reads a requires or ensures pin's argument text: a
// comparison of two atoms, or a bare invariant ID.
func ParsePredicate(args string) (Predicate, error) {
	trimmed := strings.TrimSpace(args)
	if trimmed == "" {
		return Predicate{}, errors.New(
			"a contract pin states a fact — write a predicate over nil-ness, integer " +
				"ranges, length relations, or invariant IDs")
	}
	found, ok := findOperator(trimmed)
	if !ok {
		if !validPath(trimmed) {
			return Predicate{}, fmt.Errorf(
				"%q — a predicate compares atoms or names an invariant ID", trimmed)
		}
		return Predicate{Invariant: trimmed}, nil
	}
	leftText := strings.TrimSpace(trimmed[:found.at])
	rightText := strings.TrimSpace(trimmed[found.at+len(found.op):])
	if leftText == "" || rightText == "" {
		return Predicate{}, errors.New("the comparison is missing an atom")
	}
	left, err := parseAtom(leftText, predicateLanguage)
	if err != nil {
		return Predicate{}, err
	}
	right, err := parseAtom(rightText, predicateLanguage)
	if err != nil {
		return Predicate{}, err
	}
	if left.Kind == AtomNil && right.Kind == AtomNil {
		return Predicate{}, errors.New("a comparison of nil against nil compares nothing")
	}
	ordered := found.op != "==" && found.op != "!="
	if ordered {
		if left.Kind == AtomNil || right.Kind == AtomNil {
			return Predicate{}, errors.New("nil supports only == and != comparisons")
		}
	}
	return Predicate{Op: found.op, Left: left, Right: right}, nil
}

// operatorMatch is one located comparison operator and its byte offset.
type operatorMatch struct {
	op string
	at int
}

// findOperator locates the first top-level comparison operator.
func findOperator(text string) (operatorMatch, bool) {
	depth := 0
	for i := range len(text) {
		switch text[i] {
		case '(':
			depth++
		case ')':
			depth--
		}
		if depth != 0 {
			continue
		}
		for _, candidate := range comparisonOps {
			if strings.HasPrefix(text[i:], candidate) {
				return operatorMatch{op: candidate, at: i}, true
			}
		}
	}
	return operatorMatch{}, false
}

// FormatPredicate prints a predicate canonically: the invariant ID alone,
// or the two atoms around their operator with single spaces.
func FormatPredicate(parsed Predicate) string {
	if parsed.Invariant != "" {
		return parsed.Invariant
	}
	return formatAtom(parsed.Left) + " " + parsed.Op + " " + formatAtom(parsed.Right)
}
