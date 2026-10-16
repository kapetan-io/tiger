// Package fakedb stands in for a database driver: it lives outside the
// fixture module and imports os, so it reaches syscall like a real driver.
package fakedb

import "os"

type Pool struct {
	file *os.File
}

func Open(path string) (*Pool, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	return &Pool{file: file}, nil
}

func (p *Pool) Exec(query string) error {
	_, err := p.file.WriteString(query)
	return err
}

func (p *Pool) Query(key string) (string, error) {
	data, err := os.ReadFile(p.file.Name())
	return string(data) + key, err
}

func (p *Pool) Close() error {
	return p.file.Close()
}

// Ping and Pong call each other, and only Ping reaches os, so a walk that
// starts at Ping meets Ping again while still inside it.
func Ping(n int) error {
	if n > 0 {
		return Pong(n - 1)
	}
	_, err := os.Stat(".")
	return err
}

func Pong(n int) error {
	return Ping(n)
}
