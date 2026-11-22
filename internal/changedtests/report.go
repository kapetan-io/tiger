package changedtests

import (
	"cmp"
	"maps"
	"slices"
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
	report := Report{BaseTests: len(both.base), BasePackages: both.base.packages()}
	for _, name := range slices.SortedFunc(maps.Keys(both.base), testName.order) {
		was := both.base[name]
		now, ok := both.head[name]
		if !ok {
			report.Changed = append(report.Changed, changedTest(name, was[0], Deleted))
			continue
		}
		if def, changed := now.changedFrom(was); changed {
			report.Changed = append(report.Changed, changedTest(name, def, Modified))
		} else if def, changed := was.changedFrom(now); changed {
			report.Changed = append(report.Changed, changedTest(name, def, Modified))
		}
	}
	slices.SortStableFunc(report.Changed, ChangedTest.order)
	return report
}

func (n testName) order(other testName) int {
	return cmp.Or(cmp.Compare(n.dir, other.dir), cmp.Compare(n.function, other.function))
}

func (c ChangedTest) order(other ChangedTest) int {
	return cmp.Or(cmp.Compare(c.Package, other.Package), cmp.Compare(c.Function, other.Function))
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
