package directive

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// EffectSet is the closed effect lattice in canonical order (invariant 2).
// The zero value is purity, spelled none in pin syntax. IO holds built-in
// tier qualifiers and Mutate holds parameter-rooted paths, both sorted and
// deduplicated so structural equality is set equality.
type EffectSet struct {
	Alloc  bool
	IO     []string
	Block  bool
	Panic  bool
	Rand   bool
	Time   bool
	Mutate []string
	Spawn  bool
}

// ioQualifiers is the built-in io tier; the declared tier (io(database))
// arrives with the surfaces analyzer in a later wave.
var ioQualifiers = []string{"disk", "env", "exec", "net"}

// ParseEffects reads an effects pin's argument text into an EffectSet.
// Anything outside the closed lattice is an error naming the offending
// token — a misspelled qualifier is never a silently meaningless string.
func ParseEffects(args string) (EffectSet, error) {
	trimmed := strings.TrimSpace(args)
	if trimmed == "" {
		return EffectSet{}, errors.New("an effects pin states a fact — write none for purity")
	}
	if trimmed == "none" {
		return EffectSet{}, nil
	}
	set := EffectSet{}
	for _, term := range splitCommas(trimmed) {
		if term == "" {
			return EffectSet{}, errors.New("empty effect term")
		}
		if term == "none" {
			return EffectSet{}, errors.New(
				`"none" states purity and cannot combine with other effects`)
		}
		if err := parseEffectTerm(&set, term); err != nil {
			return EffectSet{}, err
		}
	}
	set.IO = normalize(set.IO)
	set.Mutate = normalize(set.Mutate)
	return set, nil
}

// parseEffectTerm folds one comma-separated term into set.
func parseEffectTerm(set *EffectSet, term string) error {
	split, err := splitCall(term)
	if err != nil {
		return err
	}
	switch split.name {
	case "alloc", "block", "panic", "rand", "time", "spawn":
		if split.inner != nil {
			return fmt.Errorf("%q takes no arguments", split.name)
		}
		markBare(set, split.name)
	case "io":
		if len(split.inner) == 0 {
			return errors.New("io requires a qualifier: io(disk), io(env), io(exec), io(net)")
		}
		for _, qualifier := range split.inner {
			if !slices.Contains(ioQualifiers, qualifier) {
				return fmt.Errorf("io qualifier %q is not in the built-in tier (%s)",
					qualifier, strings.Join(ioQualifiers, ", "))
			}
		}
		set.IO = append(set.IO, split.inner...)
	case "mutate":
		if len(split.inner) == 0 {
			return errors.New("mutate requires a location: mutate(r.log)")
		}
		for _, location := range split.inner {
			if !validPath(location) {
				return fmt.Errorf("mutate location %q is not a parameter-rooted path", location)
			}
		}
		set.Mutate = append(set.Mutate, split.inner...)
	default:
		return fmt.Errorf(
			"unknown effect %q — the lattice is closed (alloc, io, block, panic, rand, "+
				"time, mutate, spawn)", split.name)
	}
	return nil
}

// markBare sets the lattice member named by a bare (unparameterized) term.
func markBare(set *EffectSet, name string) {
	switch name {
	case "alloc":
		set.Alloc = true
	case "block":
		set.Block = true
	case "panic":
		set.Panic = true
	case "rand":
		set.Rand = true
	case "time":
		set.Time = true
	case "spawn":
		set.Spawn = true
	}
}

// FormatEffects prints a set canonically: lattice order, sorted qualifier
// and location lists, none for purity. This is the byte form a pin freezes.
func FormatEffects(set EffectSet) string {
	terms := []string{}
	if set.Alloc {
		terms = append(terms, "alloc")
	}
	if len(set.IO) > 0 {
		terms = append(terms, "io("+strings.Join(normalize(set.IO), ", ")+")")
	}
	if set.Block {
		terms = append(terms, "block")
	}
	if set.Panic {
		terms = append(terms, "panic")
	}
	if set.Rand {
		terms = append(terms, "rand")
	}
	if set.Time {
		terms = append(terms, "time")
	}
	if len(set.Mutate) > 0 {
		terms = append(terms, "mutate("+strings.Join(normalize(set.Mutate), ", ")+")")
	}
	if set.Spawn {
		terms = append(terms, "spawn")
	}
	if len(terms) == 0 {
		return "none"
	}
	return strings.Join(terms, ", ")
}

// Empty reports purity: no member of the lattice is present.
func (set EffectSet) Empty() bool {
	return !set.Alloc && !set.Block && !set.Panic && !set.Rand && !set.Time && !set.Spawn &&
		len(set.IO) == 0 && len(set.Mutate) == 0
}

