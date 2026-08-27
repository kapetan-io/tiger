# Diagnostic Message Rewrite Blueprint

Linear: ENG-163. Branch: `thrawn01/eng-163-diagnostic-message-rewrite-plain-language-for-readers-new-to`.

## Objective

Tiger's diagnostics are its voice. On a first adoption run they are hundreds of lines of first
impressions, and today they are written for a reader who already knows tiger's internal
vocabulary:

```
core/core.go:10:1: TS-F02: computed effects io(net) are not declared by this pin — introduced by a call to fixture.example/facts/helper.Ping at core.go:12:20 — remove the call or update the pin to //tiger:effects io(net)
```

The message is correct. A newcomer reads three terms of art (computed effect, pin, declared by)
before reaching the one thing they need: this function does network IO that its
`//tiger:effects` comment does not mention. This feature writes a message style guide, holds every
message tiger prints to it, and rewrites the ones that fail.

The wording target is a **newcomer human**: someone who has never seen a `//tiger:` directive
must be able to state, after one read, what the code did and the exact edit to make. This
supersedes the wave-1 blueprint's "AI agents are the primary consumer" line as the *wording*
standard. An agent that can act on a newcomer's message can act on any message; the reverse is
false, and the newcomer is the one who decides whether tiger gets adopted. The wave-1
"self-correcting" requirement (every finding names the compliant form) stays.

## Mental Model

A diagnostic is one line with two halves:

```
<path>:<line>:<col>: TS-XXX: <body>
```

- The **prefix** `path:line:col: TS-XXX:` is a parsing contract. The position is rendered by the
  CLI from `token.Position`; the `TS-XXX:` code is the first thing each analyzer writes into its
  message. ENG-153's structured output, `.github/report.awk`, and the ENG-162 audit method all
  key on it. This feature does not change it.
- The **body** is prose for a human. It has three parts in a fixed order, separated by ` — `:
  1. **What the code does.** Concrete and local: "this function makes a network call
     (`helper.Ping` at core.go:12:20)".
  2. **What tiger expected.** The gap, in plain words: "but its `//tiger:effects` comment
     doesn't list `io(net)`".
  3. **The edit.** A change the reader can make without opening the spec: "add `io(net)` to the
     comment, or remove the call".

Parts 1 and 2 may merge when one clause carries both ("goto jumps to a label" already says what
tiger expected). Part 3 is never omitted from a blocking finding.

The `--show-facts` channel prints facts, not findings, and has two parts: what tiger computed,
and the directive that would freeze it: "this function's effects are `io(net)` — to freeze them,
add `//tiger:effects io(net)`".

## Core Design Principles

1. **The prefix is frozen; only the body changes.** `path:line:col: TS-XXX:` is byte-identical
   before and after this feature on every channel that carries it.
2. **Plain words.** No internal vocabulary in a body unless glossed in passing. The banned list
   starts with: pin, pinned, computed, fact, facts, effect set, frame condition, lattice,
   closed set, back edge, dominating, synthesized, ranking, allowlist. Words that are directive
   verbs (`effects`, `frame`, `variant`, `requires`, `ensures`, `batched`) appear only as the
   literal directive spelling.
3. **Directives are spelled literally.** `//tiger:effects io(net)`, never "the pin" or "the
   effects declaration".
4. **The edit is an edit.** A comment to add, a call to remove, a name to change, a snippet to
   paste. Never a restatement of the rule.
5. **No rationale by default.** The why lives in the specification and the ENG-154 docs, which
   will quote the message and add the reasoning. One short plain clause may stay only when the
   edit would look arbitrary without it (TS-N08: "`Save(true)` means nothing at the call site").
6. **One rule code per line.** A body never mentions another `TS-` code. Where a message pointed
   at a sibling rule ("or use the ctx.Done() event-loop shape (TS-S03)") it now names the shape in
   words.
7. **One line, bounded.** No newline. Soft target 160 characters of body, hard cap 240. A code
   snippet stays only when the edit *is* the snippet (`for _, k := range slices.Sorted(maps.Keys(m))`).
