// Command fullcoverage runs the call 8 TS-T02 prototype standalone, or as
// a vet tool: go vet -vettool=$(which fullcoverage) [-sites] ./...
package main

import (
	"fullcoverage/fullcoverage"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(fullcoverage.Analyzer) }
