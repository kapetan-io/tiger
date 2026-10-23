// Package pin is tiger pin: freezing a named function's computed facts into
// //tiger: pins written at their targets.
package pin

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/kapetan-io/tiger/internal/command"
	"github.com/kapetan-io/tiger/internal/directive"
	"github.com/kapetan-io/tiger/internal/driver"
	"github.com/kapetan-io/tiger/internal/facts"
	"github.com/kapetan-io/tiger/internal/placement"
	"github.com/kapetan-io/tiger/internal/rules"
)

// The fact category pin consumes, and the rule whose blocking findings
// mean an existing pin disagrees with the code.
const catVariant = "TS-V01-facts"

func pinRule(ruleID string) bool {
	return ruleID == "TS-V01"
}

// Usage is tiger pin's entry in the command listing.
const Usage = `  tiger pin [-C dir] [--dry-run] <Name> [<Name>...] [packages]
      freeze each named function's computed facts into //tiger: pins
      written at their targets; exit 0 written or already satisfied,
      1 refusals, 2 could not proceed
`

// Run is tiger pin: resolve each named function, collect the facts the
// same analyzer run reports for it, and freeze them into //tiger:
// directives written at their targets. Insert-only by construction — the
// writer this command calls has no other operation.
func Run(args []string, streams command.Streams) int {
	flags := flag.NewFlagSet("tiger pin", flag.ContinueOnError)
	flags.SetOutput(streams.Stderr)
	flags.Usage = func() { command.PrintUsage(flags, Usage) }
	chdir := flags.String("C", ".", "run as if tiger was started in this directory")
	dryRun := flags.Bool(
		"dry-run", false, "print every directive that would be written and where, write nothing",
	)
	if err := flags.Parse(args); err != nil {
		return command.ExitOperational
	}
	given := splitArgs(flags.Args())
	if len(given.names) == 0 {
		flags.Usage()
		return command.ExitOperational
	}
	patterns := given.patterns
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	loaded, err := loadForPin(*chdir, patterns)
	if err != nil {
		fmt.Fprintf(streams.Stderr, "tiger pin: %v\n", err)
		return command.ExitOperational
	}
	targets, err := resolve(given.names, declaredFuncs(loaded))
	if err != nil {
		fmt.Fprintf(streams.Stderr, "tiger pin: %v\n", err)
		return command.ExitOperational
	}

	findings, err := driver.Check(*chdir, patterns, rules.Analyzers(), nil)
	if err != nil {
		fmt.Fprintf(streams.Stderr, "tiger pin: %v\n", err)
		return command.ExitOperational
	}
	root, err := filepath.Abs(*chdir)
	if err != nil {
		fmt.Fprintf(streams.Stderr, "tiger pin: %v\n", err)
		return command.ExitOperational
	}

	work, err := plan(root, targets, findings)
	if err != nil {
		fmt.Fprintf(streams.Stderr, "tiger pin: %v\n", err)
		return command.ExitOperational
	}
	sources, err := sourceFiles(work.writes)
	if err != nil {
		fmt.Fprintf(streams.Stderr, "tiger pin: %v\n", err)
		return command.ExitOperational
	}
	numberLines(work.writes, sources)
	if !*dryRun {
		if err := apply(work.writes, sources, streams); err != nil {
			return command.ExitOperational
		}
	}
	report(streams, targets, work.writes, work.refused)
	if len(work.refused) > 0 {
		return command.ExitFindings
	}
	return command.ExitClean
}

// invocation is pin's positional arguments, split into function names and
// package patterns.
type invocation struct {
	names    []string
	patterns []string
}

// splitArgs separates function names from package patterns: an argument
// that starts with "." or "/" or contains a "/" is a pattern, everything
// else — bare names, Store.Flush, (*Store).Flush — is a name.
func splitArgs(args []string) invocation {
	given := invocation{}
	for _, arg := range args {
		if strings.HasPrefix(arg, ".") || strings.HasPrefix(arg, "/") ||
			strings.Contains(arg, "/") {
			given.patterns = append(given.patterns, arg)
			continue
		}
		given.names = append(given.names, arg)
	}
	return given
}

