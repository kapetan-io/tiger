package layerfile

import (
	"fmt"

	"github.com/kapetan-io/tiger/assert"
)

// Message words a verdict that does not allow the import: what was imported, and the
// exact name to add, or the way to keep it out.
func (v Verdict) Message() string {
	entryText := fmt.Sprintf("the %q entry in %s", v.entryKey, FileName)
	text := ""
	switch v.breach {
	case unlistedPackage:
		text = fmt.Sprintf("%s: %s doesn't list %s — add it, or pass what you need in through "+
			"an interface", RuleID, entryText, packageName(v.name))
	case unlistedModule:
		text = fmt.Sprintf("%s: %s doesn't list %s — add it, or its module path, to the entry",
			RuleID, entryText, v.imported)
	case unlistedImpure:
		text = fmt.Sprintf("%s: %s doesn't list %s, so this package can't import %s — add %q "+
			"to it, or pass it in through an interface", RuleID, entryText, v.name, v.imported,
			v.name)
	default:
		assert.Unreachable("an allowed import is never reported")
	}
	return text
}

// MissingEntryMessage words the finding for the package in dir, "." or
// "internal/store", that no entry covers.
func MissingEntryMessage(dir string) string {
	key := packageKey(dir)
	subject := "package " + key
	if key == rootKey {
		subject = packageName(key)
	}
	return fmt.Sprintf("%s: %s has no entry in %s — add it as a key listing the module "+
		"packages, modules and IO or random subtrees (os, net, ...) it imports, or [] for none",
		RuleID, subject, FileName)
}

// StaleMessage words the finding for an entry that matches no package.
func StaleMessage(key string) string {
	return fmt.Sprintf("%s: the %q entry matches no package directory — remove it, or fix its "+
		"path", RuleID, key)
}

// packageName names a package key in a message; "." alone reads as
// punctuation.
func packageName(key string) string {
	if key == rootKey {
		return `the root package "."`
	}
	return key
}
