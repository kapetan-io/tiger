// The TS-F01 compliant shapes: an exact pin over own instructions, a
// mutate pin over a parameter-rooted write, and a pin exercising the
// stdlib effects table.
package ts01

import "time"

// Pure does nothing effectful — an exact none pin.
//
//tiger:effects none
func Pure() int { // want Pure:`none`
	return 42
}

// Ring is a minimal struct with a log field to exercise mutate(r.log).
type Ring struct {
	log []byte
}

// Append grows the ring's log: a write through the receiver plus the
// append builtin's own allocation — both declared, exactly.
//
//tiger:effects mutate(r.log), alloc
func (r *Ring) Append(entry byte) { // want Append:`alloc`
	r.log = append(r.log, entry)
}

// Clock reads the wall clock through the stdlib effects table.
//
//tiger:effects time
func Clock() time.Time { // want Clock:`time`
	return time.Now()
}
