package codec

import "testing"

// TestPlaceholder exists only so package codec loads through its
// test-augmented variant ("codec [codec.test]"), proving TS-X01's package
// facts resolve by import path across that seam, not by load ID.
func TestPlaceholder(t *testing.T) {}
