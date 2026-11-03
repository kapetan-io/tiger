// Package words splits a Go identifier into its camelCase tokens.
//
// This is shared infrastructure for the naming analyzers (participle and
// others): each needs the same tokenization so a dictionary or table
// lookup runs against the same words a reader would see.
package words

import "unicode"

func Split(name string) []string {
	var tokens []string
	var start int
	for i := 0; i <= len(name); i++ {
		if i < len(name) && name[i] != '_' {
			continue
		}
		if segment := name[start:i]; segment != "" {
			tokens = append(tokens, splitCamel(segment)...)
		}
		start = i + 1
	}
	return tokens
}

func splitCamel(segment string) []string {
	runes := []rune(segment)
	var tokens []string
	var current []rune
	for i, r := range runes {
		if unicode.IsUpper(r) && len(current) > 0 && boundary(runes, i) {
			tokens = append(tokens, string(current))
			current = nil
		}
		current = append(current, r)
	}
	if len(current) > 0 {
		tokens = append(tokens, string(current))
	}
	return tokens
}

func boundary(runes []rune, i int) bool {
	previous := runes[i-1]
	if !unicode.IsUpper(previous) {
		return true
	}
	return i+1 < len(runes) && unicode.IsLower(runes[i+1])
}
