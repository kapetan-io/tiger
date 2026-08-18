// Package helper carries the effect the pin in core never declared: the
// widening is introduced here, a package boundary away from the pin that
// fails.
package helper

import "net"

// Ping opens and closes one connection to the probe address.
func Ping() error {
	conn, err := net.Dial("tcp", "localhost:1")
	if err != nil {
		return err
	}
	return conn.Close()
}
