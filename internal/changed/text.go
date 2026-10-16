package changed

import (
	"fmt"
	"strings"

	"github.com/kapetan-io/tiger/internal/revision"
)

const (
	purposeText = "Functions in this change that started or stopped calling another " +
		"package, or a function in their own package that calls one. " +
		"Check that each one is intended."

	emptyEverything = "No function started or stopped calling another package, " +
		"or a function in its own package that calls one."
	emptyList = "No function in an existing package started or stopped calling another " +
		"package, or a function in its own package that calls one."

	blindSpots = "Tiger can't see: calls into outside packages that never reach the " +
		"operating system (fmt, strings, error libraries), calls between methods of one " +
		"type, calls inside a package to a function that calls no other package or to an " +
		"interface the package declares, writes through io.Writer, work handed over a " +
		"channel, or a new implementation behind an unchanged interface. A call moved " +
		"into a new helper, or a renamed type whose calls changed, shows as removed " +
		"plus added."
)

// maxNamed is how many names a footer lists before it counts the rest.
const maxNamed = 5

// minPrefix is the shortest ref taken for an abbreviated commit hash, git's
// own minimum abbreviation.
const minPrefix = 4

// Text renders the report for a terminal. The bytes depend only on the report
// and the revisions, never on the terminal, the clock or the map order.
func (r Report) Text(rev revision.Comparison) string {
	var out strings.Builder
	fmt.Fprintf(&out, "tiger changed: %s -> %s\n", baseLabel(rev), headLabel(rev))
	out.WriteString(purposeText + "\n\n")
	out.WriteString(r.countsLine() + "\n\n")
	switch {
	case r.empty():
		out.WriteString(emptyEverything + "\n\n")
	case len(r.Functions) == 0:
		out.WriteString(emptyList + "\n\n")
	default:
		writeList(&out, r.Functions, "")
		out.WriteString("\n")
	}
	for _, rename := range r.Renamed {
		fmt.Fprintf(&out, "Renamed: %s looks renamed to %s; "+
			"it calls the same %d types and functions in other packages\n\n",
			rename.From, rename.To, rename.Callees)
	}
	for _, pkg := range r.NewPackages {
		writeNewPackage(&out, pkg)
	}
	for _, pkg := range r.RemovedPackages {
		fmt.Fprintf(&out, "Removed package %s: %d types and functions\n\n", pkg.Name, pkg.Owners)
	}
	out.WriteString(r.Scope.footer() + "\n" + blindSpots + "\n")
	return out.String()
}

func (r Report) countsLine() string {
	parts := r.countParts()
	labels := make([]string, len(parts))
	for i, part := range parts {
		labels[i] = part.label()
	}
	return strings.Join(labels, " · ")
}

// countPart is one "Label: N" of the counts line.
type countPart struct {
	name  string
	count int
}

func (c countPart) label() string { return fmt.Sprintf("%s: %d", c.name, c.count) }

// countParts lists the counts line in order: the two call counts always, then
// each of the other categories only when it is not zero.
func (r Report) countParts() []countPart {
	parts := []countPart{{"Calls added", r.added()}, {"Calls removed", r.removed()}}
	for _, extra := range []countPart{
		{"New packages", len(r.NewPackages)},
		{"Removed packages", len(r.RemovedPackages)},
		{"Renamed", len(r.Renamed)},
	} {
		if extra.count > 0 {
			parts = append(parts, extra)
		}
	}
	return parts
}

func writeList(out *strings.Builder, functions []Function, indent string) {
	for _, fn := range functions {
		out.WriteString(indent + fn.Name + "\n")
		for _, call := range fn.Calls {
			sign := "-"
			if call.Added {
				sign = "+"
			}
			out.WriteString(indent + sign + "   " + call.Callee + "\n")
		}
	}
}

func writeNewPackage(out *strings.Builder, pkg NewPackage) {
	fmt.Fprintf(out, "New package %s: %d types and functions, %d calls\n",
		pkg.Name, pkg.Owners, pkg.calls())
	fmt.Fprintf(out, "  Uses existing packages: %s\n", usesText(pkg.Uses.Prior, ""))
	fmt.Fprintf(out, "  Uses new packages: %s\n", usesText(pkg.Uses.New, ""))
	fmt.Fprintf(out, "  Uses outside packages: %s\n", usesText(pkg.Uses.Outside, ""))
	if pkg.calls() > 0 {
		out.WriteString("  Every call, by function:\n")
		writeList(out, pkg.Functions, "    ")
	}
	out.WriteString("\n")
}

func usesText(uses []Use, quote string) string {
	if len(uses) == 0 {
		return "none"
	}
	parts := make([]string, len(uses))
	for i, use := range uses {
		parts[i] = fmt.Sprintf("%s%s%s (%s)", quote, use.Package, quote, strings.Join(use.Symbols, ", "))
	}
	return strings.Join(parts, "; ")
}

// footer is the first of the two lines every report ends with: what was
// compared and what was left out.
func (s Scope) footer() string {
	line := fmt.Sprintf("Compared %d types and functions in %d packages; ", s.Owners, s.Packages)
	if len(s.Generated) == 0 {
		line += "skipped all test files (no generated files found)"
	} else {
		line += fmt.Sprintf("skipped %d generated files (%s) and all test files",
			len(s.Generated), named(s.Generated))
	}
	if len(s.NotCompared) > 0 {
		line += fmt.Sprintf("; %d packages did not load and were not compared (%s)",
			len(s.NotCompared), named(s.NotCompared))
	}
	return line + "."
}

// named joins up to maxNamed names and counts the rest.
func named(names []string) string {
	if len(names) <= maxNamed {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(names[:maxNamed], ", "), len(names)-maxNamed)
}

// baseLabel prints the ref with its tip hash, or the tip hash alone when the ref is a
// hash prefix, and says "merge base" with that hash when it is not the tip.
func baseLabel(rev revision.Comparison) string {
	base := splitBase(rev)
	if base.note == "" {
		return base.name
	}
	return fmt.Sprintf("%s (%s)", base.name, base.note)
}

// baseName is the base label split into the name a reader recognises and the
// parenthesised note after it, empty when the name says everything.
type baseName struct {
	name string
	note string
}

func splitBase(rev revision.Comparison) baseName {
	tip := revision.Short(rev.BaseTip)
	prefix := strings.HasPrefix(rev.BaseTip, strings.ToLower(rev.BaseRef))
	byName := len(rev.BaseRef) < minPrefix || !prefix
	switch {
	case rev.MergeBase != rev.BaseTip && byName:
		return baseName{rev.BaseRef, "merge base " + revision.Short(rev.MergeBase)}
	case rev.MergeBase != rev.BaseTip:
		return baseName{tip, "merge base " + revision.Short(rev.MergeBase)}
	case byName:
		return baseName{rev.BaseRef, tip}
	}
	return baseName{name: tip}
}

// headLabel names the head side by commit, never by branch, because CI checks
// out a detached merge commit that has no branch.
func headLabel(rev revision.Comparison) string {
	label := revision.Short(rev.Head)
	if rev.Dirty {
		label += " + uncommitted changes"
	}
	return label
}