// loadForPin parses the packages pin resolves names against and writes
// into. Syntax only — type checking and analysis stay the driver's job, so
// there is no second computation path.
func loadForPin(root string, patterns []string) ([]*packages.Package, error) {
	config := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedSyntax,
		Dir:   root,
		Tests: true,
	}
	loaded, err := packages.Load(config, patterns...)
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
	}
	if len(loaded) == 0 {
		return nil, fmt.Errorf("no packages matched")
	}
	return dedupeVariants(loaded), nil
}

// dedupeVariants mirrors the driver's package dedupe: drop the synthesized
// test main, and the plain variant of a package when its test-augmented
// variant is present, so no declaration is seen twice.
func dedupeVariants(loaded []*packages.Package) []*packages.Package {
	augmented := map[string]bool{}
	for _, pkg := range loaded {
		plain, _, found := strings.Cut(pkg.ID, " [")
		if found && !strings.HasSuffix(plain, "_test") {
			augmented[plain] = true
		}
	}
	kept := []*packages.Package{}
	for _, pkg := range loaded {
		if strings.HasSuffix(pkg.ID, ".test") || augmented[pkg.ID] {
			continue
		}
		kept = append(kept, pkg)
	}
	return kept
}

// declared is one function or method declaration in the loaded packages.
type declared struct {
	pkg  *packages.Package
	file *ast.File
	decl *ast.FuncDecl
}

// declaredFuncs enumerates every function and method the loaded packages
// declare — the universe a requested name resolves against.
func declaredFuncs(loaded []*packages.Package) []declared {
	universe := []declared{}
	for _, pkg := range loaded {
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				universe = append(universe, declared{pkg: pkg, file: file, decl: fn})
			}
		}
	}
	return universe
}

// receiver returns a method's receiver type name and pointer-ness, or ""
// for a plain function.
func receiver(decl *ast.FuncDecl) (name string, pointer bool) {
	if decl.Recv == nil || len(decl.Recv.List) == 0 {
		return "", false
	}
	expr := decl.Recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		pointer = true
		expr = star.X
	}
	if index, ok := expr.(*ast.IndexExpr); ok {
		expr = index.X
	}
	if index, ok := expr.(*ast.IndexListExpr); ok {
		expr = index.X
	}
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return "", pointer
	}
	return ident.Name, pointer
}

// qualified prints a declaration in the form a caller can retry with:
// pkgpath.Name, pkgpath.Recv.Name, or pkgpath.(*Recv).Name.
func qualified(d declared) string {
	recv, pointer := receiver(d.decl)
	if recv == "" {
		return d.pkg.PkgPath + "." + d.decl.Name.Name
	}
	if pointer {
		return d.pkg.PkgPath + ".(*" + recv + ")." + d.decl.Name.Name
	}
	return d.pkg.PkgPath + "." + recv + "." + d.decl.Name.Name
}

// resolved is one requested name bound to its single declaration.
type resolved struct {
	raw string
	declared
}

// resolve matches each requested name against the declared universe.
// Matching is exact: a bare name matches functions and methods by their own
// name, Store.Flush and (*Store).Flush match only that receiver's method.
// No match and more-than-one match are both errors that abort the whole
// run.
func resolve(names []string, universe []declared) ([]resolved, error) {
	targets := []resolved{}
	for _, name := range names {
		want := parseRequested(name)
		matched := []declared{}
		for _, d := range universe {
			if d.decl.Name.Name != want.method {
				continue
			}
			declRecv, _ := receiver(d.decl)
			if want.recv != "" && declRecv != want.recv {
				continue
			}
			matched = append(matched, d)
		}
		if len(matched) == 0 {
			return nil, fmt.Errorf(
				"name %q matched nothing%s", name, closest(want.method, universe),
			)
		}
		if len(matched) > 1 {
			lines := make([]string, 0, len(matched))
			for _, d := range matched {
				lines = append(lines, "  "+qualified(d))
			}
			sort.Strings(lines)
			return nil, fmt.Errorf(
				"name %q matched more than one function:\n%s", name, strings.Join(lines, "\n"),
			)
		}
		targets = append(targets, resolved{raw: name, declared: matched[0]})
	}
	return targets, nil
}

// requested is one parsed name argument: an optional receiver type and
// the function or method name.
type requested struct {
	recv   string
	method string
}

