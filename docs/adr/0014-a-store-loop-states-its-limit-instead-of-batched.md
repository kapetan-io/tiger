# 14. A store loop states its limit instead of carrying //tiger:batched

Date: 2026-10-12

## Status

Accepted

Supersedes the loop-bound half of [ADR-0004](0004-batched-waives-the-loop-bound-on-cursor-shaped-loops.md).
`//tiger:batched` remains the waiver for TS-M10.

## Context

ADR-0004 let `//tiger:batched <reason>` waive TS-S02's bound on a cursor-shaped loop such as
`for rows.Next()`. Its premise was that the store is finite, and that any cap in the code would be
"either a magic number or a restatement of however big the store is".

ENG-191 audited the 30 cursor-shaped loops in querator that TS-S02 and TS-V01 both flag
(`docs/features/ENG-191-due-diligence-validate-tigers-approach/decision.md`, section 5 and
call 13). Seventeen already declare a real limit, the page or request size, either in the SQL
(`LIMIT $2`, `SetLimit(opts.Limit)`) or in a check deep in the body. Restating that limit in the
loop header is not a magic number. Five more are whole-partition scans with no limit, and all five
name their context `_`, so nothing can cancel them. The specification's B1 already rejects "the
store is finite" as a bound, since a store with no declared limit is bounded by whatever runs out
first.

Tiger never reads a `//tiger:batched` reason. `experiments/annotations` puts the same reason on a
cursor that returns 50 rows and on one that never runs out. Tiger reports the same thing for both,
and the second never returns. A `//tiger:variant` is the opposite case: tiger proves it, and every
false variant in the experiment is blocked. ADR-0004 also left TS-V01 firing on loops TS-S02
waived, the clash tracked as ENG-183.

`experiments/cursorbound` shows that a limit restated in the header holds whatever the store does:

```go
for count := 0; count < opts.Limit && rows.Next(); count++ {
```

With a database that ignores `LIMIT` and sends rows forever, the loop stops after `opts.Limit`
rows. Tiger accepts the loop with no annotation, because the counter is the proof.

A loop that filters rows before keeping them needs more than one limit. Six of querator's nine
Badger list loops skip rows by namespace. `experiments/cursorbound/filtered.go` runs three shapes
against a table whose matching rows come last. Counting every row read against the page limit
passes tiger and returns an empty page while matches exist. Counting only kept rows reads a million
rows to return one, and tiger blocks it. Two limits fill the page and stop a sparse scan with an
error.

## Decision

`//tiger:batched` no longer waives TS-S02. It stays the waiver for TS-M10, IO inside a loop.

A store loop states its limit in the header, restating the one its query already declares.

A loop that filters rows before keeping them has two limits:

- A page limit that counts kept rows, clamped to a declared `pageMax`.
- A scan limit that counts every row read, a declared `scanMax` related to the page limit by
  `const _ = uint(scanMax - pageMax)`. The loop's other exit is the page filling, so under ADR-0013
  the scan limit is a safety cap, and the loop returns an error when it reaches `scanMax`.

A whole-partition scan takes one of the two shapes B1 allows: a declared maximum partition size,
with the loop capped at it and an assert if it is exceeded, or paging that checks `ctx` on every
pass so the scan can be cancelled.

## Consequences

- A loop's bound is in code tiger checks, not in a reason it takes on trust. ENG-183 closes,
  because no waiver is left for TS-V01 to disagree with.
- Tiger keeps bounding how many times a loop runs, not only how much it returns. A filtered list
  can return `ErrScanLimit` on a sparse table, and callers have to handle it.
- The five whole-partition scans in querator stop hiding behind "the table size is the bound" and
  must either declare a maximum or become cancellable.
- Tiger cannot check that the restated limit matches the query's, since that would mean parsing SQL
  and knowing each driver's API. A loop limit smaller than the query's drops rows, which is a
  correctness bug for tests and review, not a loop that never ends.
- Tiger cannot see the short page in the shape that counts every row read: its only other exit
  drains the cursor, so the counter reads as a page size. The "filtered page" pattern
  (`cursorbound/ListFilteredTwoLimits`, ENG-191 call 15) covers that gap.
