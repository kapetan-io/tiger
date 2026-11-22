package changedtests

import (
	"fmt"
	"strings"

	"github.com/kapetan-io/tiger/internal/revision"
)

const (
	purpose = "Test functions that existed at the merge base and that this change " +
		"modifies or deletes. Each one guarded behavior that may now be different. " +
		"Check that each change is intended."

	empty = "No test function that existed at the merge base was modified or deleted."

	scope = "Counted only top-level Test, Benchmark, Fuzz and Example functions in " +
		"_test.go files. Tests added since the merge base, test methods on suite " +
		"types, helpers and testdata are not listed, and an edit to comments or " +
		"spacing alone is not a change."
)

func (r Report) Text(rev revision.Comparison) string {
	var out strings.Builder
	fmt.Fprintf(&out, "tiger changed-tests: %s -> %s\n", rev.BaseLabel(), rev.HeadLabel())
	out.WriteString(purpose + "\n\n")
	fmt.Fprintf(&out, "Modified: %d · Deleted: %d\n\n", r.count(modified), r.count(deleted))
	if len(r.Tests) == 0 {
		out.WriteString(empty + "\n\n")
	} else {
		r.writeList(&out)
		out.WriteString("\n")
	}
	fmt.Fprintf(&out, "Compared %d test functions in %d packages at the merge base.\n",
		r.Compared, r.Packages)
	out.WriteString(scope + "\n")
	return out.String()
}

func (r Report) writeList(out *strings.Builder) {
	start := 0
	for i := range r.Tests {
		if i+1 == len(r.Tests) || r.Tests[i+1].Package != r.Tests[start].Package {
			writePackage(out, r.Tests[start:i+1])
			start = i + 1
		}
	}
}

func writePackage(out *strings.Builder, tests []ChangedTest) {
	changeWidth, nameWidth := 0, 0
	for _, test := range tests {
		changeWidth = max(changeWidth, len(test.Change))
		nameWidth = max(nameWidth, len(test.Function))
	}
	out.WriteString(tests[0].Package + "\n")
	for _, test := range tests {
		fmt.Fprintf(out, "  %-*s  %-*s  %s\n",
			changeWidth, test.Change, nameWidth, test.Function, test.File)
	}
}
