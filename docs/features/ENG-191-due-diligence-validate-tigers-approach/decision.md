# ENG-191 — Decision: what tiger's static analysis can hold, and the rule set that follows

Date: 2026-09-17

## Decision

Rule-based static analysis, as tiger practices it, gets the project its stated goal for one class
of rule and fails it for two others. The class it holds is the exact structural rule over a
restricted language, plus the opt-in pin checked in both directions. Every real bug either trial
found came from that class, and that class has no false positive by construction. The two classes
it fails are judgment rules (naming, distance, style), which the project already removed and
should not revisit, and termination proofs beyond the bound shape, which TS-V01 attempts with a
grammar too small to verify real loops and which this decision folds into the pin model. What
static analysis cannot decide at all is whether a declaration is true, and the design already
places that with review. Tiger's job there is to keep the review surface small and impossible to
silence, not to judge the claim, and not to embed a language model to judge it.

The rule set that follows keeps 35 of the 40 registered custom rule codes as they are, changes the
mechanism of three, converts one to a pinned fact, demotes one to an advisory trial, and adds no
new rule. The largest single change is a fix rather than a rule. Four blocking rules recognize
compliance by an identifier's name where the behavior is computable, and an agent satisfies each
by renaming. The three explainer
disagreements resolve as one wrong claim (`//nolint`), one product call this document makes
(`//tiger:restrict` stays, opt-in), and one place where the explainer and the code agree and the
specification is stale (map order).

## Evidence base

Three sources, none of them the specification's own claims about itself.

**The two trials and their reruns.** ENG-148 (querator, ~36k lines) and ENG-159 (git-server,
~17k lines) ran wave-1 tiger; ENG-149 and ENG-162 reran both pins after the wave-1.5 tuning and
the advisory audit. Together they found ten real defects, all from blocking rules of the
structural class.

| Rule | Real bugs | What it caught |
|---|---:|---|
| TS-S02 loop bound | 5 | a pause/shutdown deadlock; a remotely triggerable CPU-spin DoS through an uncapped tag peel chain; two uncapped pkt-line command loops; an uncapped ref-update accumulation |
| TS-S08 closed switch | 3 | two storage backends dropping an action with no default and no log; a diff writer that silently emits nothing for a future op tag; a pack writer returning an invalid type code |
| TS-C05 blocking op | 1 | three client waits with no cancellation that the doc comment claimed |
| TS-C02 goroutine owner | 1 | an unwaited `WaitGroup` letting `Shutdown` return early |
| TS-E02 discard | 1 | a hash-format validation error discarded, a latent corrupted object id |

**A fresh run of today's tiger against the querator pin.** Built from `main` at `5126797`, run
against querator at `1fd1bb2`, the same pin ENG-148 used. This is what a new adopter sees now,
after every removal and tuning to date.

| Rule | Findings | Verdict on the code |
|---|---:|---|
| TS-E02 discard needs comment | 86 | true by spec; about half are deferred `Close`/`Rollback`/`Shutdown` cleanup where the only remedy is a boilerplate comment |
| TS-N07 same-type params, max 4 | 45 | ~30 genuine swap hazards; ~15 are the `t, ctx, client` test-helper prefix burning the cap by convention |
| TS-T06 test has doc comment | 40 | all genuinely undocumented tests (the `Goal:` token requirement is gone) |
| TS-V01 loop variant | 37 | 30 are the cursor loops TS-S02 waives; 7 are provably terminating loops the synthesizer cannot see; none is a bug |
| TS-S09 no labeled continue | 35 | 30 copies of one `continue nextBatch` idiom across four mirrored backends; zero gotos |
| TS-S02 loop bound | 35 | 30 cursor drains, waivable with `//tiger:batched`; the pause/shutdown deadlock still fires |
| TS-S18 naked panic | 15 | true; blocked on adopting an assert package |
| TS-S08 closed switch | 15 | true; same prerequisite; three sites would change behavior if fixed mechanically |
| TS-C02 goroutine owner | 14 | 4 real; 10 are complete `wg.Add`/`wg.Wait` supervision the rule has no vocabulary for |
| TS-C05 blocking op | 13 | the real missing-cancellation waits still fire |
| TS-C09 reactive spawn | 12 | 12 of 12 in `_test.go`, the bounded fan-out-then-`Wait` test idiom; zero production hits |
| TS-T02 map order | 11 | sampled 4: one real order leak into logs, three order-insensitive bodies the allowlist cannot prove (append followed by a sort, nested deletes on a second map, a map write under `if`/`else`) |
| TS-E06 return arity | 11 | 10 idiomatic multi-value helpers, 1 pinned by a third-party interface |
| TS-X01 single-impl interface | 9 | 9 of 9 are role interfaces over one `service.Service`, the segregation idiom; the named remedy widens every consumer's dependency to the whole service |
| TS-N08 bool param | 5 | 1 strong, 4 weak |
| TS-T10, TS-C12, TS-S06, TS-S01, TS-M10 | 4, 3, 2, 2, 2 | true by spec; TS-S01 names two real call cycles through the request loop |
| **Total blocking** | **401** | ENG-161 predicted ~350 for this pin after the naming removals; the difference is TS-V01 |

