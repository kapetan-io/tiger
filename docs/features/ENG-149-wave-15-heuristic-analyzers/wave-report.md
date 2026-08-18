# ENG-149 — Wave 1.5 report: trial-pin reruns

The blueprint's acceptance targets are recorded here against actual reruns of both trial
codebases: querator at `1fd1bb2` (the ENG-148 pin) and git-server in the mono-repo at `321da03`
(the ENG-159 pin, after `task duh-gen` + `task proto-gen`). "Baseline" is tiger built at
`f1b863a` (the ENG-161 merge this branch starts from); "wave 1.5" is tiger built from this
branch. Counts are per diagnostic category taken from the position-prefixed output field, never
by substring match on message text (the grep-methodology bug ENG-159 documented).

## Criterion 1 — TS-S02

**Querator: baseline 37 → 11 after tuning (target ≤5).** Monotone-lenient holds exactly: the
wave-1.5 finding set is a strict subset of the baseline's; no site fires that did not fire
before. The path from 37 to 11:

- 2 dropped by the tuple-post reversal widening alone (`internal/store/postgres.go:532`,
  `transport/auth/apikey.go:47` — both the `i, j = i+1, j-1` reversal shape).
- 24 cursor-shaped store drains (`iter.Valid()`, `rows.Next()`, `cur.Next(ctx)`,
  `scanner.Scan()`, `p.Next()`) waived by adding `//tiger:batched <reason>` — each annotation
  consumed, and each surfacing as a TS-L09-escape standing advisory (24 advisories appear).
- 11 remain: the pause/shutdown deadlock `internal/handlers.go:117` (**the real bug — still
  fires**), 4 loop-carried pagination-token loops (`for {}` over a `lastID` batch cursor in
  mongo/postgres — the blueprint's documented known-miss class: outside the cursor waiver,
  restate a bound or restructure), 1 bare-decrement-under-`!= 0` (`internal/types/batch.go:109`
  — excluded by design; a bare decrement cannot prove termination against wrap), and 5
  atomic/variable-bound waits (`for x.Load() != 0`, `for x.Load() < int64(n)`,
  `for totalLeased < itemCount`) that no wave-1.5 widening admits.

The ≤5 target assumed the pagination-token and atomic-wait shapes would fall inside the
grammar; they do not, and admitting them without a termination proof would break the exact-tier
invariant. The 6 non-waived, non-bug sites are each in a documented miss/exclusion class.

**Git-server: baseline 26 → 23 (target ≤7).** Monotone-lenient holds (strict subset). All 4
real bugs still fire: the annotated-tag peel-chain CPU-spin DoS
(`internal/graphapi/resolve.go:114`), both pkt-line command loops
(`internal/protocol/engine.go:99` and `:129`), and the ref-update accumulation
(`internal/protocol/receive.go:99`). Only 3 dropped (`graphapi/diff.go:120`,
`myers.go:100`, `myers.go:139` — compound conditions with a bounded operand). The remaining
Myers loops compare against *hoisted variables* (`for x < n && y < m` where `n := len(a)` lines
earlier) — no operand is a literal, constant, `len()`, or `cap()`, so the compound widening has
no bounded leaf to stand on. The varint loops (`for b&0x80 != 0` re-reading `b` from a reader)
are not the monotone-counter shape either: the condition operand is a masked expression, not an
identifier, and the loop's "advance" is a fresh read, not a shift or division. The ≤7 target
misclassified these; the grammar as specified (and proved) cannot admit them, and they stay
firing. Candidate follow-up widenings, each needing its own termination proof: a bounded-leaf
rule for identifiers whose sole assignment is `len(...)`, and a masked-byte varint shape.

## Criterion 2 — TS-S03 / TS-C05 (querator)

- TS-S03: 3 → 0. `requestLoop`'s select passes via the name recognition on `shutdownCh`
  (`internal/logical.go:658`); `stopCleanup`'s selects pass via the `struct{}` closed-channel
  broadcast recognition (`internal/auth_cache.go:211`, `:240`). **No querator code changes.**
- TS-C05: 19 → 13, and the real missing-cancellation client waits the ENG-148 report names —
  `internal/logical.go:197/264/310/353`, the `<-req.ReadyCh` waits — **all still fire**.

This criterion forced a build-time correction to the blueprint's §3: `req.ReadyCh` is
`chan struct{}`, so exempting bare operations under the *type* recognition silenced the exact
findings this criterion pins. Resolution: select cases accept either recognition; bare
receives/sends are exempt under the *name* recognition only. The blueprint and spec now say so.

## Criterion 3 — TS-S08 open enum (git-server)

Adding `//tiger:openenum` to `PackReason`'s doc comment silences exactly its `String()` switch
(`internal/storage/errors.go:64`; TS-S08 total 14 → 13). Both real S08 bugs still fire:
`writeHunk`'s defaultless switch (`internal/graphapi/diff.go:333`) and `packCode`'s invalid
zero nibble (`internal/pack/writer.go:66`).

## Criterion 4 — TS-M10 (querator)

With the seed allowlist alone, querator shows 2 TS-M10 findings (stdlib IO in loops). The
per-id validation loops call third-party drivers, so the rerun exercised the per-repo
extension:

