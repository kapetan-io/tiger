# tiger pin Blueprint

Source specification: the Tiger Go Specification (`docs/Tiger Specification.md`), the pin lifecycle
in Part I and the output contract in Part V ("a `tiger pin <func>` command can write the
declaration mechanically").

Builds on the SSA wave (`docs/features/ENG-150-ssa-wave/blueprint.md`), which computes the facts
this command freezes and, by design, prints them in exact pin syntax so freezing one is a paste.

## Objective

Build `tiger pin`: the subcommand that turns an analyzer-computed fact into a blocking contract by
writing it into the source as a `//tiger:` directive.

The SSA wave left this a mechanical job on purpose. `tiger check --show-facts` already prints every
computed effect set, frame, and synthesized variant as the exact line a developer would paste, and
`directive.Format` is the single site that produces that text. What is missing is the last step:
finding the right line in the right file and putting the directive there, without a human
transcribing it.

The work is therefore not computation. It is **placement and policy** — where a directive goes so
it binds to the node whose fact it froze, and what the command refuses to do.

## Mental Model

Three ideas carry the design.

1. **`pin` is a paste, mechanized.** The spec's fact lifecycle is computed → reported → pinned, and
   report output and pin syntax are one format precisely so the last arrow is a copy. `pin` copies
   for you. It computes nothing the analyzers did not already compute and formats nothing the
   grammar package does not already format. If `pin` ever needs to build a `//tiger:` line itself,
   the design is wrong.

2. **`pin` only ever inserts.** It has no path that edits or deletes an existing directive. That is
   not a limitation to lift later — it is the feature. A pin's whole value is that the analyzer can
   disagree with it; a tool that rewrites a pin to match the code deletes the disagreement and with
   it the contract. Changing a pin stays a human edit visible in the diff, which is what puts a
   reviewer in front of the new promise.

3. **Placement is the mirror of collection.** `pins.Collect` is the one site that decides which node
   a directive binds to: the node starting on the line after the comment group ends. Writing is the
   same rule run backwards, and it lives in one place for the same reason formatting does. The test
   that proves it is not a golden file but a round trip — write a pin, collect it back, assert it
   landed on the node whose fact it came from.

## Core Design Principles

Wave-1 and SSA-wave principles carry over unchanged. This feature adds:

- **Insert only, never rewrite.** The writer exposes insertion and nothing else. A stale pin is
  reported, never fixed. This is ADR-0003's admission test applied to a command instead of a
  directive: `tiger pin --update` would be a one-command dismissal of any blocking pin violation,
  which is a cheaper path to green than the fix.
- **One placement site.** All three verbs go through one writer, symmetric with `pins.Collect` on
  the read side and `directive.Format` on the format side.
- **Sparse pins stay sparse.** `pin` takes function names. There is no sweep, because nothing in the
  dialect yet requires a package's whole exported surface to be pinned. The closed-dispatch trigger
  (TS-K03 under TS-P01) is the event that would justify one, and its analyzer is not built.
- **Never freeze a fact from non-conforming code.** A target carrying a blocking finding is refused.
  A fact computed from code the dialect rejects is not a contract worth writing down.
- **Refusals are loud and specific.** Every refusal names the target, the reason, and the two exits.
  A silent skip would let an agent believe it pinned something it did not.

## Correctness Constraints

### State Invariants

1. **Every directive `pin` writes is `directive.Format` output.** `pin` holds `directive.Directive`
   values and never assembles `//tiger:` text. Enforced structurally: the writer's input is parsed
   directives, not strings.

2. **A written pin binds to the node whose fact it froze.** After `pin` writes, re-parsing the file
   and running `pins.Collect` attaches that directive to the same declaration or loop the fact
   diagnostic pointed at. Violation is caught by a round-trip property test.

3. **`pin` never removes, replaces, or reorders an existing directive.** The writer offers insertion
   only; there is no delete or replace operation to call. Every byte of the original file outside
   the inserted lines survives unchanged.

4. **`pin` is idempotent.** A second run over the same targets writes nothing: an existing pin of
   that verb whose arguments equal the computed fact is a satisfied target, not a refusal and not a
   duplicate line.

5. **A file `pin` writes still parses as Go.** Inserting whole comment lines cannot break parsing,
   and `pin` runs no printer over the file, so unrelated code is never reformatted.

6. **A pinned target checks clean.** After `tiger pin X` succeeds on X, `tiger check` reports no pin
   finding on X. This is the composition of invariants 1 and 2 with the analyzers' bidirectional
   comparison, and it is asserted end to end.

### Behavioral Constraints

7. **`pin` never leaves a file partially written.** Every edit for a run is computed before any file
   is opened for writing, and each file is written atomically. A failure during the write phase
   names the files already written.

8. **`pin` never freezes a fact from a target carrying a blocking finding.** The target is refused
   and the finding is printed.

9. **The pinned text equals the computed text exactly.** `pin` never widens a fact to be safe or
   narrows one to be tidy. Widening would recreate the defensive superset TS-F01's bidirectional
   comparison exists to prevent.

10. **`pin` writes no file the caller did not name a target in.** Pinning `Flush` never touches a
    helper `Flush` calls, in the same package or another one.

## Acceptance Criteria

- `tiger pin Append ./...` over the existing `showfacts` fixture writes exactly the directives that
  fixture's `--show-facts` golden output prints for `Append`, and the resulting file bytes match a
  committed expectation.
- Running `tiger pin` a second time over the same tree writes nothing, prints nothing new, and exits
  0 (invariant 4).
- `tiger check` over a tree `pin` just wrote exits clean, with no pin findings on the pinned targets
  (invariant 6).
- A property test over generated placements: for a declaration with no doc comment, one with a doc
  comment, one whose doc comment already ends in a `//tiger:batched` line, and a loop nested inside
  a body, the written pin is collected back by `pins.Collect` bound to the intended node
  (invariant 2).
- `tiger pin Flush` on a target whose existing `//tiger:effects` pin disagrees with the computed set
  writes nothing for that target, prints the pinned line and the computed line, and exits 1.
- `tiger pin A B C` where B is refused writes A and C, prints B's refusal, and exits 1
  (partial-progress contract).
- `tiger pin flush` on an unexported function writes the variant pins for loops in its body, writes
  no effects or frame pin, and prints the reason.
- `tiger pin Flush` where two packages in the pattern export `Flush` writes nothing, prints both
  matches in qualified form, and exits 2.
- `tiger pin --dry-run` prints every line it would write with its insertion point and leaves the
  tree byte-identical.
- A target inside a generated file is skipped with a printed reason, matching the driver's existing
  `ast.IsGenerated` policy.
- The tiger repository dogfoods the command: `tiger pin` on a function in this tree produces a
  directive that `tiger check` then accepts.

Success is proven by designed-response fixtures, per surface-testing. There is no post-ship metrics
section: this is developer infrastructure whose only consumer is the analyzer suite it feeds.

## Scope

### In Scope

**The `pin` subcommand**

```
tiger pin [-C dir] [--dry-run] <Name> [<Name>...] [packages]
```

- `-C dir` — run as if started in this directory. Same flag and meaning as `tiger check`.
- `--dry-run` — print every directive that would be written and where, write nothing.
- `<Name>` — one or more function or method names. A bare name (`Flush`) or a method in Go doc form
  (`(*Store).Flush`, `Store.Flush`). At least one is required; `pin` with no name prints usage and
  exits 2.
- `packages` — package patterns to search, defaulting to `./...` as `check` does.

**What one named target writes**

For an exported function or method:
- a `//tiger:effects` pin on the declaration, from the computed effect set;
- a `//tiger:frame` pin on the declaration, from the computed frame;
- a `//tiger:variant` pin above each loop **in that function's own body** for which the analyzer
  synthesized a variant.

For an unexported function: the variant pins only. Effects and frame pins attach to exported
functions and methods only (SSA-wave invariant 3), and a variant pin attaches to its loop wherever
the loop lives — the spec's stated placement exception.

**Placement**

The pin lines go at the end of the target node's doc comment group, so the group still ends on the
line before the node and `pins.Collect` binds them to it. Three cases:

- **No doc comment** — the pin lines go directly above the declaration or loop.
- **Doc comment ending in prose** — a bare `//` separator line, then the pin lines. This matches the
  repo's own pin fixture and Go's convention for `//go:` directives in a doc group.
- **Doc comment already ending in directives** — the pin lines append with no separator.

When a target takes more than one verb, they stack in the grammar's vocabulary order: `effects`,
then `frame`.

**Refusals**

A refused target is printed and skipped; other targets still write. The run exits 1.

- The target's existing pin of that verb disagrees with the computed fact. Prints the pinned line
  and the computed line, and names the two exits: change the code, or edit the pin by hand.
- The target carries a blocking finding. Prints the finding.
- The target is in a generated file.

**Errors**

The run writes nothing and exits 2.

- A name matches no function in the searched packages.
- A name matches more than one. Every match prints in qualified form.
- Package loading or an analyzer fails — the driver's existing terminal-error contract.
- A reported fact's message does not extract into a parseable directive. This is an internal
  invariant violation between an analyzer and the shared extractor, not a target-level refusal;
  `pin` prints the offending message and exits 2 without writing anything.

**The placement writer**

A new component alongside `pins.Collect`, in the same package, owning insertion for all three verbs.
Its contract:

- **Responsibility.** Given a node's position and the directives to attach to it, produce the text
  edits that make `pins.Collect` bind those directives to that node.
- **Inputs and outputs, conceptually.** Parsed directives and a target node in a parsed file, in;
  positioned insertions, out. It never returns modified file content — applying edits is the
  command's job, so the writer stays free of file IO.
- **Invariants.** Insertion only; the emitted edits are additive and never overlap existing bytes.
  Output is `directive.Format` text and nothing else.
- **Error categories.** A target node the file does not contain is a programming error, not a
  runtime condition. There is no partial-placement result.

Method signatures are the implementor's to draw against the real AST types.

**The fact-message contract**

The analyzers already embed the formatted directive in each reported fact's message
(`TS-F01: computed effects for Append — //tiger:effects alloc, mutate(r.log)`). `pin` needs the
directive back out of it. One helper formats those messages and one extracts the directive, both in
a single place that the three analyzers and `pin` share, and the shape is uniform across the three
verbs: rule ID, the fact kind, `for <function>`, ` — `, then the `directive.Format`-produced
directive the extractor isolates. Variant messages gain the enclosing function's name
(`TS-V01: synthesized variant for Append — //tiger:variant len(pending)`) — the position still
distinguishes each loop, the name attributes the fact to the function that owns it, matching how
`pin <Func>` collects variants by containment — and `variant.go` plus the `--show-facts` golden
fixture update accordingly as part of this feature. Extraction ends in
`directive.Parse`, so a message that does not yield a parseable directive is an operational error
rather than a silent skip. This closes the gap between the SSA wave's human-facing message and a
machine consumer without adding a second computation path through the driver.

### Out of Scope / Non-Goals

- **Rewriting or removing pins.** Not a deferral — see Core Design Principles. A future need to
  update pins mechanically must supersede this decision, not extend it.
- **Package-wide sweeps.** Deferred until the closed-dispatch trigger (TS-K03 under TS-P01) exists
  to require them.
- **`requires` and `ensures` pins.** Contracts are stated intent, not computed facts; there is no
  baseline to freeze. The `contracts` analyzer reports proven violations only and emits no fact
  diagnostics.
- **Pinning loops in functions the target calls.** `pin Flush` writes variants in `Flush`'s body
  only. A helper's loops are pinned by naming the helper.
- **Intent directives** (`hot`, `wire`, `owner`, `restrict`, `openenum`). Not computed, not
  pinnable, unchanged from the spec.
- **Unpinning, or a `--check` mode that verifies pins are current.** `tiger check` already does the
  second one; that is what a pin is for.
- **golangci-lint integration.** `pin` mutates source and is a CLI-only command; the plugin surface
  is unchanged.

## Dependencies and Constraints

- Everything the SSA wave pinned: Go 1.26+, `golang.org/x/tools`, testify. Nothing new.
- The SSA wave's fact output is the input contract. Its invariant 1 (fact output round-trips through
  the grammar package) is what makes extraction safe.
