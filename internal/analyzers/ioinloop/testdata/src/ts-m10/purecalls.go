// The TS-M10 pure-call forms: calls in os, net and database/sql that do no
// IO stay silent inside a loop body, because TS-M10 flags a call only when
// it is on the list of IO calls, not because its package does IO elsewhere.
package fixture

import (
	"database/sql"
	"net"
	"os"
	"time"
)

// splitAll splits and rejoins host:port text in a loop: net.SplitHostPort,
// net.JoinHostPort and net.ParseIP parse strings and touch no socket.
func splitAll(addrs []string) []string {
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		host, port, err := net.SplitHostPort(addr)
		if err != nil || net.ParseIP(host) == nil {
			continue
		}
		out = append(out, net.JoinHostPort(host, port))
	}
	return out
}

// envAll reads the process environment and pid in a loop: os.Getenv and
// os.Getpid read process state, not a disk or a network.
func envAll(keys []string) map[string]string {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		out[key] = os.Getenv(key)
		_ = os.Getpid()
	}
	return out
}

// scanAll scans each row of an already-run query: the IO is rows.Next in
// the loop condition, and rows.Scan copies the current row in memory.
func scanAll(rows *sql.Rows) ([]int, error) {
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// namedArgs builds named query arguments in a loop: sql.Named only wraps a
// name and a value.
func namedArgs(values map[string]any) []any {
	args := make([]any, 0, len(values))
	for name, value := range values {
		args = append(args, sql.Named(name, value))
	}
	return args
}

// deadlineAll sets a deadline on each connection: SetDeadline stores a
// timer and sends nothing.
func deadlineAll(conns []net.Conn, at time.Time) error {
	for _, conn := range conns {
		if err := conn.SetDeadline(at); err != nil {
			return err
		}
	}
	return nil
}

// unixAddrs builds socket addresses in a loop: net.ResolveUnixAddr only
// fills a struct, so the list marks it no IO and the call is silent.
func unixAddrs(paths []string) ([]*net.UnixAddr, error) {
	addrs := make([]*net.UnixAddr, 0, len(paths))
	for _, path := range paths {
		a, err := net.ResolveUnixAddr("unix", path)
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, a)
	}
	return addrs, nil
}
