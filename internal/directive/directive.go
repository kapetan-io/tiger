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
// Parse validates verb identity and each verb's argument shape: an escape
// needs a reason, and a pin takes its verb's grammar.
package directive

import (
	"errors"
	"strings"
	"unicode"
)

const Prefix = "//tiger:"

type Kind int

const (
	KindPin Kind = iota
	KindIntent
	KindEscape
)

type Directive struct {
	Verb string
	Args string
}

type Verb struct {
	Name string
	Kind Kind
}

type UnknownVerbError struct {
	Verb string
}

func (e *UnknownVerbError) Error() string {
	return "unknown directive verb " + strings.TrimSpace(Prefix+e.Verb)
}

type MissingReasonError struct {
	Verb string
}

func (e *MissingReasonError) Error() string {
	return Prefix + e.Verb + " requires a reason"
}

type MalformedArgsError struct {
	Verb   string
	Detail string
}

func (e *MalformedArgsError) Error() string {
	return Prefix + e.Verb + ": " + e.Detail
}

var vocabulary = []Verb{
	{Name: "batched", Kind: KindEscape},
	{Name: "requires", Kind: KindPin},
	{Name: "ensures", Kind: KindPin},
	{Name: "variant", Kind: KindPin},
	{Name: "openenum", Kind: KindEscape},
}

func is(text string) bool {
	return strings.HasPrefix(text, Prefix)
}

func Parse(text string) (Directive, error) {
	if !is(text) {
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
		if err := validateArgs(parsed); err != nil {
			return parsed, &MalformedArgsError{Verb: parsed.Verb, Detail: err.Error()}
		}
	}
	return parsed, nil
}

func validateArgs(parsed Directive) error {
	switch parsed.Verb {
	case "variant":
		_, err := ParseVariant(parsed.Args)
		return err
	case "requires", "ensures":
		_, err := ParsePredicate(parsed.Args)
		return err
	}
	return nil
}

func Format(d Directive) string {
	if d.Args == "" {
		return Prefix + d.Verb
	}
	return Prefix + d.Verb + " " + d.Args
}

func Lookup(name string) (Verb, bool) {
	for _, spec := range vocabulary {
		if spec.Name == name {
			return spec, true
		}
	}
	return Verb{Name: "", Kind: KindPin}, false
}

func Verbs() []Verb {
	listed := make([]Verb, len(vocabulary))
	copy(listed, vocabulary)
	return listed
}