- The existing CLI fixtures are read-only because `check` does not mutate. `pin` tests must copy a
  fixture module into `t.TempDir()` before running, which is a new test mechanic for this repo.
- Implementation is AI-driven; the binding constraint is the acceptance contract above.

User stories are deliberately omitted, as in wave 1 and the SSA wave: developer infrastructure, one
consumer type, and the correctness constraints are already at story granularity.

---

## Functional

### Resolving a name to a target

`pin` runs the same analyzer set over the same loaded packages as `check`, then matches each
requested name against the functions and methods those packages declare.

A bare name matches a top-level function or a method by its own name. A qualified name
(`(*Store).Flush`, `Store.Flush`) matches only that receiver's method. Matching is exact, never
prefix or fuzzy: a typo is an error naming the closest thing to retry with, not a silent miss.

Ambiguity is an error rather than a choice. When one name resolves to more than one object, `pin`
prints each match in the qualified form the caller can retry with and writes nothing at all — not
even for unambiguous targets in the same run, because an ambiguous name means the caller's mental
model of the tree is wrong and the rest of the run is suspect.

### Collecting the facts for a target

The facts come from the driver's findings, filtered to the three reported categories:
`TS-F01-facts` (effects), `TS-F07-facts` (frames), `TS-V01-facts` (synthesized variants). Each
carries the position of the node it describes — the declaration for effects and frames, the loop for
variants — which is exactly what placement needs.

