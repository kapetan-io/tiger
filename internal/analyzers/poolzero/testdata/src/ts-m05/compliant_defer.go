// A deferred Put runs at function exit, so a zeroing written after the defer
// statement still comes first.
package fixture

func deferPutThenZero(entry byte) {
	buf, ok := bufferPool.Get().(*Buffer)
	if !ok {
		return
	}
	defer bufferPool.Put(buf)
	buf.data = append(buf.data, entry)
	*buf = Buffer{}
}
