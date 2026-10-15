package fixture

// putAfterNamedZeroFields spells out the zero value field by field, which is
// still the zero value.
func putAfterNamedZeroFields(buf *Buffer) {
	*buf = Buffer{data: nil}
	bufferPool.Put(buf)
}
