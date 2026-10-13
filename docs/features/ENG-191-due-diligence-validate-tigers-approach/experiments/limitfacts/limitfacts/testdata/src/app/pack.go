package app

import "encoding/binary"

const objectsMax = 1 << 20

type entry struct{ offset int }

func parseObjects(raw []byte, count int) []entry { // want parseObjects:`\[1\]`
	entries := make([]entry, 0, count)
	for i := range count {
		entries = append(entries, entry{offset: i})
	}
	return entries
}

// IngestRaw is git-server's pack header: a 12-byte request decides the count.
func IngestRaw(raw []byte) []entry {
	count := int(binary.BigEndian.Uint32(raw[8:12]))
	return parseObjects(raw, count) // want `has no upper bound where it enters`
}

func IngestClamped(raw []byte) []entry {
	count := int(binary.BigEndian.Uint32(raw[8:12]))
	if count > objectsMax {
		return nil
	}
	return parseObjects(raw, count)
}

// Known miss: a limit held in a local variable whose value came through a
// slice is not a parameter or field, so nothing checks where it came from.
func spread(total int) []int { return []int{total} }

func LocalThroughSlice(requested int) int {
	n := 0
	for _, count := range spread(requested) {
		for i := 0; i < count; i++ {
			n++
		}
	}
	return n
}
