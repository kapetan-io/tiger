// A package that only calls itself assert is not tiger's assert package.
package assert

// Ok stands in for tiger's assert.Ok.
func Ok(cond bool, reason string) {}