8. **One voice across every channel.** Blocking findings, the `--show-facts` channel, the two
   standing advisories (escape use, skipped test), the `tiger golangci` config-audit lines, and
   the directive parse-error text spliced into TS-L09 all follow the same guide.

## Correctness Constraints

### State Invariants

Each holds for every line tiger prints: every message an analyzer emits on its corpus, and every
line the `tiger golangci` audit emits on its fixture config. The enforcement mechanism is the
corpus meta-test for analyzers and a fixture-driven test in `internal/golangci` for the audit
(see Testing) unless stated.

- **I1** The message starts with `<RuleID>: ` for a RuleID that resolves in the registry.
  (Already enforced by `TestEveryCorpusDiagnosticResolvesInRegistry`.)
- **I2** The body contains no substring matching `TS-[A-Z]+[0-9]+`.
- **I3** The message contains no `\n`.
- **I4** The body is at most 240 characters on corpus output. Runtime messages interpolate real
  identifiers and may exceed it; the cap binds the template as exercised by the corpus, which is
  the executable specification of each rule.
- **I5** The body contains no word from the banned list (case-insensitive, word-boundary match).
- **I6** If the body mentions a directive, the substring `//tiger:` is present.
- **I7** The rewrite changes no diagnostic's position, count, or category. `analysistest`
  enforces this: every corpus `// want` line must still match exactly once at the same position.

### Behavioral Constraints

- **B1** Never change the prefix. Not the separator, not the code, not the position rendering.
- **B2** Never let the corpus and the analyzer drift: a wording change lands with its `// want`
  update in the same commit, or the corpus fails.
- **B3** Never delete a `// want` line to make a rewrite pass. A `want` regex may be re-worded; the
  count per corpus file is unchanged (I7).
- **B4** Determinism holds: two runs over the same tree produce byte-identical stdout, with and
  without `--show-facts`. Messages built from sorted joins (norecursion's cycle names, effects'
  qualifiers) keep their sorting.

Concurrency, reversibility, partial failure: the feature changes string constants and one test.
There is no shared state and no runtime operation; rollback is `git revert`. The only
partial-failure shape is a message rewritten without its corpus, and B2 covers it.

## Acceptance Criteria

1. `docs/adr/0009-*.md` records the style decision (Accepted). Number re-checked against
   `origin/main` at rebase time: ENG-151 lands 0007 and 0008 first.
2. `docs/Tiger Specification.md` has a "Diagnostics" section: the three-part structure, the
   banned list, the prefix contract, and one worked before/after pair.
3. `internal/rules/meta_test.go` fails on any corpus message violating I2–I6, and the
   `internal/golangci` fixture test fails on any audit line violating them, each naming the
   offending source, message, and violated rule in the failure output. Verified by one temporary
   deliberate violation per invariant I2–I6 during the build (a second code in a body, a newline,
   a 241-character body, a banned word, a directive named without `//tiger:`), each recorded in
   the report with the failure output, then reverted.
4. Every message template listed in the report (all 27 analyzers, plus `internal/golangci` and
   `internal/directive` detail strings) has a before/after pair, or is marked "unchanged:
   conforms" with the blind reader's answers.
5. Each "after" passed the blind-reader check: a reader given only the message wrote (a) what the
   code did wrong and (b) the edit, and both match the author's stated intent. Both answers are
   in the report.
6. `go test ./...` green. CI `dogfood`, `determinism`, and `plugin-smoke` jobs green.
7. `grep -rn "TS-[A-Z][0-9][0-9]" internal/analyzers --include='*.go' | grep -v _test | grep -v testdata`
   shows each code only at the start of a message string (I2, human spot-check backing the test).

## Scope

### In Scope

- Style guide: ADR-0009 and the specification section.
- Meta-test extension enforcing I2–I6.
- Rewrite of every non-conforming message in `internal/analyzers/*` (all waves), the facts
  channel, the two standing advisories, `internal/golangci` audit messages, and
  `internal/directive` parse-error `Detail` strings.
