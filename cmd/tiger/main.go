// The tiger command checks Tiger Go code against the rules no off-the-shelf
// linter covers, and audits the golangci-lint configuration that enforces
// the rest.
package main

import (
	"os"

	"github.com/kapetan-io/tiger/internal/cli"
	"github.com/kapetan-io/tiger/internal/command"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], command.Streams{Stdout: os.Stdout, Stderr: os.Stderr}))
}
