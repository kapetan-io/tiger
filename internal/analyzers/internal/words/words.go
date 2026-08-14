// Package words splits a Go identifier into its camelCase tokens.
//
// This is shared infrastructure for the naming analyzers (namedeny,
// namepairs, participle): each needs the same tokenization so a dictionary
// or table lookup runs against the same words a reader would see.
package words

import "unicode"

// Split breaks name into its camelCase words, treating an acronym run as
// one word ("HTTPServer" splits to "HTTP", "Server"; "parseURL" splits to
// "parse", "URL") and underscores as explicit boundaries. A run of digits
// sticks to the word that precedes it ("userID2" splits to "user", "ID2").
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

// splitCamel breaks one underscore-free segment into camelCase words.
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

// boundary reports whether an uppercase rune at index i in runes starts a
// new word: either the previous rune is not itself uppercase (a lowercase
// letter or a digit ends its word here), or the previous rune is uppercase
// and the next one is lowercase (the point an acronym run hands off to a
// following Capitalized word, as in HTTPServer).
func boundary(runes []rune, i int) bool {
	previous := runes[i-1]
	if !unicode.IsUpper(previous) {
		return true
	}
	return i+1 < len(runes) && unicode.IsLower(runes[i+1])
}
