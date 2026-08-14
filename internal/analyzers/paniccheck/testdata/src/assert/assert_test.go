// The assert package's external tests share the exemption: proving panic
// behaviour requires panicking.
package assert_test

import "testing"

func TestExerciseCrashPath(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected a panic")
		}
	}()
	panic("the assert test package may panic to prove panic behaviour")
}
