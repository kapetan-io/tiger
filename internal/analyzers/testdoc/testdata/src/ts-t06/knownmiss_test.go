// known-miss: TS-T06 in wave 1 checks only Test functions taking a bare
// *testing.T parameter. Benchmarks and fuzz targets state their goal and
// method just as much as a Test does, but BenchmarkNoGoal and FuzzNoGoal
// below stay silent with no Goal line, and so does a lowercase test helper
// (its name does not match the Test-function shape at all).
package fixture

import "testing"

func BenchmarkNoGoal(b *testing.B) {
	_ = b
}

func FuzzNoGoal(f *testing.F) {
	_ = f
}

func testHelper(t *testing.T) {
	_ = t
}