// parseRequested splits one name argument: "(*Store).Flush" and
// "Store.Flush" name Store's method, a bare "Flush" leaves the receiver
// empty.
func parseRequested(arg string) requested {
	before, after, found := strings.Cut(arg, ".")
	if !found {
		return requested{recv: "", method: arg}
	}
	return requested{
		recv:   strings.TrimSuffix(strings.TrimPrefix(before, "(*"), ")"),
		method: after,
	}
}

// closest names the declared function nearest to a mistyped name, so the
// error carries what to retry with.
func closest(name string, universe []declared) string {
	best, bestDistance := "", 4
	for _, d := range universe {
		distance := editDistance(pair{spelled: name, candidate: d.decl.Name.Name})
		if distance < bestDistance {
			best, bestDistance = d.decl.Name.Name, distance
		}
	}
	if best == "" {
		return ""
	}
	return fmt.Sprintf(" (closest: %s)", best)
}

// pair is the two names editDistance compares: what the caller spelled
// and one declared candidate.
type pair struct {
	spelled   string
	candidate string
}

// editDistance is the Levenshtein distance between a pair's names.
func editDistance(p pair) int {
	previous := make([]int, len(p.candidate)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(p.spelled); i++ {
		current := make([]int, len(p.candidate)+1)
		current[0] = i
		for j := 1; j <= len(p.candidate); j++ {
			cost := 1
			if p.spelled[i-1] == p.candidate[j-1] {
				cost = 0
			}
			current[j] = min(previous[j]+1, current[j-1]+1, previous[j-1]+cost)
		}
		previous = current
	}
	return previous[len(p.candidate)]
}

// write is one node's pending insertion, with the output lines it prints.
type write struct {
	target   *resolved
	absFile  string
	relFile  string
	origLine int
	insert   placement.Insertion
	dirs     []directive.Directive
	printed  []string
}

// refusal is one target pin declined to write, with the block it prints.
type refusal struct {
	target *resolved
	block  string
}

// planned is one run's computed work: every insertion, decided before any
// file is opened for writing (behavioral constraint 7), and every refusal.
type planned struct {
	writes  []write
	refused []refusal
}

// plan computes the run's work. A fact message that does not yield a
// parseable directive is an operational error for the whole run — the
// analyzer-to-pin contract is broken, so every collected fact is suspect.
func plan(root string, targets []resolved, found []driver.Finding) (planned, error) {
	rel := relativizer(root)
	writes := []write{}
	refused := []refusal{}
	for i := range targets {
		target := &targets[i]
		relFile := rel(target.position().Filename)
		if ast.IsGenerated(target.file) {
			refused = append(refused, refusal{target: target, block: fmt.Sprintf(
				"refused %s (%s:%d): target is in a generated file\n",
				target.raw, relFile, target.position().Line,
			)})
			continue
		}
		if blocking := onTarget(target, relFile, found, isBlocking); len(blocking) > 0 {
			refused = append(refused, refusal{
				target: target, block: refusalBlock(target, relFile, blocking),
			})
			continue
		}
		found, err := targetWrites(target, relFile, onTarget(target, relFile, found, isFact))
		if err != nil {
			return planned{}, err
		}
		writes = append(writes, found...)
	}
	return planned{writes: writes, refused: refused}, nil
}

func (r *resolved) position() token.Position {
	return r.pkg.Fset.Position(r.decl.Pos())
}

// lineSpan is the inclusive line range a finding must fall in to be "on"
// a target: the doc comment (a disagreeing pin's finding points at the pin
// line) through the declaration's end.
type lineSpan struct {
	first int
	last  int
}

func (r *resolved) span() lineSpan {
	start := r.decl.Pos()
	if r.decl.Doc != nil {
		start = r.decl.Doc.Pos()
	}
	return lineSpan{
		first: r.pkg.Fset.Position(start).Line,
		last:  r.pkg.Fset.Position(r.decl.End()).Line,
	}
}

func isBlocking(category string) bool {
	entry, known := rules.ByCategory(category)
	return known && entry.Severity == rules.SeverityBlocking
}

func isFact(category string) bool {
	_, known := rules.ByFact(category)
	return known
}

// onTarget filters the run's findings to those positioned on target and
// passing keep.
func onTarget(
	target *resolved, relFile string, found []driver.Finding, keep func(string) bool,
) []driver.Finding {
	span := target.span()
	matched := []driver.Finding{}
	for _, finding := range found {
		if finding.Position.Filename != relFile {
			continue
		}
		if finding.Position.Line < span.first || finding.Position.Line > span.last {
			continue
		}
		if !keep(finding.Category) {
			continue
		}
		matched = append(matched, finding)
	}
	return matched
}

// refusalBlock renders one refused target: the reason, the blocking
// findings, and — when the findings mean an existing pin disagrees with
// the code — the pinned lines and the two exits the specification defines.
func refusalBlock(target *resolved, relFile string, blocking []driver.Finding) string {
	pinned := existingPins(target)
	disagreement := false
	for _, finding := range blocking {
		entry, _ := rules.ByCategory(finding.Category)
		if pinRule(entry.RuleID) && len(pinned) > 0 {
			disagreement = true
		}
	}
	block := strings.Builder{}
	reason := "target carries a blocking finding"
	if disagreement {
		reason = "existing pin disagrees with the computed fact"
	}
	fmt.Fprintf(&block, "refused %s (%s:%d): %s\n",
		target.raw, relFile, target.position().Line, reason)
	if disagreement {
		for _, pin := range pinned {
			fmt.Fprintf(&block, "  pinned: %s\n", directive.Format(pin))
		}
	}
	for _, finding := range blocking {
		fmt.Fprintf(&block, "  %s: %s\n", finding.Position, finding.Message)
	}
	if disagreement {
		block.WriteString("  change the code, or edit the pin by hand\n")
	}
	return block.String()
}

// existingPins collects the variant pins already on target's loops.
func existingPins(target *resolved) []directive.Directive {
	set := placement.Collect(target.pkg.Fset, []*ast.File{target.file})
	collected := []directive.Directive{}
	ast.Inspect(target.decl, func(node ast.Node) bool {
		if loop, ok := node.(*ast.ForStmt); ok {
			for _, pin := range set.At(loop, "variant") {
				collected = append(collected, pin.Directive)
			}
		}
		return true
	})
	return collected
}

// targetWrites turns one target's variant facts into writes, each on its
// own loop.
func targetWrites(
	target *resolved, relFile string, factFindings []driver.Finding,
) ([]write, error) {
	writes := []write{}
	for _, finding := range factFindings {
		if finding.Category != catVariant {
			continue
		}
		extracted, err := facts.Extract(finding.Message)
		if err != nil {
			return nil, err
		}
		loop := loopAtLine(target, finding.Position.Line)
		if loop == nil {
			return nil, fmt.Errorf(
				"no loop at %s for fact %q", finding.Position, finding.Message,
			)
		}
		writes = append(writes, newWrite(target, relFile, loop, []directive.Directive{extracted}))
	}
	return writes, nil
}

func newWrite(target *resolved, relFile string, node ast.Node, dirs []directive.Directive) write {
	fset := target.pkg.Fset
	return write{
		target:   target,
		absFile:  fset.Position(node.Pos()).Filename,
		relFile:  relFile,
		origLine: fset.Position(node.Pos()).Line,
		dirs:     dirs,
	}
}

// loopAtLine finds the for statement in target's body starting on line.
func loopAtLine(target *resolved, line int) ast.Node {
	var found ast.Node
	ast.Inspect(target.decl, func(node ast.Node) bool {
		loop, ok := node.(*ast.ForStmt)
		if ok && found == nil && target.pkg.Fset.Position(loop.Pos()).Line == line {
			found = loop
		}
		return found == nil
	})
	return found
}

// sourceFiles reads each file the run writes into, exactly once.
func sourceFiles(writes []write) (map[string][]byte, error) {
	files := map[string][]byte{}
	//tiger:batched the filesystem offers no batch read; each target file is read once
	for _, w := range writes {
		if _, done := files[w.absFile]; done {
			continue
		}
		content, err := os.ReadFile(w.absFile)
		if err != nil {
			return nil, err
		}
		files[w.absFile] = content
	}
	return files, nil
}

// numberLines computes each write's insertion and the final line number of
// every inserted directive, accounting for lines earlier insertions add to
// the same file.
func numberLines(writes []write, sources map[string][]byte) {
	for i := range writes {
		w := &writes[i]
		w.insert = placement.Insert(placement.Target{
			Fset: w.target.pkg.Fset, File: w.target.file, Src: sources[w.absFile],
			Node: w.nodeAtOrigin(), Directives: w.dirs,
		})
	}
	byFile := map[string][]*write{}
	for i := range writes {
		byFile[writes[i].absFile] = append(byFile[writes[i].absFile], &writes[i])
	}
	for _, file := range slices.Sorted(maps.Keys(byFile)) {
		group := byFile[file]
		sort.Slice(group, func(i, j int) bool {
			return group[i].insert.Offset < group[j].insert.Offset
		})
		shift := 0
		for _, w := range group {
			total := strings.Count(w.insert.Text, "\n")
			first := total - len(w.dirs)
			for i, d := range w.dirs {
				w.printed = append(w.printed, fmt.Sprintf(
					"%s:%d: %s", w.relFile, w.origLine+shift+first+i, directive.Format(d),
				))
			}
			shift += total
		}
	}
}

// nodeAtOrigin re-finds the node a write inserts at: the declaration
// itself, or the loop starting on the recorded line.
func (w *write) nodeAtOrigin() ast.Node {
	if w.origLine == w.target.position().Line {
		return w.target.decl
	}
	return loopAtLine(w.target, w.origLine)
}

// apply splices every insertion into its file, descending by offset so no
// insertion invalidates another's, and replaces each file atomically. A
// failure names the files already written (behavioral constraint 7).
func apply(writes []write, sources map[string][]byte, streams command.Streams) error {
	byFile := map[string][]write{}
	for _, w := range writes {
		byFile[w.absFile] = append(byFile[w.absFile], w)
	}
	written := []string{}
	for _, file := range slices.Sorted(maps.Keys(byFile)) {
		group := byFile[file]
		sort.Slice(group, func(i, j int) bool {
			return group[i].insert.Offset > group[j].insert.Offset
		})
		updated := append([]byte{}, sources[file]...)
		for _, w := range group {
			updated = append(updated[:w.insert.Offset],
				append([]byte(w.insert.Text), updated[w.insert.Offset:]...)...)
		}
		if err := replace(file, updated); err != nil {
			fmt.Fprintf(streams.Stderr, "tiger pin: writing %s: %v\n", file, err)
			if len(written) > 0 {
				fmt.Fprintf(streams.Stderr,
					"tiger pin: already written: %s\n", strings.Join(written, ", "))
			}
			return err
		}
		written = append(written, group[0].relFile)
	}
	return nil
}

// replace writes content to a temporary file beside path and renames it
// into place, preserving the original mode.
func replace(path string, content []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".tiger-pin-*")
	if err != nil {
		return err
	}
	name := temp.Name()
	if _, err := temp.Write(content); err != nil {
		return errors.Join(err, temp.Close(), os.Remove(name))
	}
	if err := temp.Close(); err != nil {
		return errors.Join(err, os.Remove(name))
	}
	if err := os.Chmod(name, info.Mode()); err != nil {
		return errors.Join(err, os.Remove(name))
	}
	return os.Rename(name, path)
}

// report prints, per target in the requested order: the refusal block, then
// the written lines.
func report(streams command.Streams, targets []resolved, writes []write, refused []refusal) {
	for i := range targets {
		target := &targets[i]
		for _, r := range refused {
			if r.target == target {
				fmt.Fprint(streams.Stdout, r.block)
			}
		}
		lines := []string{}
		for _, w := range writes {
			if w.target == target {
				lines = append(lines, w.printed...)
			}
		}
		sort.Strings(lines)
		for _, line := range lines {
			fmt.Fprintln(streams.Stdout, line)
		}
	}
}

// relativizer mirrors the driver's finding relativization, bound to one
// root, so target positions compare against finding positions byte for
// byte.
func relativizer(root string) func(string) string {
	return func(filename string) string {
		relative, err := filepath.Rel(root, filename)
		if err != nil || strings.HasPrefix(relative, "..") {
			return filename
		}
		return filepath.ToSlash(relative)
	}
}
