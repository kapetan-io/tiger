// The TS-M10 compliant forms: a //tiger:batched loop whose per-item IO is
// what the outside world forces, the same IO hoisted above the loop
// instead, IO outside any loop entirely, and an in-memory bytes.Buffer
// write inside a loop — proving the classifier resolves package identity
// through pass.TypesInfo rather than matching on the io.Writer interface or
// the Write method name.
package fixture

import (
	"bytes"
	"net/http"
	"os"
)

// readAllBatched keeps the per-item read in the loop, but states the world
// constraint that forces it.
func readAllBatched(paths []string) error {
	//tiger:batched paths originate from an external drop directory; each read is required I/O
	for i := 0; i < len(paths); i++ {
		_, err := os.ReadFile(paths[i])
		if err != nil {
			return err
		}
	}
	return nil
}

// readOnceThenLoop hoists the read above the loop: it runs once, not once
// per iteration.
func readOnceThenLoop(path string, counts []int) ([]byte, int) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0
	}
	total := 0
	for range counts {
		total++
	}
	return data, total
}

// readOne calls os.ReadFile outside any loop.
func readOne(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// bufferAll writes to an in-memory bytes.Buffer inside a loop: Buffer.Write
// resolves to package "bytes", never to the allowlist, and is not matched
// by name or by the io.Writer interface it happens to satisfy.
func bufferAll(chunks [][]byte) []byte {
	buf := &bytes.Buffer{}
	for _, chunk := range chunks {
		buf.Write(chunk)
	}
	return buf.Bytes()
}

// setHeaders calls http.Header.Set in a loop: the callee lives in net/http
// (an allowlist package), but its receiver's underlying type is a map — a
// pure container that cannot hold a connection or descriptor — so the
// call is an in-memory write, not IO. Only struct-backed receivers and
// package-level functions from an allowlist package are flagged.
func setHeaders(req *http.Request, headers map[string]string) {
	for k, v := range headers {
		req.Header.Set(k, v)
	}
}