Advisory output after budgets is 2 skipped tests and 1 misordered struct. The channel is what
ADR-0006 said it should be.

**Two fixture probes** run against the same binary, checking a claim the run raised. A cursor
loop carrying `//tiger:batched`, a two-pointer reversal with `i, j = i+1, j-1`, and a
`for n > 0 { n /= 62 }` loop all fire TS-V01. Pinning `//tiger:variant j - i` on the reversal
and `//tiger:variant n` on the division is rejected as unverifiable. The only exit the rule
offers is an artificial iteration cap, which ADR-0004 rejected for cursor loops as "either a
magic number or a restatement of however big the store is."

## Where the line falls

The specification's own framing survives contact with the evidence. "Machines check the
correspondence; humans and AI review the declarations" is the right split, and every place tiger
has gone wrong is a place it crossed that line in one direction or the other.

**Sufficient: exact structural rules over the restricted language.** TS-S02, S03, S08, S09, C02,
C05, C12, E02, E06, M10, N07, N08, N14, T06, T10, L09, L10, M05, S01, S21, S22, T02, and the
cross-package invariant rules classify every instance of a construct as an allowed shape or a
finding. Whether a loop has a stated bound has an answer for every loop; whether it halts does
not. This class found all ten bugs, produces byte-identical output across runs, and cannot fire on
compliant code except where its shape grammar is narrower than the language, which is the known
miss discipline the corpus already enforces. The wave-1 correctness constraint that a false
positive on a blocking rule is an analyzer bug, never a suppression, held on both codebases (seven
analyzer defects fixed on querator, one on git-server, zero suppressions).

**Sufficient: opt-in pins checked bidirectionally.** Effects, frames, and restriction axes are
computed for every function and package and fire nothing until someone freezes one. Absence of a
declaration is never a finding. This is the model that lets a whole-program analysis ship with
zero noise on a codebase that has not adopted it, and it is the model TS-V01 should have used.

**Not sufficient, already resolved: judgment rules.** Naming dictionaries (N12, N13, N15),
single-caller prefixes (N06), and declaration distance (S13) were removed after trials showed
their output dominated by domain vocabulary and by shapes the metric cannot distinguish from
misplacement. ENG-161 and ENG-162 got this right and ADR-0006 recorded it. The specification keeps
the maxims with review as enforcement. Nothing in this investigation argues for reviving them, and
the config file's package scoping does not change the verdict, because the failure was never
scoping. A rule whose best-case yield across two codebases is `itemPtr` and `srcSize` does not
earn a blocking slot.

**Not sufficient, unresolved until now: termination proofs beyond the shape.** TS-V01 is a
ranking-function prover with a closed grammar (`len(s) > c`, `i < n`, `low < high` against
`i++`, `s = s[1:]`, `high--`). On querator it fires 37 times, catches nothing TS-S02 does not,
misses the one real termination bug (a `for { select }` loop, by design outside its scope), and
rejects a correct pin on a loop TS-S02's wave-1.5 grammar already accepts as bounded. Two rules
apply two different termination grammars to the same loop, and the stricter one has no escape.
The explainer's claim that synthesis "covers nearly every real loop" is false on the only real
codebase it was measured against. This is not a tuning problem. A prover small enough to be exact
is too small to verify the loops people write, and growing it is the "exponential in the tail"
cost the specification's honest-limits section names.

