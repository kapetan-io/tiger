// A //line directive does not turn a production file into a test file.
package fixture

//line gen_test.go:1
func generated(entries []string) {
	for _, entry := range entries {
		go notifyOne(entry) // want `TS-C09: this loop starts a goroutine per item`
	}
}
