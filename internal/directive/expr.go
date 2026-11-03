package directive

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/kapetan-io/tiger/assert"
)

type AtomKind int

const (
	AtomNumber AtomKind = iota
	AtomRef
	AtomLen
	AtomNil
)

type Atom struct {
	Kind AtomKind
	Int  int64
	Ref  string
}

type VariantTerm struct {
	Minus bool
	Atom  Atom
}

type Variant struct {
	Terms []VariantTerm
}

type Predicate struct {
	Invariant string
	Op        string
	Left      Atom
	Right     Atom
}

var comparisonOps = []string{"==", "!=", "<=", ">=", "<", ">"}

func ParseVariant(args string) (Variant, error) {
	trimmed := strings.TrimSpace(args)
	if trimmed == "" {
		return Variant{}, errors.New(
			"has nothing after it — write the expression that shrinks on every pass of the loop, " +
				"for example //tiger:variant high - low")
	}
	parsed := Variant{}
	minus := false
	for i, segment := range splitTerms(trimmed) {
		if i%2 == 1 {
			minus = segment == "-"
			continue
		}
		if segment == "" {
			return Variant{}, errors.New("starts or ends with an operator — remove it")
		}
		atom, err := parseAtom(segment, variantLanguage)
		if err != nil {
			return Variant{}, err
		}
		parsed.Terms = append(parsed.Terms, VariantTerm{Minus: minus, Atom: atom})
	}
	return parsed, nil
}

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

type language struct {
	name     string
	allowNil bool
	atoms    string
	hint     string
}

var (
	variantLanguage = language{
		name:  "variant",
		atoms: "an integer, a path, or len(path)",
		hint:  ", joined by + or -",
	}
	predicateLanguage = language{
		name:     "comparison",
		allowNil: true,
		atoms:    "an integer, nil, a path, or len(path)",
	}
)

func parseAtom(text string, lang language) (Atom, error) {
	outside := fmt.Errorf("has %q, which is not %s — rewrite the %s using only those%s",
		text, lang.atoms, lang.name, lang.hint)
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
			return Atom{}, fmt.Errorf("has len(%s), whose path does not start at a parameter "+
				"or receiver — write one, for example len(r.items)", split.inner[0])
		}
		return Atom{Kind: AtomLen, Ref: split.inner[0]}, nil
	}
	if !validPath(text) {
		return Atom{}, outside
	}
	return Atom{Kind: AtomRef, Ref: text}, nil
}

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

func ParsePredicate(args string) (Predicate, error) {
	trimmed := strings.TrimSpace(args)
	if trimmed == "" {
		return Predicate{}, errors.New(
			"has nothing after it — write a comparison over nil, integers, or lengths, or name " +
				"an invariant ID, for example n > 0")
	}
	found, ok := findOperator(trimmed)
	if !ok {
		if !validPath(trimmed) {
			return Predicate{}, fmt.Errorf(
				"has %q, which is neither a comparison nor an invariant ID — write one, "+
					"for example n > 0", trimmed)
		}
		return Predicate{Invariant: trimmed}, nil
	}
	leftText := strings.TrimSpace(trimmed[:found.at])
	rightText := strings.TrimSpace(trimmed[found.at+len(found.op):])
	if leftText == "" || rightText == "" {
		return Predicate{}, errors.New(
			"is missing one side of the comparison — write both sides, for example n > 0")
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
		return Predicate{}, errors.New("compares nil against nil, which says nothing — " +
			"compare a parameter or result against nil")
	}
	ordered := found.op != "==" && found.op != "!="
	if ordered {
		if left.Kind == AtomNil || right.Kind == AtomNil {
			return Predicate{}, errors.New(
				"orders nil with < or >, but nil supports only == and != — use one of those")
		}
	}
	return Predicate{Op: found.op, Left: left, Right: right}, nil
}

type operatorMatch struct {
	op string
	at int
}

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

func FormatPredicate(parsed Predicate) string {
	if parsed.Invariant != "" {
		return parsed.Invariant
	}
	return formatAtom(parsed.Left) + " " + parsed.Op + " " + formatAtom(parsed.Right)
}

func splitCommas(text string) []string {
	elements := []string{}
	depth, start := 0, 0
	for i, r := range text {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				elements = append(elements, strings.TrimSpace(text[start:i]))
				start = i + 1
			}
		}
	}
	return append(elements, strings.TrimSpace(text[start:]))
}

type call struct {
	name  string
	inner []string
}

func splitCall(term string) (call, error) {
	open := strings.IndexByte(term, '(')
	if open < 0 {
		if strings.ContainsRune(term, ')') {
			return call{}, fmt.Errorf("has an unmatched %q in %q — balance the parentheses",
				")", term)
		}
		return call{name: term}, nil
	}
	if !strings.HasSuffix(term, ")") {
		return call{}, fmt.Errorf("has an unclosed %q in %q — close it", "(", term)
	}
	split := call{name: strings.TrimSpace(term[:open])}
	body := term[open+1 : len(term)-1]
	if strings.TrimSpace(body) == "" {
		split.inner = []string{}
		return split, nil
	}
	split.inner = splitCommas(body)
	return split, nil
}

func validPath(text string) bool {
	for segment := range strings.SplitSeq(text, ".") {
		if !validIdentifier(segment) {
			return false
		}
	}
	return true
}

func validIdentifier(text string) bool {
	if text == "" {
		return false
	}
	for i, r := range text {
		letter := unicode.IsLetter(r) || r == '_'
		if i == 0 && !letter {
			return false
		}
		if !letter && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
