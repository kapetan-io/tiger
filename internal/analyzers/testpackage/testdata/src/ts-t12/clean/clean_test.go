// The compliant shape with a real import: the external package calls the exported API.
package clean_test

import (
	"testing"

	"ts-t12/clean"
)

func TestAdd(t *testing.T) { _ = clean.Add(1, 2) }
