// Package stdlibio is TS-M10's IO list: which standard library calls do IO.
package stdlibio

import (
	"maps"
	"slices"
)

type Class uint8

const (
	Unlisted Class = iota
	IO
	NoIO
	Unproven
)

func Classify(fullName string) Class {
	return classes[fullName]
}

func Names() []string {
	return slices.Sorted(maps.Keys(classes))
}
