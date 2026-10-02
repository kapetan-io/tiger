package gaps

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
)

// ListRequest is a list request as decoded off the wire.
type ListRequest struct {
	Limit int32
}

// ListOptions carries a page size into a store.
type ListOptions struct {
	Limit int
}

// listMax is the largest page any request may ask for.
const listMax = 1_000

// CountPage counts at most opts.Limit rows from a stream. Its limit is a
// field, so change 6 checks every value written into it.
func CountPage(scanner *bufio.Scanner, opts ListOptions) int {
	count := 0
	for ; count < opts.Limit && scanner.Scan(); count++ {
	}
	return count
}

// ListRaw writes the request's limit into the field as is, so the client
// decides the page size. querator's six unbounded list endpoints (ENG-194)
// have this shape.
func ListRaw(scanner *bufio.Scanner, req ListRequest) int {
	return CountPage(scanner, ListOptions{Limit: int(req.Limit)})
}

// ListClamped clamps the request's limit where the request enters.
func ListClamped(scanner *bufio.Scanner, req ListRequest) int {
	return CountPage(scanner, ListOptions{Limit: min(int(req.Limit), listMax)})
}

// errBadPack reports a pack header that claims more objects than allowed.
var errBadPack = errors.New("pack claims more objects than it can hold")

// objectsMax is the most objects one pack may hold.
const objectsMax = 1 << 20

// packEntry is one parsed object's position in a pack.
type packEntry struct {
	offset int
}

// packCount reads the object count from a pack header.
func packCount(raw []byte) int {
	return int(binary.BigEndian.Uint32(raw[8:12]))
}

// ParsePackRaw sizes its entries by the header's count, a number the client
// wrote, before reading any object. git-server's pack ingest (ENG-193) has
// this shape; it is a boundary no list of config, flag and request sources
// covers.
func ParsePackRaw(raw []byte) []packEntry {
	count := packCount(raw)
	entries := make([]packEntry, 0, count)
	reader := bytes.NewReader(raw[12:])
	for i := 0; i < count && reader.Len() > 0; i++ {
		entries = append(entries, packEntry{offset: i})
		_, _ = reader.ReadByte() // a demo object is one byte; an empty pack ends the loop
	}
	return entries
}

// ParsePack rejects a count larger than the pack could hold or than
// objectsMax, where the count is decoded.
func ParsePack(raw []byte) ([]packEntry, error) {
	count := packCount(raw)
	if count > len(raw)-12 || count > objectsMax {
		return nil, errBadPack
	}
	return ParsePackRaw(raw), nil
}

// partitionsMax is the most partitions one queue may ask for.
const partitionsMax = 256

// spread splits requested partitions across one backend, the way querator's
// assignPartitions does across several.
func spread(requested int) []int {
	return []int{requested}
}

// CreateRaw loops once per requested partition with no upper bound, reached
// through spread's returned slice. querator's create-queue (ENG-195) has this
// shape.
func CreateRaw(requested int) int {
	created := 0
	for _, count := range spread(requested) {
		for i := 0; i < count; i++ {
			created++
		}
	}
	return created
}

// clampPartitions is a clamp in a helper. Under the bounded-result refinement
// its result carries the bound to every caller.
func clampPartitions(requested int) int {
	return min(requested, partitionsMax)
}

// CreateClamped clamps through the helper before spreading.
func CreateClamped(requested int) int {
	return CreateRaw(clampPartitions(requested))
}

// SpinThroughValue calls the loop through a function value, which change 6
// does not follow.
func SpinThroughValue(config SpinConfig, done func() bool) error {
	spin := SpinReported
	return spin(done, config.Spins)
}
