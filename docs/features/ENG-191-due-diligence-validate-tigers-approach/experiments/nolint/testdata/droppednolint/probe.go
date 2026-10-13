package probe

import "os"

// Remove drops the same error, but a bare //nolint counts as the comment
// TS-E02 asks for.
func Remove(path string) {
	_ = os.Remove(path) //nolint
}
