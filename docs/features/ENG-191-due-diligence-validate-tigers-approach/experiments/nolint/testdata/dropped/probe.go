package probe

import "os"

// Remove drops the error from os.Remove with no comment, which TS-E02 reports.
func Remove(path string) {
	_ = os.Remove(path)
}
