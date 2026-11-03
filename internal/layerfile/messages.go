package layerfile

import (
	"fmt"

	"github.com/kapetan-io/tiger/assert"
)

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

func StaleMessage(key string) string {
	return fmt.Sprintf("%s: the %q entry matches no package directory — remove it, or fix its "+
		"path", RuleID, key)
}

func packageName(key string) string {
	if key == rootKey {
		return `the root package "."`
	}
	return key
}