// Equal is exact set equality — the comparison bidirectional pin
// enforcement runs in both directions.
func (set EffectSet) Equal(other EffectSet) bool {
	return set.Alloc == other.Alloc && set.Block == other.Block &&
		set.Panic == other.Panic && set.Rand == other.Rand &&
		set.Time == other.Time && set.Spawn == other.Spawn &&
		slices.Equal(normalize(set.IO), normalize(other.IO)) &&
		slices.Equal(normalize(set.Mutate), normalize(other.Mutate))
}

// Union folds two sets into one.
func (set EffectSet) Union(other EffectSet) EffectSet {
	return EffectSet{
		Alloc:  set.Alloc || other.Alloc,
		Block:  set.Block || other.Block,
		Panic:  set.Panic || other.Panic,
		Rand:   set.Rand || other.Rand,
		Time:   set.Time || other.Time,
		Spawn:  set.Spawn || other.Spawn,
		IO:     normalize(slices.Concat(set.IO, other.IO)),
		Mutate: normalize(slices.Concat(set.Mutate, other.Mutate)),
	}
}

// Diff returns the members of set absent from other: with the arguments one
// way it is the undeclared computed effects, the other way the
// declared-but-absent ones.
func (set EffectSet) Diff(other EffectSet) EffectSet {
	missing := func(have, exclude []string) []string {
		kept := []string{}
		for _, member := range normalize(have) {
			if !slices.Contains(exclude, member) {
				kept = append(kept, member)
			}
		}
		return kept
	}
	diffed := EffectSet{
		Alloc:  set.Alloc && !other.Alloc,
		Block:  set.Block && !other.Block,
		Panic:  set.Panic && !other.Panic,
		Rand:   set.Rand && !other.Rand,
		Time:   set.Time && !other.Time,
		Spawn:  set.Spawn && !other.Spawn,
		IO:     missing(set.IO, other.IO),
		Mutate: missing(set.Mutate, other.Mutate),
	}
	if len(diffed.IO) == 0 {
		diffed.IO = nil
	}
	if len(diffed.Mutate) == 0 {
		diffed.Mutate = nil
	}
	return diffed
}

// ParseFrame reads a frame pin's argument text into a sorted location list;
// none is the empty frame.
func ParseFrame(args string) ([]string, error) {
	trimmed := strings.TrimSpace(args)
	if trimmed == "" {
		return nil, errors.New("a frame pin states a fact — write none for the empty frame")
	}
	if trimmed == "none" {
		return nil, nil
	}
	locations := []string{}
	for _, location := range splitCommas(trimmed) {
		if location == "" {
			return nil, errors.New("empty location")
		}
		if location == "none" {
			return nil, errors.New(
				`"none" states the empty frame and cannot combine with locations`)
		}
		if !validPath(location) {
			return nil, fmt.Errorf("frame location %q is not a parameter-rooted path", location)
		}
		locations = append(locations, location)
	}
	return normalize(locations), nil
}

// FormatFrame prints a location list canonically: sorted, comma-separated,
// none for the empty frame.
func FormatFrame(locations []string) string {
	if len(locations) == 0 {
		return "none"
	}
	return strings.Join(normalize(locations), ", ")
}

// splitCommas splits text on commas outside parentheses, trimming each
// element. Empty elements are preserved so callers can name them.
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

// call is one term split into its name and, when parenthesized, its
// comma-split inner arguments; a bare term carries nil inner.
type call struct {
	name  string
	inner []string
}

// splitCall splits one term into its call form.
func splitCall(term string) (call, error) {
	open := strings.IndexByte(term, '(')
	if open < 0 {
		if strings.ContainsRune(term, ')') {
			return call{}, fmt.Errorf("unbalanced %q in %q", ")", term)
		}
		return call{name: term}, nil
	}
	if !strings.HasSuffix(term, ")") {
		return call{}, fmt.Errorf("unclosed %q in %q", "(", term)
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

// validPath reports whether text is a parameter-rooted path: one or more
// dot-separated Go identifiers.
func validPath(text string) bool {
	for segment := range strings.SplitSeq(text, ".") {
		if !validIdentifier(segment) {
			return false
		}
	}
	return true
}

// validIdentifier reports whether text is a Go identifier.
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

// normalize sorts and deduplicates a qualifier or location list,
// returning nil for an empty one so zero values stay comparable.
func normalize(list []string) []string {
	if len(list) == 0 {
		return nil
	}
	sorted := slices.Clone(list)
	slices.Sort(sorted)
	return slices.Compact(sorted)
}