Effects and frame facts are matched to a target by declaration position. Variant facts are matched by
containment: a synthesized-variant fact whose position falls inside the target's body belongs to that
target.

A target with an existing agreeing pin has no fact diagnostic to collect — the analyzers report facts
for unpinned functions only. That is what makes invariant 4 (idempotence) fall out for free rather
than needing a comparison: nothing to collect means nothing to write.

A target with an existing *disagreeing* pin has no fact diagnostic either; it has a blocking pin
finding. That is the refusal path, and it is why the refusal message can print both lines — the
blocking finding already contains them.

### Writing

All edits for the whole run are computed before any file is opened for writing (constraint 7). Edits
are grouped by file and applied in descending position order, so an insertion never invalidates the
offset of one not yet applied. Each file is written atomically: a temporary file in the same
directory, then a rename.

`pin` runs no formatter over the result. Whole comment lines inserted at line boundaries preserve
gofmt-clean formatting, and running a printer would risk reformatting code the caller did not touch.

### Output

One line per directive written, naming the file, the line, and the directive:

```
store.go:42: //tiger:effects alloc, io(disk)
store.go:43: //tiger:frame r.log
store.go:58: //tiger:variant len(pending)
```

One block per refusal, naming the target and the reason. Under `--dry-run` the written lines print
in the same form with nothing on disk changed.