- Lockstep corpus updates (177 `// want` lines across 49 files as of `aeb3d90`), the
  `internal/cli` and `internal/golangci` exact-string tests, the plugin-smoke grep, and the
  `.github/testdata/report.txt` / `report.golden.md` fixtures (static input to the awk
  summarizer; updated so they don't quote stale wording).
- The rewrite report with every before/after pair and blind-reader answers.

### Out of Scope / Non-Goals

- Changing the prefix or the `[advisory]` marker mechanics.
- The CLI/plugin asymmetry where `[advisory]` is added only by `tiger check` (plugin output has
  no marker). Pre-existing; not a wording matter.
- Wiring `CustomRule.Title` (unused today) into messages or checking it against them.
- A message-builder helper. Shapes vary too much (four TS-L09 shapes, facts reports, audit lines)
  for a structural builder, and a prefix-only helper buys nothing the meta-test doesn't already
  guarantee.
- Adding rationale or doc links to messages. ENG-154 owns the reference material.
- Changing which rules fire, where, or at what severity.

## Dependencies and Constraints

- ENG-150 (SSA wave) and ENG-162 (advisory noise audit) are merged; both blockers cleared.
- ENG-151 (`tiger pin`) is in flight and will take ADR numbers 0007 and 0008.
- ENG-154 (docs) lands after this and quotes real messages, so wording here is what the docs
  will teach.
- ENG-153 (structured output) depends on the prefix contract (B1).
- ADR-0002: analyzers stay pure passes; the driver owns severity and formatting. The rewrite
  touches message strings only; no analyzer gains output logic.

---

## Functional

**Style guide.** ADR-0009 states the decision and its reason (newcomer target, supersedes the
wave-1 wording stance). The spec section is the operative checklist an author runs before
committing a message:

1. Does the body open with what the code does, in words a Go programmer with no tiger context
   understands?
2. Does it name the gap without a banned word?
3. Does it end with an edit the reader can make now?
4. Is every directive spelled `//tiger:<verb> <args>`?
5. Is there exactly one `TS-` code, in the prefix?
6. One line, under 160 characters of body if you can, under 240 always?
7. If a why clause is present, would the edit look arbitrary without it?

**Worked example (the ticket's).**

Before:
```
TS-F02: computed effects io(net) are not declared by this pin — introduced by a call to fixture.example/facts/helper.Ping at core.go:12:20 — remove the call or update the pin to //tiger:effects io(net)
```
After:
```
TS-F02: this function makes a network call (helper.Ping at core.go:12:20) but its //tiger:effects comment doesn't list io(net) — add io(net) to the comment, or remove the call
```

**Rewrite report.** `docs/features/ENG-163-diagnostic-message-rewrite/rewrite-report.md`. One
table row per message template: rule code, analyzer, before, after (or "unchanged"), blind
reader's (a) fault and (b) edit, pass/fail. Templates with interpolated arguments show one
representative instantiation from the corpus.

**Blind-reader protocol.** For each "after", a sub-agent receives the message text only (no spec,
no code, no glossary, no "before") and must answer: "What did the code do wrong? What exact edit
fixes it?" The author compares both answers with the intent. A miss means the message is
rewritten and re-read. The report records the final round.

## Architecture

No component changes. Three code surfaces are touched:

- `internal/analyzers/*/*.go` message constants and format strings, plus their
  `testdata/**/*.go` `// want` regexes.
- `internal/golangci/golangci.go` three `Sprintf` templates, plus `internal/cli/golangci_test.go`
  exact strings. Each template repeats the rule code mid-sentence today ("auto rule TS-M10 (...)
  is not enforced"); the rewrite drops the repeat so I2 holds, and a new test in
  `internal/golangci` runs the audit over a fixture config and applies the I2–I6 checks to every
  emitted line. Whether the checks are shared with the meta-test as an exported helper or
  duplicated is the implementor's call.
- `internal/directive/*.go` `MalformedArgsError.Detail` strings, reachable only through TS-L09.

One test surface grows: `internal/rules/meta_test.go`'s corpus walk gains the I2–I6 assertions
and an unexported banned-word list. Contract of the extended test: for every analyzer in the
registry, run its corpus, and for every diagnostic emitted, fail with the analyzer name, the full
message, and the violated invariant name if I1–I6 does not hold. The banned list is data in the
test file; the spec section is its human-readable twin, and the report's acceptance check reads
both.

CI adjustments: `plugin-smoke`'s `! grep -q "computed effects"` (proving reported-severity facts
never leak through golangci-lint) becomes `! grep -q "TS-F0"`, anchored on rule codes rather than
wording that this feature changes. The `dogfood` golden fixtures are static awk input and are
re-worded to match, not regenerated.

## Data Design

No persistent data. The banned-word list is the one new datum; it lives in
`internal/rules/meta_test.go` as an unexported slice, matched case-insensitively at word
boundaries against the body (everything after `<RuleID>: `).

### Invariant Preservation

- I1–I6 are properties of strings; the meta-test evaluates them on every corpus diagnostic, and
  the corpus is every rule's executable specification, so no shipped rule escapes the check.
- I7 is enforced by `analysistest` itself: a changed count or position at any `// want` line
  fails the corpus.
- B1: the prefix is assembled by `token.Position.String()` in `internal/cli/check.go` and by the
  `RuleID + ": "` literal at the front of each message; neither file's prefix logic is in scope,
  and I1 plus `TestCheckOutputIsDeterministic` catch a drift.
- B4: the determinism CI job diffs two runs; message content with unsorted joins would already
  fail it today, and the rewrite keeps every existing sort.

### Illegal State Analysis

All invariants are enforced by test logic, not structurally: a message is a Go string and the
type system cannot forbid a newline or a second rule code. That is acceptable here because the
test runs on every rule's corpus on every `go test`, and an analyzer cannot ship without a corpus
(ADR-0002's meta-test consequence).

## Security

None. No input crosses a trust boundary; messages are derived from the analyzed source's
identifiers, which already reach stdout today.

## PII

None. Messages contain source identifiers and file paths of the analyzed tree, as they do today.

## Scale

Message length is bounded by I4 on templates. A first adoption run prints one line per finding,
unchanged in count (I7).

## Testing

Testing follows the `surface-testing` skill.

Key surfaces:
- integration: `internal/rules/meta_test.go` corpus walk (I1–I6), run via `go test ./internal/rules/`.
- integration: every `internal/analyzers/*/testdata` corpus via `analysistest` (I7, B2, B3).
- integration: `internal/cli/cli_test.go` `TestCheckOutputIsDeterministic` and
  `TestCheckShowFactsIsDeterministic` (B4), and any exact-string CLI tests updated in lockstep.
- integration: `internal/cli/golangci_test.go` exact strings for the audit channel, and a new
  `internal/golangci` fixture-config test applying I2–I6 to every audit line.
- integration: `plugin/plugin_test.go` `TestPluginDropsReportedFacts` (severity isolation is
  unchanged by wording).
- CI: `dogfood`, `determinism`, `plugin-smoke` jobs.
- fakes needed: none. No external dependency, no clock, no async behavior.

The meta-test's own correctness is proven once during the build with one deliberate violation per
invariant I2–I6, confirming each failure output names the invariant; the report records all five
runs. I5 and I6 get this treatment because their word-boundary and directive-spelling logic is the
part most likely to be wrong quietly, and I4 because a byte-versus-rune or off-by-one slip would
pass every real message.

## Limitations & Future Work

- Runtime messages can exceed 240 characters when identifiers are long (norecursion joins every
  name in a cycle). The cap binds corpus output only.
- The `[advisory]` marker is applied by `tiger check` alone; the plugin path prints advisories
  without it. Pre-existing; a candidate for ENG-153.
- `CustomRule.Title` is unused registry metadata. A later ticket could derive it from, or check
  it against, the message's first clause.
- Words that are directive verbs (`frame`, `variant`) cannot be on the banned list without
  false positives on literal directives. The blind-reader check is the guard for their misuse as
  vocabulary.

## Open Questions

- Whether ENG-154 wants the spec's "Diagnostics" section to be the canonical style page or to
  move it into the docs tree. Not blocking: the section is short and ENG-154 can relocate it.
