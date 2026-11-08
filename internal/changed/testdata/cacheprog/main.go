// Command cacheprog is the smallest GOCACHEPROG program Go accepts. It keeps
// every put in a directory of its own and answers a get from what it kept, as
// a remote build cache does.
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type request struct {
	ID       int64
	Command  string
	ActionID []byte
	OutputID []byte
	BodySize int64
}

type response struct {
	ID            int64
	KnownCommands []string `json:",omitempty"`
	Miss          bool     `json:",omitempty"`
	OutputID      []byte   `json:",omitempty"`
	Size          int64    `json:",omitempty"`
	DiskPath      string   `json:",omitempty"`
}

func main() {
	dir := os.Args[1]
	in := bufio.NewReaderSize(os.Stdin, 1<<20)
	out := json.NewEncoder(os.Stdout)
	decode := json.NewDecoder(in)
	if err := out.Encode(response{KnownCommands: []string{"get", "put", "close"}}); err != nil {
		os.Exit(1)
	}
	for {
		var req request
		if err := decode.Decode(&req); err != nil {
			return
		}
		switch req.Command {
		case "get":
			if err := out.Encode(lookup(dir, req)); err != nil {
				os.Exit(1)
			}
		case "put":
			var body []byte
			if req.BodySize > 0 {
				var encoded string
				if err := decode.Decode(&encoded); err != nil {
					os.Exit(1)
				}
				var err error
				if body, err = base64.StdEncoding.DecodeString(encoded); err != nil {
					os.Exit(1)
				}
			}
			path := filepath.Join(dir, "o-"+base64.RawURLEncoding.EncodeToString(req.OutputID))
			if err := writeAtomic(path, body); err != nil {
				os.Exit(1)
			}
			index := base64.StdEncoding.EncodeToString(req.OutputID) + " " + strconv.Itoa(len(body))
			if err := writeAtomic(actionPath(dir, req.ActionID), []byte(index)); err != nil {
				os.Exit(1)
			}
			if err := out.Encode(response{ID: req.ID, DiskPath: path}); err != nil {
				os.Exit(1)
			}
		case "close":
			_ = out.Encode(response{ID: req.ID})
			return
		}
	}
}

func actionPath(dir string, action []byte) string {
	return filepath.Join(dir, "a-"+base64.RawURLEncoding.EncodeToString(action))
}

func lookup(dir string, req request) response {
	index, err := os.ReadFile(actionPath(dir, req.ActionID))
	if err != nil {
		return response{ID: req.ID, Miss: true}
	}
	encoded, size, _ := strings.Cut(string(index), " ")
	output, err := base64.StdEncoding.DecodeString(encoded)
	length, sizeErr := strconv.ParseInt(size, 10, 64)
	if err != nil || sizeErr != nil {
		return response{ID: req.ID, Miss: true}
	}
	path := filepath.Join(dir, "o-"+base64.RawURLEncoding.EncodeToString(output))
	return response{ID: req.ID, OutputID: output, Size: length, DiskPath: path}
}

// writeAtomic keeps a reader in another cache program process, which shares
// this directory, from seeing half a file.
func writeAtomic(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), "tmp-")
	if err != nil {
		return err
	}
	_, err = temp.Write(data)
	if err = errors.Join(err, temp.Close()); err != nil {
		return errors.Join(err, os.Remove(temp.Name()))
	}
	return os.Rename(temp.Name(), path)
}
