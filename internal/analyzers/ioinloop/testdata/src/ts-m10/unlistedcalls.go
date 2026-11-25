// The TS-M10 silent forms for calls the IO list does not mark IO: a call in
// a standard library package that is not on the list at all, and a call the
// list marks as unproven, both stay silent in a loop body. TS-M10 decides IO
// from the list, not from the package a call lives in.
package fixture

import (
	"context"
	"database/sql"
	"net"
	"net/http"
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

// writeAllResponse writes each chunk through an http.ResponseWriter: an
// httptest.ResponseRecorder is in memory, so the list marks the interface
// method unproven and the call is silent.
func writeAllResponse(w http.ResponseWriter, chunks [][]byte) error {
	for _, chunk := range chunks {
		if _, err := w.Write(chunk); err != nil {
			return err
		}
	}
	return nil
}

// roundTripAll sends each request through an http.RoundTripper: a test
// transport can answer in memory, so the list marks RoundTrip unproven and
// the call is silent.
func roundTripAll(transport http.RoundTripper, reqs []*http.Request) error {
	for _, req := range reqs {
		resp, err := transport.RoundTrip(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
	}
	return nil
}

// resolveAll resolves listen addresses in a loop: an address such as ":8080"
// or "10.0.0.1:53" resolves without a DNS lookup, so the list marks
// net.ResolveTCPAddr and net.LookupPort unproven and the calls are silent.
func resolveAll(addrs []string) ([]*net.TCPAddr, error) {
	resolved := make([]*net.TCPAddr, 0, len(addrs))
	for _, addr := range addrs {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if _, err := net.LookupPort("tcp", port); err != nil {
			return nil, err
		}
		a, err := net.ResolveTCPAddr("tcp", net.JoinHostPort(host, port))
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, a)
	}
	return resolved, nil
}

// pingEach borrows a pooled connection per item: db.Conn can reuse an idle
// connection and conn.Close returns it to the pool, so the list marks both
// unproven and they are silent.
func pingEach(ctx context.Context, db *sql.DB, n int) error {
	for i := 0; i < n; i++ {
		conn, err := db.Conn(ctx)
		if err != nil {
			return err
		}
		if err := conn.Close(); err != nil {
			return err
		}
	}
	return nil
}

// resolveEach resolves UDP and IP addresses and ports through a Resolver in
// a loop: each skips the lookup for an IP literal or a port number, so the
// list marks them unproven and the calls are silent.
func resolveEach(ctx context.Context, r *net.Resolver, hosts []string) error {
	for _, host := range hosts {
		if _, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, "53")); err != nil {
			return err
		}
		if _, err := net.ResolveIPAddr("ip", host); err != nil {
			return err
		}
		if _, err := r.LookupPort(ctx, "tcp", "443"); err != nil {
			return err
		}
	}
	return nil
}