An unexported target prints one additional informational line before its written lines, naming the
target and that effects/frame pins do not apply (`flush: unexported, no effects or frame pin`).
This is not a refusal: it does not affect the exit code, and the target's variant pins still write
normally.

Exit codes:

- **0** — every named target was written or was already satisfied.
- **1** — at least one target was refused; everything else was written.
- **2** — the run could not proceed: no names given, a name matched nothing, a name matched more than
  one thing, or the driver failed to load or run.

This splits along the same seam as `check`: 1 means the tool did its job and found something you
must act on, 2 means the tool could not do its job.

## Architecture

New and changed components; internal shapes are the implementor's.

- **`internal/cli/pin.go`** — the subcommand. Owns flag parsing, name resolution, refusal policy,
  edit ordering, atomic writes, output, and exit codes. All run-level policy stays in the CLI, per
  ADR-0002.
- **`internal/pins`** — the directive-attachment package, relocated from
  `internal/analyzers/internal/pins` because that `internal/` nesting made it unimportable from the
  CLI. It gains the placement writer described in Scope, alongside the existing `Collect`. One
  package now owns both directions of the attachment rule.
- **A shared fact-message helper** — one format site and one extract site for the reported-fact
  message shape, used by `effects`, `frames`, `variant`, and `pin`.
- **`internal/driver`** — unchanged. `pin` calls the same `Check` entry point `check` does. There is
  deliberately no second driver path: a fact `pin` freezes is the same fact `check` reports, computed
  by the same run.
- **`internal/directive`** — unchanged. It remains the only formatting and parsing site.

### Invariant Preservation

- **Invariant 1** (Format-only text) is structural. The writer's input type is
  `directive.Directive`; there is no code path from a string to a written line.
- **Invariant 3** (never rewrite) is structural. The writer exposes insertion and no other
  operation, so no caller — including a future one — can express a replacement.
