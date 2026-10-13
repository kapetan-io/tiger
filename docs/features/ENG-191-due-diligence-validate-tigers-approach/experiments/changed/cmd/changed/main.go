// Command changed prints which owners in a Go module started or stopped
// talking to another package between two versions.
//
//	changed <base-dir> <head-dir>
//	changed -base <rev> [module-dir]
//	changed -edges <base-dir> <head-dir>
//
// With -base, the base is rev of the git repository holding module-dir
// (default "."), and the head is module-dir as it is on disk.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"changed/changed"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "changed:", err)
		os.Exit(2)
	}
}

func run() error {
	rev := flag.String("base", "", "git revision to compare the module directory against")
	mermaid := flag.Bool("mermaid", false, "print a Mermaid graph of the changed edges after the report")
	list := flag.Bool("edges", false, "print every edge of the head instead of the report")
	byFunction := flag.Bool("by-function", false, "keep an outside call only when the function itself reaches syscall")
	mixed := flag.Bool("mixed", false, "judge outside free functions by their body and outside methods by their package")
	flag.Parse()

	filter := changed.ByPackage
	if *byFunction {
		filter = changed.ByFunction
	}
	if *mixed {
		filter = changed.Mixed
	}
	var base, head changed.Edges
	var err error
	switch {
	case *rev != "" && flag.NArg() <= 1:
		dir := "."
		if flag.NArg() == 1 {
			dir = flag.Arg(0)
		}
		if base, err = changed.LoadRevision(dir, *rev, filter); err != nil {
			return err
		}
		if head, err = changed.Load(dir, filter); err != nil {
			return err
		}
	case *rev == "" && flag.NArg() == 2:
		if base, err = changed.Load(flag.Arg(0), filter); err != nil {
			return err
		}
		if head, err = changed.Load(flag.Arg(1), filter); err != nil {
			return err
		}
	default:
		return fmt.Errorf("usage: changed <base-dir> <head-dir> | changed -base <rev> [module-dir]")
	}
	if *list {
		for _, edge := range head.List() {
			fmt.Printf("%s -> %s: %s\n", edge.Owner, edge.Target, strings.Join(edge.Calls, "; "))
		}
		return nil
	}
	changes := changed.Diff(base, head)
	fmt.Print(changed.Report(changes))
	if *mermaid && len(changes) > 0 {
		fmt.Print("\n```mermaid\n" + changed.Mermaid(changes) + "```\n")
	}
	return nil
}
