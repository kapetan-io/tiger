package changedtests

import (
	"cmp"
	"maps"
	"slices"

	"github.com/kapetan-io/tiger/internal/module"
)

type Change string

const (
	Modified Change = "modified"
	Deleted  Change = "deleted"
)

type Report struct {
	Changed      []ChangedTest
	BaseTests    int
	BasePackages int
	Unread       []UnreadModule
}

type UnreadModule struct {
	Dir    string
	Reason string
}

type ChangedTest struct {
	Package  string
	File     string
	Function string
	Change   Change
}

type sides struct {
	base testFunctions
	head testFunctions
}

func changedTests(both sides) Report {
	report := Report{BaseTests: both.base.count(), BasePackages: both.base.packages()}
	for _, name := range slices.SortedFunc(maps.Keys(both.base), testName.order) {
		report.Changed = append(report.Changed, changesTo(name, both)...)
	}
	slices.SortFunc(report.Changed, ChangedTest.order)
	return report
}

func changesTo(name testName, both sides) []ChangedTest {
	unchanged := matching{base: both.base[name], head: both.head[name]}.
		next(sameFileAndTokens).
		next(sameTokens)
	edited := unchanged.next(sameFile)
	moved := edited.next(anyDefinition)
	var changes []ChangedTest
	for _, def := range slices.Concat(edited.matchedHead, moved.matchedHead) {
		changes = append(changes, changedTest(name, def, Modified))
	}
	for _, def := range moved.base {
		changes = append(changes, changedTest(name, def, Deleted))
	}
	return changes
}

func (n testName) order(other testName) int {
	return cmp.Or(cmp.Compare(n.dir, other.dir), cmp.Compare(n.function, other.function))
}

func (c ChangedTest) order(other ChangedTest) int {
	return cmp.Or(cmp.Compare(c.Package, other.Package), cmp.Compare(c.Function, other.Function),
		cmp.Compare(c.File, other.File), cmp.Compare(c.Change, other.Change))
}

func changedTest(name testName, def definition, change Change) ChangedTest {
	return ChangedTest{Package: def.pkg, File: def.file, Function: name.function, Change: change}
}

func (r Report) count(change Change) int {
	n := 0
	for _, test := range r.Changed {
		if test.Change == change {
			n++
		}
	}
	return n
}

type unreadAt struct {
	base []module.Unread
	head []module.Unread
}

func (u unreadAt) modules() []UnreadModule {
	unread := []UnreadModule{}
	for _, mod := range u.head {
		reason := mod.Reason + " at head"
		if slices.Contains(u.base, mod) {
			reason += " and the merge base"
		}
		unread = append(unread, UnreadModule{Dir: mod.Dir, Reason: reason})
	}
	for _, mod := range u.base {
		if !slices.Contains(u.head, mod) {
			unread = append(unread,
				UnreadModule{Dir: mod.Dir, Reason: mod.Reason + " at the merge base"})
		}
	}
	slices.SortFunc(unread, func(a, b UnreadModule) int {
		return cmp.Or(cmp.Compare(a.Dir, b.Dir), cmp.Compare(a.Reason, b.Reason))
	})
	return unread
}

func (u unreadAt) all() []module.Unread {
	return slices.Concat(u.base, u.head)
}
