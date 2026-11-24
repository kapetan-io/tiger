// The TS-M10 silent forms for calls the IO list does not mark IO: a call in
// a standard library package that is not on the list at all, and a call the
// list marks as unproven, both stay silent in a loop body. TS-M10 decides IO
// from the list, not from the package a call lives in.
package fixture

import (
	"database/sql"
	"net"
	"syscall"
)

// pidAll reads the process id in a loop: syscall.Getpid is not on the list,
// so it is silent even though syscall holds raw system calls.
func pidAll(n int) int {
	total := 0
	for i := 0; i < n; i++ {
		total += syscall.Getpid()
	}
	return total
}

// readAllConn reads through the net.Conn interface: the implementer may be
// an in-memory net.Pipe, so the list marks (net.Conn).Read unproven and the
// call is silent.
func readAllConn(conn net.Conn, chunks [][]byte) error {
	for _, chunk := range chunks {
		if _, err := conn.Read(chunk); err != nil {
			return err
		}
	}
	return nil
}

// openAll opens a handle per name: sql.Open usually only validates its
// arguments and opens no connection until first use, so the list marks it
// unproven and the call is silent.
func openAll(names []string) ([]*sql.DB, error) {
	dbs := make([]*sql.DB, 0, len(names))
	for _, name := range names {
		db, err := sql.Open("driver", name)
		if err != nil {
			return nil, err
		}
		dbs = append(dbs, db)
	}
	return dbs, nil
}