The fix is to apply tiger's own pin model. Variant synthesis stays as a computed fact, printed
under `--show-facts` and frozen by `tiger pin`. A pinned variant is verified bidirectionally,
exactly as now. An unpinned loop that TS-S02 accepts is not a finding. The loop-bound rule remains
the blocking rule for termination, and it already carries the reviewed cursor waiver ADR-0004
admitted. This removes 37 findings from querator, restores the waiver, and keeps every guarantee a
pin gives.

**Not static at all, by design: the truth of a declaration.** Whether a `//tiger:batched` reason
is true, whether a type is genuinely an open enum, whether a package should claim closed dispatch,
whether the comment above `_ = f.Close()` is honest, whether `SequenceMonotonic` is the invariant
the protocol needs. The specification's "what no amount of tooling fixes" table is correct that
these are the same thing, and the ticket's question about LLM-judged review lands here. The answer
is that this review already exists and its reviewer may be a human or an AI agent reading the pull
request. What tiger owes that reviewer is a surface that is small, visible on every run, and
impossible to shrink without a diff. The standing advisories, the budget file, and the directive
lines are that surface. Tiger should not judge the claims itself. The README's "never a language
model" holds, because a model inside the gate would make the verdict nondeterministic and would
turn the one thing agents cannot fake, a green run, into something they can argue with.

**Runtime checks stay where the specification put them.** TS-T11's double-run diff, `goleak`,
`-race`, and the mutation score are the backstop for what static rules approximate. TS-T02 is the
example. The analyzer bans a shape and the double run proves the output. Neither replaces the other.

**One gap that is neither decidability nor judgment: recognition by name.** Four blocking rules
decide compliance from an identifier's name where the behavior is computable. `Done()` matches
any method named `Done` with no receiver check. A channel named `shutdown`, `stop`, `quit`, or
`done` satisfies TS-S03 and TS-C05 with no `close` or send anywhere. Any method named `Reset`
satisfies TS-M05, including an empty one. Every `go` statement inside a function the config names
as a supervisor is exempt, or was until ENG-153 removed the flag, leaving `errgroup` as the only
compliant path and ten correct `WaitGroup` supervisions on querator with no way to pass. The
canceled ticket ENG-177 describes each fix. These are the holes an agent optimizing for green
finds first, and they are the strongest argument in this investigation that the current
implementation falls short of its own exactness claim. They are analyzer work, not a change of
direction.

## The rule set

Every shipped custom rule code, with its disposition. "Keep" means blocking, unchanged. Auto rules
delegated to golangci-lint are out of scope here; the `tiger golangci` audit governs them and
neither trial found a problem with that split.

