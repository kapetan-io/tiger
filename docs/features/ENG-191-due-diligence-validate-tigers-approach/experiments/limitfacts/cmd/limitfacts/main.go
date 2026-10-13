package main

import (
	"limitfacts/limitfacts"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(limitfacts.Analyzer) }
