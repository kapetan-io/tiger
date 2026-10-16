package changed

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/kapetan-io/tiger/internal/revision"
)

const (
	purposeMarkdown = "Functions in this PR that started or stopped calling another " +
		"package, or a function in their own package that calls one. " +
		"Check that each one is intended."

	// marker is the first line of the comment; CI finds its one sticky comment by it.
	marker = "<!-- tiger-changed -->"

	// maxComment is where the report starts dropping detail. GitHub rejects a
	// comment over 65,536 characters, and the rest of the room is left for the
	// text added after the cut.
	maxComment = 60000
)

// Markdown renders the report as a pull request comment. A report that would
// pass maxComment is cut: first the new packages' call lists, then the main
// list after its last whole function that fits.
func (r Report) Markdown(rev revision.Comparison) string {
	full := r.markdown(rev, view{shown: len(r.Functions)})
	if tooLong(full) {
		folded := view{shown: len(r.Functions), fold: true}
		if full = r.markdown(rev, folded); tooLong(full) {
			folded.shown = sort.Search(len(r.Functions), func(n int) bool {
				return tooLong(r.markdown(rev, view{shown: n + 1, fold: true}))
			})
			full = r.markdown(rev, folded)
		}
	}
	return full
}

func tooLong(comment string) bool { return utf8.RuneCountInString(comment) > maxComment }

// view says how much of the report a comment shows: how many of the main
// list's functions, and whether the new packages' call lists are folded away.
type view struct {
	shown int
	fold  bool
}

func (r Report) markdown(rev revision.Comparison, show view) string {
	var out strings.Builder
	base := splitBase(rev)
	fmt.Fprintf(&out, "%s\n#### tiger changed: `%s`", marker, base.name)
	if base.note != "" {
		fmt.Fprintf(&out, " (%s)", base.note)
	}
	out.WriteString(" → `" + revision.Short(rev.Head) + "`")
	if rev.Dirty {
		out.WriteString(" + uncommitted changes")
	}
	out.WriteString("\n\n" + purposeMarkdown + "\n\n")
	out.WriteString(r.countsMarkdown() + "\n\n")
	switch {
	case r.empty():
		out.WriteString(emptyEverything + "\n\n")
	case len(r.Functions) == 0:
		out.WriteString(emptyList + "\n\n")
	default:
		hidden := r.Functions[show.shown:]
		if show.shown > 0 {
			out.WriteString("```diff\n")
			writeList(&out, r.Functions[:show.shown], "")
			out.WriteString("```\n\n")
		}
		if len(hidden) > 0 {
			calls := 0
			for _, fn := range hidden {
				calls += len(fn.Calls)
			}
			fmt.Fprintf(&out, "%d more calls in %d functions not shown; run `tiger changed %s` for the rest.\n\n",
				calls, len(hidden), rev.BaseRef)
		}
	}
	for _, rename := range r.Renamed {
		fmt.Fprintf(&out, "**Renamed:** `%s` looks renamed to `%s`; "+
			"it calls the same %d types and functions in other packages.\n\n",
			rename.From, rename.To, rename.Callees)
	}
	for _, pkg := range r.NewPackages {
		writeNewPackageMarkdown(&out, pkg, show, rev.BaseRef)
	}
	for _, pkg := range r.RemovedPackages {
		fmt.Fprintf(&out, "**Removed package `%s`** — %d types and functions.\n\n", pkg.Name, pkg.Owners)
	}
	out.WriteString("<sub>" + r.Scope.footer() + "<br>\n" + blindSpots + "</sub>\n")
	return out.String()
}

// countsMarkdown bolds each count that is not zero, so a reader's eye lands
// on what changed.
func (r Report) countsMarkdown() string {
	parts := r.countParts()
	labels := make([]string, len(parts))
	for i, part := range parts {
		labels[i] = part.label()
		if part.count > 0 {
			labels[i] = "**" + labels[i] + "**"
		}
	}
	return strings.Join(labels, " · ")
}

// writeNewPackageMarkdown keeps the summary bullets open because they are the
// wiring a reviewer checks, and folds the call list. Summary text uses <code>
// because GitHub does not render markdown inside <summary>.
func writeNewPackageMarkdown(out *strings.Builder, pkg NewPackage, show view, base string) {
	fmt.Fprintf(out, "**New package `%s`** — %d types and functions, %d calls:\n\n",
		pkg.Name, pkg.Owners, pkg.calls())
	fmt.Fprintf(out, "- Uses existing packages: %s\n", usesText(pkg.Uses.Prior, "`"))
	fmt.Fprintf(out, "- Uses new packages: %s\n", usesText(pkg.Uses.New, "`"))
	fmt.Fprintf(out, "- Uses outside packages: %s\n\n", usesText(pkg.Uses.Outside, "`"))
	switch {
	case pkg.calls() == 0:
	case show.fold:
		fmt.Fprintf(out, "Too long for a comment; run `tiger changed %s` for every call.\n\n", base)
	default:
		fmt.Fprintf(out, "<details>\n<summary>Every call from <code>%s</code>, by function (%d)</summary>\n\n",
			pkg.Name, pkg.calls())
		out.WriteString("```diff\n")
		writeList(out, pkg.Functions, "")
		out.WriteString("```\n\n</details>\n\n")
	}
}
