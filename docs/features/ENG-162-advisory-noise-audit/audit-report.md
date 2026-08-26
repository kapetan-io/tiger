# ENG-162 — Advisory noise audit

Every advisory rule was measured on three codebases: tiger's own tree, querator at the ENG-148
pin `1fd1bb2`, and git-server in the mono-repo at the ENG-159 pin `321da03` (after
`task proto-gen`). "Before" is tiger built at `cf7a26e`, the ENG-150 SSA-wave merge this branch
starts from; "after" is tiger built from this branch. Counts are per diagnostic category, taken
from the `TS-XXX [advisory]` prefix the CLI prints, never by substring match on message text —
the grep-methodology bug ENG-159 documented.

## Measured advisory volume

| rule | analyzer | tiger | querator | git-server | total | after |
|---|---|---:|---:|---:|---:|---|
| TS-S13 declaration distance | `declusedistance` | 40 | 336 | 124 | **500** | removed |
| TS-N06 single-caller prefix | `declorder` | 284 | 28 | 80 | **392** | removed |
| TS-D07 skipped tests | `skipcheck` | 0 | 2 | 10 | 12 | kept |
| TS-L05 struct order | `declorder` | 0 | 1 | 4 | 5 | kept |
| TS-L09-escape escape uses | `directives` | 0 | 0 | 0 | 0 | kept |
| TS-L10-distance defer distance | `deferdistance` | 0 | 0 | 0 | 0 | kept |
| **total** | | **324** | **367** | **218** | **909** | **17** |

Two rules were 98% of the channel. The two standing advisories that exist on purpose — escape
uses and skipped tests, the metrics ENG-153's ratchet is built on — were 1.3% of it.

## TS-N06 — removed

The rule asks a single-caller helper to carry its caller's name. Its suggested rename compounds
down a call chain, because each level's accepted name becomes the next level's required prefix.
Applying it to `boundedloop.go` yields `checkEventLoopSelectHasTerminationCase`,
`zeroMovingAssignmentSelfShiftOrDivide`, `monotoneToZeroUnconditionalAssignment`. A file
decomposed into small single-purpose helpers — the shape the dialect asks for everywhere else —
draws findings in proportion to how well it is decomposed. That is not a threshold to tune; the
mechanical form of the rule contradicts the style it serves.

The volume confirmed it from the other direction: 284 of tiger's own 324 advisories were TS-N06
against tiger's own analyzers, so tiger's dogfood comment was a wall of self-inflicted rename
suggestions with the escape and skip advisories nowhere in sight.

The maxim survives in the spec as human review. The analyzer half of `declorder` that implemented
it is gone; `declorder` now enforces TS-L05 only.

## TS-S13 — removed

The rule fires when more than 10 lines separate a statement-level declaration from its first use.
Classifying all 500 findings by comparing the indentation of the declaration line against the
indentation of the first-use line:

| class | tiger | querator | git-server | total |
|---|---:|---:|---:|---:|
| first use in a deeper block — the declaration cannot move | 33 | 183 | 80 | **296** (59%) |
| the declaration statement itself spans the reported distance | 2 | 53 | 4 | **59** (12%) |
| genuinely separated by unrelated statements | 5 | 100 | 40 | 145 (29%) |
| first use in a shallower block | 0 | 0 | 0 | 0 |

The 59% majority is the accumulator-before-loop shape, and there the advice is not merely
uninteresting — it is wrong. `found := []*decl{}` declared above the loop that appends to it, or
`marked := map[*types.Named]bool{}` above the nested walk that fills it, cannot move down next to
the first `append`: that is a reset on every iteration, or a compile error when the block ends.
`start := time.Now()` in querator's benchmarks (`benchmark_main_test.go:320`) is the same failure
with a sharper edge — moving it to its first use at `time.Since(start)` measures nothing.

The 12% class is a measurement artifact: a multi-line composite literal used on the line after it
closes is reported as "declared 17 lines before its first use", the 17 lines being the literal's
own body.

That leaves at most 29% that could be acted on, and no way for the analyzer to tell the classes
apart — textual distance cannot distinguish a hoisted variable from a necessary one, because the
information that decides it is the block structure the metric ignores. TS-S13 the rule stands in
the spec, enforced on its dead-assignment half by `ineffassign` and `wastedassign`, and on the
rest by review.

## The rules that stayed

**TS-L05** (5 findings) is `declorder`'s remaining rule. Every finding is a constructor or method
sitting above the type it belongs to — `NewOID` below four `OID` methods in git-server's
`internal/storage/oid.go`, `NewBadgerQueues` above `BadgerQueues` in querator's
`internal/store/badger.go`. Low volume, correct advice, and the move it asks for is mechanical.

**TS-L10-distance** fired zero times on all three codebases. It buries nothing, so the audit has
no evidence against it; it stays as written.

**TS-L09-escape** and **TS-D07** are the advisories the channel exists for. Neither is a heuristic
that might be wrong: one reports a directive the author wrote on purpose, the other a `t.Skip`
that is there.

## Result

| | blocking before | blocking after | advisory before | advisory after |
|---|---:|---:|---:|---:|
| tiger | 0 | 0 | 324 | **0** |
| querator `1fd1bb2` | 387 | 387 | 367 | **3** |
| git-server `321da03` | 308 | 308 | 218 | **14** |

No blocking count moved — the removal touched no blocking rule. On the two real codebases the
advisory channel is now dominated by what it was built for: 12 of the 17 remaining advisories are
skipped tests, the rest struct order.

Verified on this branch: `go vet ./...` and `go test ./...` clean, `golangci-lint run ./...` 0
issues, the analysistest corpus green with `declorder` reduced to its `ts-l05` package, all three
determinism double-runs byte-identical (findings fixture, repository tree, `--show-facts`), and
the plugin smoke test through `bin/tiger-gcl` still surfacing TS-S09, TS-M10 and TS-S01 while
keeping computed facts out.

## Report tooling

The sticky pull-request comment printed `report.txt` whole, which is how 160 advisory lines
scrolled the blocking result off the screen on PR #6. `.github/report.awk` now builds the comment:
blocking findings stay verbatim, advisories collapse to a per-rule count with up to three example
positions, and the full list moves into a collapsed block so nothing is lost. A golden fixture in
`.github/testdata/` is diffed in the dogfood job, so an edit to the summarizer cannot quietly
wreck the comment.

## What this says about the advisory tier

ADR-0005 registers new heuristic rules as advisory and promotes them to blocking once trials show
every finding actionable. It named no other outcome, though both ENG-161 and this audit have now
taken one: when the trial shows the findings are not actionable, the rule is removed rather than
tuned. ADR-0006 records that missing exit — a rule whose output is dominated by noise does not get
fixed later, it teaches readers to skip the channel it prints on and takes the deliberate
advisories down with it. ADR-0005 is unchanged and still in force; its TS-M10 exact-tier decision
was never in question here.
