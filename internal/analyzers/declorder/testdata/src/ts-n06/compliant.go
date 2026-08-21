// The compliant shapes: a correctly prefixed single-caller helper, a
// helper with two callers (out of the rule's stated scope), and helpers
// whose sole caller is main, init, or a test function (exempt callers).
package fixture

// ReadPage calls readPageRetry, its single caller: the helper name already
// carries the caller's prefix (first rune case-normalized), so this stays
// silent.
func ReadPage(id int) []byte {
	return readPageRetry(id)
}

func readPageRetry(id int) []byte {
	return nil
}

// walkTree and walkNode: walkNode recurses into itself, so the recursive
// call never counts as a second caller; its one true caller is walkTree,
// and the name is correctly prefixed.
func walkTree(root *node) {
	walkTreeWalkNode(root)
}

func walkTreeWalkNode(n *node) {
	if n == nil {
		return
	}
	walkTreeWalkNode(n.next)
}

type node struct {
	next *node
}

// shared is called from two distinct FuncDecls, so the rule's single-caller
// scope never applies — it stays silent even though its name carries
// neither caller's prefix.
func firstCaller() int {
	return shared()
}

func secondCaller() int {
	return shared()
}

func shared() int {
	return 0
}

// mainHelper's only caller is main, an exempt caller name.
func main() {
	mainHelper()
}

func mainHelper() {
}

// initHelper's only caller is init, an exempt caller name.
func init() {
	initHelper()
}

func initHelper() {
}