- **Invariant 5** (still parses, nothing reformatted) is structural. Line-boundary comment insertion
  cannot change the parse of surrounding code, and no printer runs.
- **Invariant 2** (binds to the intended node) is application logic. The placement rules have three
  cases and the writer must get each right, so it carries the round-trip property test.
- **Invariant 4** (idempotence) falls out of the analyzers' behavior: a satisfied pin produces no
  fact diagnostic, so a second run collects nothing. Asserted end to end anyway, because it depends
  on an analyzer property this feature does not own.
- **Invariant 6** (pinned target checks clean) is the composition of 1 and 2 with the analyzers'
  bidirectional comparison. It is the end-to-end assertion that catches a break in either.
- **Constraint 7** (no partial writes) is enforced by phase separation — compute all, then write all
  — plus atomic per-file replacement.
- **Constraint 10** (writes only named targets' files) is structural: edits are produced per resolved
  target, and there is no traversal from a target to its callees in the write path.

### Illegal State Analysis

The writer's input makes the dangerous states unrepresentable. It accepts a node and parsed
directives, so "replace the pin at this position", "write this arbitrary text", and "delete this
comment" have no expression. The refusal path is likewise not a policy check the implementor can
forget: a disagreeing pin produces a blocking finding and no fact, so there is no fact in hand to
write even if the check were omitted.

Two things rely on application logic and get the heavier test coverage: placement across the three
doc-comment cases, and matching a variant fact to the target whose body contains it.

### Domain-boundary opacity

`pin` depends on the driver's contract — findings with positions and categories — and never on which
analyzer produced a fact. All three verbs flow through one collection path and one placement path.
Adding a fourth pinnable fact kind in a later wave adds a category to the filter and nothing else.

## Security

`pin` writes to source files in the working tree, which no other tiger command does. Two limits keep
that bounded: it writes only files containing a target the caller named, and it only ever inserts.
Recovery from an unwanted run is `git checkout`, and `--dry-run` exists to see the result first.
Files outside the loaded packages are unreachable.

## PII

None. `pin` reads and writes source files in the working tree and sends nothing anywhere. The driver
already relativizes filenames so no absolute path reaches output.

## Scale

A `pin` run costs one `check` run plus a line-insertion pass over the touched files. Targets are
named individually, so the write set is bounded by the caller's argument list regardless of tree
size.

## Testing

Testing follows the `surface-testing` skill.

Key surfaces:
- **integration**: `cli.Run([]string{"pin", ...}, Streams)` over a fixture module copied into
  `t.TempDir()`, asserting resulting file bytes, stdout, and exit code. Copying is required because
  `pin` mutates, unlike every existing CLI fixture.
- **integration**: `pin` then `check` in sequence over the same temp tree, asserting the check exits
  clean (invariant 6) — the assertion that catches a placement or formatting break end to end.
- **integration**: `pin` twice, asserting the second run writes nothing (invariant 4).
- **unit**: the placement writer's round trip — write a pin, re-parse, `pins.Collect`, assert the
  binding — across all three doc-comment cases and a nested loop (invariant 2).
- **unit**: the fact-message helper's own round trip, format then extract.
- **fakes needed**: none. The only external dependency is the filesystem, and `t.TempDir()` is the
  real thing.

No async behavior, no time dependence, no in-memory store. The observability surface is stdout: what
was written and what was refused, both asserted directly.

## Limitations & Future Work

- **No mechanical pin update.** A deliberate refactor that legitimately changes a function's effect
  set requires a hand edit. That cost is the point, but if it becomes the dominant workflow the
  answer is a reviewable diff-producing mode, not an in-place `--update`.
- **No package sweep.** Arrives with the closed-dispatch trigger that requires one.
- **Variant pins on synthesized variants are optional and low-yield today.** They lock a loop against
  a refactor that breaks synthesis, which is real but narrow. Where synthesis fails, the variant is
  mandatory and `pin` has nothing to offer — the author must write the ranking expression.
- **Name resolution is per-run.** Pinning twenty functions means twenty names on one command line.
  A file of targets would be a small addition if that becomes tedious.

## Open Questions

None blocking. The one question worth revisiting after first use: whether `--dry-run` output should
be a unified diff rather than a line list, which depends on whether a human or an agent turns out to
be the primary caller.
