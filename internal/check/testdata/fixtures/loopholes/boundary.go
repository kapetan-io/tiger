package loopholes

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
)


type ListRequest struct {
	Limit int32
}


type ListOptions struct {
	Limit int
}


const listMax = 1_000



func CountPage(scanner *bufio.Scanner, opts ListOptions) int {
	count := 0
	for ; count < opts.Limit && scanner.Scan(); count++ {
	}
	return count
}




func ListRaw(scanner *bufio.Scanner, req ListRequest) int {
	return CountPage(scanner, ListOptions{Limit: int(req.Limit)})
}


func ListClamped(scanner *bufio.Scanner, req ListRequest) int {
	return CountPage(scanner, ListOptions{Limit: min(int(req.Limit), listMax)})
}


var errBadPack = errors.New("pack claims more objects than it can hold")


const objectsMax = 1 << 20


type packEntry struct {
	offset int
}


func packCount(raw []byte) int {
	return int(binary.BigEndian.Uint32(raw[8:12]))
}





func ParsePackRaw(raw []byte) []packEntry {
	count := packCount(raw)
	entries := make([]packEntry, 0, count)
	reader := bytes.NewReader(raw[12:])
	for i := 0; i < count && reader.Len() > 0; i++ {
		entries = append(entries, packEntry{offset: i})
		_, _ = reader.ReadByte()
	}
	return entries
}



func ParsePack(raw []byte) ([]packEntry, error) {
	count := packCount(raw)
	if count > len(raw)-12 || count > objectsMax {
		return nil, errBadPack
	}
	return ParsePackRaw(raw), nil
}


const partitionsMax = 256



func spread(requested int) []int {
	return []int{requested}
}




func CreateRaw(requested int) int {
	created := 0
	for _, count := range spread(requested) {
		for i := 0; i < count; i++ {
			created++
		}
	}
	return created
}



func clampPartitions(requested int) int {
	return min(requested, partitionsMax)
}


func CreateClamped(requested int) int {
	return CreateRaw(clampPartitions(requested))
}



func SpinThroughValue(config SpinConfig, done func() bool) error {
	spin := SpinReported
	return spin(done, config.Spins)
}
