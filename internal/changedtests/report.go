package changedtests

import (
	"cmp"
	"maps"
	"slices"
)

type Change string

const (
	modified Change = "modified"
	deleted  Change = "deleted"
)

type Report struct {
	Tests    []ChangedTest
	Compared int
	Packages int
}

type ChangedTest struct {
	Package  string
	File     string
	Function string
	Change   Change
}

type Sides struct {
	Base Tests
	Head Tests
}

func compare(sides Sides) Report {
	report := Report{Compared: len(sides.Base), Packages: sides.Base.packages()}
	for _, name := range slices.SortedFunc(maps.Keys(sides.Base), TestName.compare) {
		was := sides.Base[name]
		now, ok := sides.Head[name]
		switch {
		case !ok:
			report.Tests = append(report.Tests, changedTest(name, was[0].File, deleted))
		case !was.same(now):
			report.Tests = append(report.Tests, changedTest(name, now[0].File, modified))
		}
	}
	return report
}

func (n TestName) compare(other TestName) int {
	return cmp.Or(cmp.Compare(n.Package, other.Package), cmp.Compare(n.Function, other.Function))
}

func changedTest(name TestName, file string, change Change) ChangedTest {
	return ChangedTest{Package: name.Package, File: file, Function: name.Function, Change: change}
}

func (r Report) count(change Change) int {
	n := 0
	for _, test := range r.Tests {
		if test.Change == change {
			n++
		}
	}
	return n
}
