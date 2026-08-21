// Package directive parses and prints the //tiger: source-directive
// namespace.
//
// This package owns the verb vocabulary: an unknown verb is an error, never a
// silently meaningless comment. It is the single formatting site for directive
// text — no analyzer or driver builds a //tiger: line by hand — which is what
// makes the round-trip contract enforceable: for every well-formed directive,
// Parse(Format(d)) == d. When later waves print computed facts, the same
// contract guarantees report output is byte-identical to pin syntax.
//
// Wave 1 validates verb identity and each verb's minimal argument shape (an
// escape needs a reason). Richer per-verb argument grammars — effect
// lattices, frame sets — belong to later waves, and this package is where
// they will live.
package directive

import (
	"errors"
	"strings"
	"unicode"
)

// Prefix introduces every tiger directive, with no space after the slashes,
// following Go's own directive convention (//go:build, //go:generate).
const Prefix = "//tiger:"

// Kind classifies a verb by lifecycle, per the specification's declaration
// model. Wave 1 has exactly one escape verb, batched — the only verb that
// surfaces as a standing advisory finding.
type Kind int

const (
	// KindPin marks a verb that freezes an analyzer-computed fact into a
	// blocking contract. Wave 1 validates pin verbs' well-formedness only;
	// enforcement arrives with the waves that compute the facts.
	KindPin Kind = iota
	// KindIntent marks a verb stating something no analyzer can compute;
	// stated first, enforced after.
	KindIntent
	// KindEscape marks a verb loosening one rule at one site, always with a
	// reason, surfaced as a standing advisory finding on every run.
	KindEscape
)

// Directive is one parsed //tiger: comment: a verb from the closed
// vocabulary and its raw argument text.
//
// A Directive is canonical when Args carries no leading or trailing
// whitespace; Parse always returns canonical directives and Format expects
// one, which is what makes the round trip exact.
type Directive struct {
	Verb string
	Args string
}

// Verb describes one entry in the closed vocabulary.
type Verb struct {
	Name string
	Kind Kind
}

// UnknownVerbError reports a //tiger: comment whose verb is not in the
// vocabulary. There is deliberately no unknown-verb loophole: a directive
// that does not parse is a blocking finding (TS-L09), not a comment.
type UnknownVerbError struct {
	Verb string
}

func (e *UnknownVerbError) Error() string {
	return "unknown directive verb " + strings.TrimSpace(Prefix+e.Verb)
}

// MissingReasonError reports an escape directive with no reason. An escape
// without a reason is a rule silently deleted (TS-L09).
type MissingReasonError struct {
	Verb string
}

func (e *MissingReasonError) Error() string {
	return Prefix + e.Verb + " requires a reason"
}

// MalformedArgsError reports a pin directive whose arguments fail the
// verb's grammar. The detail names the offending token, so the TS-L09
// finding can point at exactly what to fix (invariant 2: a name outside the
// closed lattice is a parse error, never a silently meaningless string).
type MalformedArgsError struct {
	Verb   string
	Detail string
}

func (e *MalformedArgsError) Error() string {
	return Prefix + e.Verb + ": " + e.Detail
}

// vocabulary is the closed verb set, in declaration-model order: the sole
// wave-1 escape, then the pin and intent verbs later waves enforce
// (validated for well-formedness from wave 1).
var vocabulary = []Verb{
	{Name: "batched", Kind: KindEscape},
	{Name: "effects", Kind: KindPin},
	{Name: "frame", Kind: KindPin},
	{Name: "requires", Kind: KindPin},
	{Name: "ensures", Kind: KindPin},
	{Name: "variant", Kind: KindPin},
	{Name: "restrict", Kind: KindIntent},
	{Name: "hot", Kind: KindIntent},
	{Name: "wire", Kind: KindIntent},
	{Name: "owner", Kind: KindIntent},
	{Name: "openenum", Kind: KindIntent},
}

// Is reports whether a comment's text is a tiger directive. Text with a
// space after the slashes is prose, not a directive, matching how Go treats
// its own //go: comments.
func Is(text string) bool {
	return strings.HasPrefix(text, Prefix)
}

// Parse reads one //tiger: comment into a Directive.
//
// On error the returned Directive still carries whatever verb and argument
// text was present, so a caller can name the offending verb in a finding.
// The errors are *UnknownVerbError and *MissingReasonError.
func Parse(text string) (Directive, error) {
	if !Is(text) {
		return Directive{Verb: "", Args: ""}, errors.New("not a tiger directive: " + text)
	}
	rest := text[len(Prefix):]
	cut := strings.IndexFunc(rest, unicode.IsSpace)
	if cut < 0 {
		cut = len(rest)
	}
	parsed := Directive{
		Verb: rest[:cut],
		Args: strings.TrimSpace(rest[cut:]),
	}
	spec, ok := Lookup(parsed.Verb)
	if !ok {
		return parsed, &UnknownVerbError{Verb: parsed.Verb}
	}
	if spec.Kind == KindEscape && parsed.Args == "" {
		return parsed, &MissingReasonError{Verb: parsed.Verb}
	}
	if spec.Kind == KindPin {
		if err := validatePinArgs(parsed); err != nil {
			return parsed, &MalformedArgsError{Verb: parsed.Verb, Detail: err.Error()}
		}
	}
	return parsed, nil
}

// validatePinArgs applies the per-verb argument grammar to a pin. Intent
// verbs stay free-form until the waves that enforce them arrive.
func validatePinArgs(parsed Directive) error {
	switch parsed.Verb {
	case "effects":
		_, err := ParseEffects(parsed.Args)
		return err
	case "frame":
		_, err := ParseFrame(parsed.Args)
		return err
	case "variant":
		_, err := ParseVariant(parsed.Args)
		return err
	case "requires", "ensures":
		_, err := ParsePredicate(parsed.Args)
		return err
	}
	return nil
}

// Format prints a directive canonically: the prefix, the verb, and — when
// arguments are present — a single space and the argument text.
func Format(d Directive) string {
	if d.Args == "" {
		return Prefix + d.Verb
	}
	return Prefix + d.Verb + " " + d.Args
}

// Lookup finds a verb in the closed vocabulary.
func Lookup(name string) (Verb, bool) {
	for _, spec := range vocabulary {
		if spec.Name == name {
			return spec, true
		}
	}
	return Verb{Name: "", Kind: KindPin}, false
}

// Verbs returns the closed vocabulary in its declaration-model order.
func Verbs() []Verb {
	listed := make([]Verb, len(vocabulary))
	copy(listed, vocabulary)
	return listed
}
