# 13. A loop tiger cannot prove ends blocks

Date: 2026-10-12

## Status

Accepted

## Context

Two rules check that loops end. TS-S02 asks that a loop's header state a bound: a constant, a
`len`, or a counter. TS-V01 asks for proof that the loop ends: a value that shrinks on every pass
and has a floor, found by tiger or written by a developer as `//tiger:variant <expr>` and checked
by the same rules. A loop with a counter in its header is outside TS-V01's scope, so a counter cap
is the way out when tiger cannot find a proof.

Measuring both rules on querator and git-server found two problems that pull in
opposite directions.

TS-V01 blocked 62 loops that end. On git-server, 21 of its 25 findings are loops TS-S02 already
accepts, and every one terminates: slice-consuming parsers, `size >>= 7` varint encoders, Myers
diff walks. Tiger's proof search knows only fixed steps such as `i--` and `s[1:]`, so it rejects
these, and it rejects the correct `//tiger:variant` a developer writes for them too. The two rules
also disagree: TS-V01 verifies `high - low` on a two-pointer loop, and TS-S02 still blocks it.

Both rules also accept loops that run forever. A bound in a loop's header is a claim, and
A due-diligence run on querator and git-server found 20 loops where the claim passes and the loop never ends or quietly cuts
its work short. TS-S02 trusts any counter whose name appears in the condition, including one inside
`||`, one the body resets, and a `uint8` counted to `<= 255`. TS-V01 matches variables by name and
looks only inside the loop body, so a shadowing copy or growth through a pointer defeats it. The
worklist TS-S01 tells developers to write in place of recursion grows its own limit and runs
forever on a cycle, and tiger reports nothing. A cap of any size passes, including `math.MaxInt`,
and a cap that is hit stops without telling anyone, so a truncated result is returned as complete.

Making TS-V01 informational would answer the false findings but fails: with
only TS-S02 blocking, the loops that append
into the slice they measure, or skip the shrink with `continue`, pass and run forever.

## Decision

A loop tiger cannot prove ends is a blocking finding. False findings on correct code are fixed by
teaching tiger more proofs, never by turning the block off.

Because a counter cap is the way out when no proof exists, the counter check must be as sound as
the proof itself, and a cap that stands in for a proof must report when it is hit. Concretely,
in one release, with the soundness fixes in place before TS-S02 trusts a proof:

1. TS-V01 matches variables by identity. Growth through a pointer or closure made before the loop,
   or a bound at the counter type's maximum, defeats a proof.
2. TS-V01 learns four more shrink forms: division or shift by a constant under `> 0`; slicing by
   the length of a part tiger can show is not empty; a struct field no call in the body receives;
   and an `&&` condition where one side shrinks.
3. A counter is a bound only when its comparison is a top-level `&&` term, pairs `<`/`<=` with
   `++`/`+=` or `>`/`>=` with `--` (never `!=`), nothing in the body writes the counter or its limit,
   and the limit is below the counter type's maximum.
4. TS-S02 accepts a loop whose variant tiger verified.
5. A counter cap is a page size, needing no report, only when its limit is not a constant and every
   other exit drains a stream whose type is declared outside the module (`rows.Next()`,
   `scanner.Scan()`). Any other exit makes it a safety cap, which must assert or return an error
   when the counter reaches the limit.
6. Every loop limit is traced back to where it enters the program. Each value written into it must
   be a constant, a length, another bounded limit, or a value clamped against a declared maximum.
   A call passing `math.MaxInt` or a constant at least half the type's maximum is flagged.
7. `for ctx.Err() == nil` on a `context.Context` parameter counts as a forever loop, like a
   `select` on `ctx.Done()`. A context from `context.Background()` or `context.TODO()` does not.
8. Ranging over an iterator counts as bounded only for the standard library's iterators. Any other
   iterator is a TS-S02 finding, like a channel.

Tiger's own 14 worklist loops and TS-S01's documented worklist move to the capped form that returns
an error on a cycle. Every one of the 20 loops that pass while never ending, the variant expressions tried against TS-V01, and the loops carrying `//tiger:variant` or `//tiger:batched` annotations become analyzer test cases.

## Consequences

- The sideband writer, the varint encoders and the two-pointer loops pass. The loops
  that run forever are blocked until they are rewritten.
- A safety cap is a visible failure, not a silent truncation. On the trial code this fires twice,
  both in git-server's Myers diff: one needs an assert, and one clears by moving the search into its
  own function.
- The limit trace already found three bugs in the trial code that no rule reported: a pack header
  that allocated 352 MB for a 12-byte push (ENG-193), list endpoints accepting any page size up to
  2³¹−1 (ENG-194), and a create-queue loop with no partition limit (ENG-195).
- TS-S02 trusts TS-V01's proofs, so a hole in TS-V01 now clears both rules at once. That is why the
  soundness fixes ship first and every hole stays a test case.
- Three misses are recorded, each with a test: a limit passed through a function value or interface
  method is not traced; a constant below the threshold, such as `1 << 40`, passes as a limit; and a
  standard-library stream over an endless in-program source counts as an outside stream. Ranging
  over a slice while appending to it ends, so it stays a correctness bug for tests, not a loop rule.
- `//tiger:batched` stops waiving the loop bound on cursor loops. That replaces part of ADR-0004 and
  is recorded in ADR-0014.
