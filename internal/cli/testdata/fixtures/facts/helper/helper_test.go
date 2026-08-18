// An internal (whitebox) test file: its presence makes go/packages split
// helper into a plain and a test-augmented variant, so the run must still
// analyze helper before core and propagate the io(net) fact across the
// variant seam.
package helper

var _ = Ping