```
tiger check -ioinloop.packages=github.com/dgraph-io/badger/v4,github.com/jackc/pgx/v5,\
github.com/jackc/pgx/v5/pgxpool,go.mongodb.org/mongo-driver/mongo ./...
```

→ 164 TS-M10 findings on the unannotated pin, concentrated in the four store backends
(badger 95, postgres 40, mongo 27) — including every `continue nextBatch` per-id validation
loop (30 `continue nextBatch` sites, all inside flagged loops). With the 24 cursor-drain
`//tiger:batched` annotations from the criterion-1 rerun in place, the count drops to 107 (a
cursor drain's per-item IO is waived by the same directive that waives its bound). Annotating
one per-id loop with `//tiger:batched store offers only per-id Get; no bulk lookup API`
silenced that loop's findings, left the sibling loops firing (per-decision accounting), and
surfaced the directive as a TS-L09-escape standing advisory (advisory count 24 → 25).

## Criterion 5 — repo health

- `go build ./...`, `go vet ./...`, `go test ./...`: green.
- `golangci-lint run ./...`: 0 issues.
- `tiger check ./...` on tiger's own tree: exit 0, **0 blocking**, 166 advisory; two
  consecutive runs byte-identical. (164 at feature completion; the second hardening round below
  added two helpers that carry the same single-caller TS-N06 advisory as this file's others.) Dogfooding the new rules forced real fixes to the wave's own
  code: the new analyzers' `token.Token` switches became if-chains (TS-S08), five-parameter
  walkers became method sets on a walker struct (TS-N07/N08), and two genuine TS-M10 hits in
  existing code (`internal/golangci` config probing, the registry meta-test's per-analyzer
  stat) were rewritten to single batched `ReadDir` calls.
- Plugin parity: `golangci-lint custom` build; the smoke fixture surfaces both TS-S09 and the
  new TS-M10 through `tiger-gcl run` (exit 1), matching the CI greps.

## Post-build hardening

An adversarial review of the widenings found five acceptances the stated termination proofs do
not actually cover, each closed with a corpus failure pin (red before the fix):

1. Negation now De Morgan-flips the compound rules: `!(A && B)` is `!A || !B`, so under a `!`
   the &&-either rule becomes the ||-every rule, and the monotone/tuple widenings never apply
   in a negated context (`!(x != 0)` runs *while* x is zero).
2. The monotone-counter shift admission requires an unsigned counter (a signed arithmetic
   shift converges to −1 from a negative start, never zero) and a constant shift amount of at
   least one (`x >>= 0` and `x >>= k` prove nothing).
3. The monotone widening is integer-only (float division never moves NaN or Inf).
4. The tuple-post reversal is integer-only (at large float magnitudes `a+1` is absorbed).
5. The direction scans now see `for x = range s` as an assignment to x, and the cursor waiver
   rejects package-qualified calls (`for strings.Contains(...)` is not a method on a cursor).

Both trial pins were rerun after the tightening: identical TS-S02 site sets on querator and
git-server — the holes were latent, which is why they survived the corpus.

A second adversarial review (an independent review agent, after the fixes above) found two more
acceptances in the `!= 0` monotone-counter widening, both demonstrated with probe corpora that
made TS-S02 stay silent on a genuinely infinite loop:

6. Assignments were matched by identifier *name*, not `types.Object` identity — a shadowed
   inner redeclaration's right shift "proved" an outer counter the loop never touched. Matching
   now goes through `TypesInfo.Uses`, so a shadow counts neither for nor against the proof
   (which also stops a shadow's assignment from spuriously defeating the tuple-post rule).
7. No assignment was required to actually execute: a qualifying shift nested under an `if`, or
   sitting after a `continue`, may run on no iteration at all. The proof now requires at least
   one zero-moving assignment that is the Post clause (Go runs Post even on `continue`) or a
   direct body statement with no `continue`/`goto` before it. The conservative edge — moving
   the counter in both arms of an `if`/`else` terminates but is rejected — is corpus-pinned as
   a documented failure case.

Both closed red-first in the ts-s02 corpus (four new failure pins, three new compliant pins),
plus nil guards on two unprotected `TypesInfo.TypeOf(...).Underlying()` calls the same review
flagged. Both trial pins were rerun on the hardened build: querator shows 35 TS-S02 on the
clean pin (exactly the 37 baseline minus the 2 tuple drops; handlers.go:117 and batch.go:109
still fire), git-server shows the same 23 sites with all 4 real bugs firing and the 3 compound
drops still dropped — the monotone widening had admitted no real-world site on either pin, so
the tightening cost nothing.

One TS-M10 false positive surfaced on the git-server pin itself: `req.Header.Set(k, v)` in a
loop was flagged (twice, `conformance/suite.go:189/:211`) because `http.Header.Set` lives in
net/http — but it is a map write, not IO. Per wave-1 constraint 7 the classifier was refined,
not suppressed: within an allowlist package, a method whose receiver's underlying type is a
map, slice, or basic value is exempt (a pure container holds no connection or descriptor); IO
stays flagged behind struct-backed receivers and package functions. Regression-pinned in the
ts-m10 corpus; git-server TS-M10 drops 8 → 6, and all six remaining are genuine
`os.Stat`/`os.Chmod`/`os.WriteFile` calls in loops.
