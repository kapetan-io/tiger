// Package smoke contains one known TS-S09 violation and one known TS-M10
// violation for the plugin smoke test: both findings must surface through
// a golangci-lint run.
package smoke

import "os"

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

// ReadAll calls stdlib IO once per path inside a bounded, unannotated loop
// so ioinloop fires.
func ReadAll(paths []string) error {
	for i := 0; i < len(paths); i++ {
		if _, err := os.ReadFile(paths[i]); err != nil {
			return err
		}
	}
	return nil
}
