// The TS-M10 IO-call forms beyond failure.go: each listed IO call in a loop
// body is reported, whether it is a package function, a method on a
// concrete type, or a method promoted from an unexported embedded type.
package fixture

import (
	"database/sql"
	"net"
	"net/http"
	"os"
)

// dialAll dials each address once per iteration.
func dialAll(addrs []string) error {
	for _, addr := range addrs {
		conn, err := net.Dial("tcp", addr) // want `TS-M10: this call does IO`
		if err != nil {
			return err
		}
		conn.Close()
	}
	return nil
}

// lookupAll resolves each host once per iteration.
func lookupAll(hosts []string) error {
	for _, host := range hosts {
		if _, err := net.LookupHost(host); err != nil { // want `TS-M10: this call does IO`
			return err
		}
	}
	return nil
}

// doAll sends each request through the client once per iteration.
func doAll(client *http.Client, reqs []*http.Request) error {
	for _, req := range reqs {
		resp, err := client.Do(req) // want `TS-M10: this call does IO`
		if err != nil {
			return err
		}
		resp.Body.Close()
	}
	return nil
}

// queryEach runs one query per id inside a for body.
func queryEach(db *sql.DB, ids []int) error {
	for i := 0; i < len(ids); i++ {
		rows, err := db.Query("SELECT 1 WHERE id = ?", ids[i]) // want `TS-M10: this call does IO`
		if err != nil {
			return err
		}
		rows.Close() // want `TS-M10: this call does IO`
	}
	return nil
}

// readChunks reads the file once per chunk.
func readChunks(file *os.File, chunks [][]byte) error {
	for _, chunk := range chunks {
		if _, err := file.Read(chunk); err != nil { // want `TS-M10: this call does IO`
			return err
		}
	}
	return nil
}

// writeTCP writes through a *net.TCPConn, whose Write is promoted from an
// unexported embedded type in net.
func writeTCP(conn *net.TCPConn, chunks [][]byte) error {
	for _, chunk := range chunks {
		if _, err := conn.Write(chunk); err != nil { // want `TS-M10: this call does IO`
			return err
		}
	}
	return nil
}

// openAllDir opens each name through an http.Dir, which opens a file on
// disk even though Dir is a string type.
func openAllDir(dir http.Dir, names []string) error {
	for _, name := range names {
		file, err := dir.Open(name) // want `TS-M10: this call does IO`
		if err != nil {
			return err
		}
		file.Close()
	}
	return nil
}