| Rule | Disposition | Reason |
|---|---|---|
| TS-S02, S03 | keep | 5 real bugs; cursor waiver in place; wave-1.5 grammar widenings held under adversarial review |
| TS-S08 | keep | 3 real bugs; `//tiger:openenum` covers the open-vocabulary case |
| TS-C05 | keep | real bugs still fire after the shutdown-shape tuning |
| TS-C02 | keep, and revive ENG-177 item 1 | 4 real; 10 of 14 are correct `wg.Add`/`wg.Wait` supervision the rule must learn to compute |
| TS-E02 | keep, exempt one shape | the discarded result of a call inside a `defer` statement is the boilerplate-comment case both trials named; the rule caught one real bug outside that shape. ADR-0006 admits tuning when the misfires form a bounded shape that can be named |
| TS-N07 | keep, exempt one shape | do not count a `testing.TB` or `context.Context` leading parameter toward the cap of four; the adjacent-same-type check is untouched and is where the ~30 real hazards live |
| TS-C09 | keep, scope to non-test files | 12 of 12 querator findings are the fan-out-then-`Wait` test idiom; zero production hits on either codebase |
| TS-T02 | keep; grow the allowlist by corpus | inverted rule is exact over its allowlist; 3 of 4 sampled findings are order-insensitive shapes it cannot prove (append-then-sort, nested map deletes, conditional map writes). Extending the allowlist is the blueprint's stated mechanism |
| TS-V01 | convert to pin-only fact | see above; synthesis and pin verification stay, the unpinned-loop finding goes |
| TS-X01 | demote to advisory trial (ADR-0005/0006) | shipped blocking in ENG-152 after both trials; 9 of 9 querator findings are the role-interface idiom and the named remedy is a design regression; needs the git-server rerun before a verdict |
| TS-S09, S06, S07, S18, S21, S22, T06, T10, E06, M10, M05, S01, C12, N08, N14, L09, L10 | keep | true by spec on real code; volume where it exists (S09's 30 copies of one idiom) counts one decision many times, not many wrong decisions |
| TS-F01, F02, F07, V03, P01, P02, K03, A07, A09 | keep | opt-in; fire nothing until declared; zero noise on both codebases |
| TS-L05, L10-distance, D07, L09-escape | keep advisory | the accounting channel; 17 findings across three codebases after ENG-162 |
| TS-D06 | keep | the ratchet is the mechanism that makes every escape honest |

Nothing new enters as a rule. The additions are analyzer fixes (ENG-177's four items), one
silencing-channel closure (below), and the spec and explainer reconciliation.

**What does not ship, and why.** No generic `//tiger:<rule-id>` deviation directive. ADR-0003's
admission test still holds and nothing in either trial produced a finding that a compliant rewrite
could not clear, except the cursor loops ADR-0004 already covers. No per-site config exemption for
signatures pinned by third-party interfaces (ENG-175); the one case on record is a single func
literal in one repo, and an adapter function is the compliant shape. No blocking-finding baseline
for brownfield adoption; a first run against 36k lines produces around 400 findings and the tool
offers no ratchet for blocking rules, which is consistent with the stated goal of AI-written code
and is a real cost for adopting an existing tree. This decision records that cost rather than
adding a baseline, because a baseline is the mechanism ADR-0011 refused for advisories and the
argument is the same.

## The three explainer disagreements

**`//nolint`.** Three documents say three things. The specification allows `//nolint` with a
reason, enforced by `nolintlint` under TS-L09. The explainer says tiger bans it outright and
"flags any `//nolint` as a finding." The code does neither: `tiger check` does not read `//nolint`
at all, and under the golangci-lint plugin a `//nolint:tiger` silences a tiger rule with no
advisory and no count (ENG-178 item 2). The explainer's claim is false about the tool as shipped.

The resolution follows the escape admission test rather than either document. Tiger's own rules
accept no suppression under any driver. The `directives` analyzer already walks every comment; a
bare `//nolint` or one naming `tiger` becomes a blocking TS-L09 finding reported at the file's
`package` clause, where the comment on the original line cannot cover it. That makes the
explainer's sentence true for tiger's rules. For auto rules the golangci-lint linters enforce, a
`//nolint:<linter> // reason` stays permitted, because several of those linters are heuristics
with genuine false positives and ADR-0003 already concedes that door is outside tiger's control.
What changes is that it is no longer uncounted. Each such comment is an escape in the
specification's own words for TS-L09, and the `directives` analyzer counts it as `TS-L09-escape`
against the package budget, the same standing advisory a `//tiger:batched` raises. No uncounted
silence, and no ban on a suppression the third-party linter may actually need.

**`//tiger:restrict`.** The explainer's commit `b452818` removed every mention of the directive
and the "How review divides" section built on it, and ENG-188 asserted that `no-reflect` and
`closed-dispatch` "are not opt-in." The specification, the rule reference, and the code all treat
the restriction set as an opt-in intent declaration enforced by TS-P01, TS-P02, and TS-K03, with
absence never a finding.

Keep the shipped design. Three reasons. Closed dispatch as a global default fails on every real
codebase tiger has run against; querator's four storage backends behind one interface are the
architecture, not a violation. The precision bound in TS-P02 only means anything as a claim a
package makes and its dependencies must support; with no claim there is no bound to compute. And
the directive is consumed, by two analyzers with corpora, which is the test ENG-178 applied when it
proposed dropping `hot`, `wire`, and `owner`. The explainer was half right about one axis:
reflection is already banned by default through the auto rule for TS-S12, and `no-reflect` on the
directive only scopes that. The explainer should say so and should restore the paragraph that
introduces the directive, because a reader who meets TS-P01 in the rule reference with no
explainer entry has no way to learn what a restriction set is.

**Map order.** The explainer says `maporder` bans every map range except a fixed set of safe body
shapes and calls the check "a heuristic rather than a proof." The specification's TS-T02
enforcement line says "range over a map whose body appends or writes. Heuristic." The code does
what the explainer says. The ENG-150 blueprint made that inversion deliberately, called it exact,
and flagged the specification line for amendment. The amendment never landed.

This is not a disagreement about the product. The explainer and the analyzer agree; the
specification is stale, and this ticket's follow-up fixes the enforcement line and the Part V
analyzer table. One wording correction goes the other way. The check is not a heuristic. It bans a
shape and is exact over its allowlist. What it is not is a proof that order never reaches an
output, because the known miss (collect keys, then range the slice) escapes it. The explainer
should say "exact over a conservative allowlist, backstopped by TS-T11" and drop "heuristic." The
querator sample shows the allowlist needs to grow, and the blueprint already says growth is an
analyzer change with a corpus case, never a knob.

## The canceled tickets

Eleven tickets were canceled on the assumption the direction might change. It does not, so most of
them describe work this decision still wants. The disposition, ticket by ticket. Reviving is a
human's call after this document is ratified; nothing here re-opens one.

| Ticket | Disposition |
|---|---|
| ENG-177 recognize behavior, not names | revive as written. The highest-value analyzer work in the backlog |
| ENG-178 close uncounted silencing channels | revive items 2, 3, 4 (`//nolint` under the plugin, `package assert` by import path, drop the three dead intent verbs). Drop item 1; a generated-file header is a bounded shape, and an advisory per generated file is noise of the kind ADR-0006 removes |
| ENG-179 trusted declarations get the escape treatment | revive item 1 (`//tiger:openenum` counted as an escape with a reason). Drop item 2; the TS-E02 shape exemption above replaces the `//tiger:discard` directive, and a new escape verb for the boilerplate case fails the admission test |
| ENG-176 revive naming rules on the config file | leave canceled. Scoping was not the failure |
| ENG-175 interface-pinned signature exemptions | leave canceled. One site, one repo, an adapter is the compliant shape |
| ENG-188 reconcile spec with explainer | superseded by this document and its follow-ups |
| ENG-181 specification gaps | fold into the specification follow-up below |
| ENG-174 JSON output | revive when convenient; tooling, not direction. Both trial reports asked for it |
| ENG-171, 172, 173 auto-fix, editor, dashboards | leave canceled until the rule set above has settled |

## Follow-ups, in order

1. **TS-V01 to pin-only.** Registry and analyzer change; the corpus's unpinned-loop failure cases
   become compliant cases; the specification's TS-V01 line and the explainer's synthesis claim move
   with it. Rerun the querator pin and record the count.
2. **ENG-177, all four items.** Corpus cases for each recognized shape plus known-miss files for
   shapes deliberately not recognized. Rerun both pins.
3. **Shape exemptions for TS-E02, TS-N07, TS-C09**, each with a corpus case naming the excluded
   shape.
4. **Silencing channels.** ENG-178 items 2, 3, 4 and ENG-179 item 1, plus counting
   `//nolint:<linter>` as `TS-L09-escape`.
5. **TS-X01 to advisory**, one registry line, and the git-server rerun that decides it.
6. **TS-T02 allowlist growth** from the querator sample, by corpus case.
7. **Specification reconciliation**, one ticket. TS-T02 enforcement line and the Part V table,
   TS-V01, TS-L09's `//nolint` wording, the restriction-set defaults, TS-X01's severity, and the
   ENG-181 gaps (`tiger.yaml`, `tiger golangci --print`, the rule count).
8. **Explainer reconciliation**, one ticket. Restore the `//tiger:restrict` paragraph, split the
   `//nolint` sentence into the custom-rule ban and the counted auto-rule escape, replace
   "heuristic" on map order, remove "covers nearly every real loop," and restore the fuller
   built-versus-described disclaimer that `b452818` shortened.
9. **An ADR** recording the pin-model rule for provers, which is that a rule needing a proof the
   analyzer cannot always construct ships as a computed fact, never as an unpinned finding. This
   document is the context and the ADR is the binding record.

## What the reviewer should ratify or veto

- TS-V01 becomes a pinned fact rather than a blocking rule. The evidence is one codebase plus two
  fixtures; the git-server pin was not rerun for this ticket.
- `//tiger:restrict` stays opt-in and returns to the explainer. This reverses a call made in the
  explainer commit.
- `//nolint` on auto rules is counted rather than banned. The alternative, banning it for every
  golangci-lint linter tiger enables, is defensible and cheaper to explain; it was not chosen
  because `gocritic`, `gosec`, and `mnd` have documented false positives the project does not own.
- No brownfield baseline for blocking findings. Recorded as a cost, not addressed.
- The canceled tickets are not re-opened by this document.
