// A deferred Put fires when a return sits between the defer and the zeroing.
package fixture

func deferPutReturnsBeforeZero(skip bool, entry byte) {
	buf, ok := bufferPool.Get().(*Buffer)
	if !ok {
		return
	}
	defer bufferPool.Put(buf) // want `TS-M05: this Put is not preceded by \*b = T\{\} on every path to it`
	if skip {
		return
	}
	buf.data = append(buf.data, entry)
	*buf = Buffer{}
}
