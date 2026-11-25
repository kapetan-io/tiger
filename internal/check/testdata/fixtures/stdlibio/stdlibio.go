// Package stdlibio calls standard library functions inside loop bodies:
// the pure calls must stay silent under TS-M10 and the IO calls must each
// be reported.
package stdlibio

import (
	"database/sql"
	"net"
	"net/http"
	"os"
	"time"
)

func Pure(addrs []string, rows *sql.Rows, conns []net.Conn, at time.Time) []any {
	var out []any
	for _, addr := range addrs {
		host, port, _ := net.SplitHostPort(addr)
		out = append(out, net.JoinHostPort(host, port), net.ParseIP(host))
		out = append(out, os.Getenv(addr), os.Getpid(), sql.Named(addr, host))
	}
	for rows.Next() {
		var id int
		_ = rows.Scan(&id)
		out = append(out, id)
	}
	for _, conn := range conns {
		_ = conn.SetDeadline(at)
	}
	return out
}

func IO(paths []string, client *http.Client, req *http.Request, db *sql.DB, file *os.File) {
	for _, path := range paths {
		_, _ = os.ReadFile(path)
		_, _ = net.Dial("tcp", path)
		_, _ = net.LookupHost(path)
		_, _ = client.Do(req)
		_, _ = db.Query(path)
		_, _ = file.Read(nil)
	}
}
