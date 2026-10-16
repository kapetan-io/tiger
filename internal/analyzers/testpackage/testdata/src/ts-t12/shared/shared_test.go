// shared.go declares package shared_test too, so this file is internal, not external.
package shared_test // want `declares package shared_test, .* declare package shared_test_test`

import "testing"

func TestHidden(t *testing.T) { _ = hidden }
