// Package smoke contains one known TS-S09 violation for the plugin smoke
// test: the finding must surface through a golangci-lint run.
package smoke

// Spin loops with a goto so nogoto fires.
func Spin(limit int) int {
	count := 0
begin:
	count++
	if count < limit {
		goto begin
	}
	return count
}
