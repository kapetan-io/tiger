# Tiger Go

**Tiger deterministically forces AI agents to write Go that is testable and free of whole classes of
production bugs. The specification lives in the source and a machine checks that the code matches it.**

Adapted from [TigerStyle](https://github.com/tigerbeetle/tigerbeetle/blob/main/docs/TIGER_STYLE.md).
Tiger Go is not a style guide. It is a restriction of Go chosen so that questions a reviewer normally
answers by reading become questions a tool answers by checking. The formatting rules are a side
effect, not the point.

This document is the whole specification. Every rule has an ID, a reason that traces to a named
benefit, and an enforcement mechanism. Where a rule cannot be mechanically enforced, it says so and
says what to do instead.

---

# Part I: Why

## The premise

One economic claim sits above everything.

> The cost of a defect grows super-linearly with the distance between the moment it is introduced and
> the moment it is detected. Distance means time, stack frames, call sites, service hops, and
> releases.

Every rule here is one of three strategies against that curve. Shrink the population of possible
defects, shorten the distance to detection, or bound the damage while the distance is being
travelled.

This also explains the goal order, safety then performance then developer experience. A safety defect
is detected by a customer, a performance defect by a dashboard, a developer experience defect
immediately by the person typing. The ordering is a statement about detection distance, not about
which one matters more.

The corollary is the part teams skip. Design is the cheapest place to fix anything, and it is
precisely where nothing can be measured or profiled, which is why the sketch and the mental model come
before the code rather than after the benchmark.

## Enforceability is a property of a rule in a language

This is the insight the rest of the document is built on, and getting it backwards hides most of the
available wins.

> The analyses that are undecidable in general Go are almost all decidable in Go restricted by the
> other rules. The analysis does not get smarter. The language gets smaller.

Alias analysis is the clearest case. In general Go it is hopeless. In a package with no package-level
mutable state, no `unsafe`, no reflection, and no escaping internal pointers, the ways a pointer can
reach a piece of state are small, enumerable, and decidable.

The restrictions compound, and they compound in a specific order:

```
no globals + no unsafe + no reflection
  → the heap graph is reachable only through parameters and receivers
    → the functions that could have written a value are few
      → a reviewer looking at a wrong value can name every function that could have written it
```

Each link is worthless alone. No globals, by itself, is mild hygiene. In the chain it is load-bearing
for a guarantee three steps away. The other half of what a reviewer needs is to know when that set
changes, and `tiger changed` (Part V) reports it: the struct or function that started calling
something new.

An earlier form of this chain continued through computed frame conditions to refactoring that is
provably behavior-preserving, so that a refactoring diff would need no review. Tiger does not make
that proof (ADR-0027). A refactoring diff is reviewed like any other.

The practical consequence: **you do not get sixty percent of the value from sixty percent of the
rules.** You get very little until a chain completes, and then a lot at once. Adopt by chain, not by
ease. The sequencing section at the end gives the order.

## Judgment is conserved

One thing cannot be designed away, and pretending otherwise is how verification efforts fail.

> Judgment is conserved. You can relocate it, you cannot delete it. Every rule promoted from review to
> lint moves a judgment out of the code and into a declaration, and someone still has to be right
> about the declaration.

`TS-S21` does not verify that `BatchMax` is correct. It verifies that `BatchMax` relates to the
constants it must fit inside. The judgment moved from a number in a file to a relation in a
declaration. That is a spectacular trade and it is not a proof.

It is a good trade for three measurable reasons. The declaration layer is small, typically two to five
percent of lines. It is stable, changing an order of magnitude less often than the code it governs.
And it is dense, so attention spent there is spent where being wrong is most expensive, which is the
opposite of how review attention is usually distributed.

## How review divides

The split is not by topic and not by directory. Every concern has two parts, and the parts get
different treatment.

**Machines check the correspondence.** Does the code satisfy the invariant it names, terminate,
cover every state, route its nondeterminism through an injectable surface, respect the declared
layering.

**Humans and AI review the declarations.** Is `SequenceMonotonic` the invariant this protocol actually
needs. Should this struct be calling the disk at all, now that `tiger changed` shows it does. Is
512-byte sectors true of the hardware. Can the `Storage` interface express a torn write.

**The declarations are a layer, not a region.** They cut across every file. There is no part of the
tree that is "the reviewed part", because invariants, limit relations, contracts, and variants sit
next to the code they govern, wherever that code is.

The policy that falls out:

> A change touching a declaration goes to someone who knows the domain. Every change is still read,
> but review does not re-check what CI already checked: termination, coverage, injection, layering.
> Review spends its attention on the declarations and on what `tiger changed` reports.

Patterns give the reviewer something concrete to hold a diff against. For the parts of a correct shape
no rule can check, `tiger pattern <name> --checklist` prints the yes-or-no questions the reviewer answers
while comparing the diff to the pattern's code.

## The nine benefits

"Safety, performance, developer experience" is the right goal hierarchy and useless as a rationale,
because every rule anyone has ever wanted can be argued to serve safety. These nine sit between the
goals and the rules. Every rule cites at least one. A rule that cannot complete the chain from rule to
mechanism to benefit to goal is a preference, and is either labelled as such or deleted.

Each benefit is stated as a claim you could argue with, plus its cost. A benefit with no stated cost
is a slogan.

**B0 Uniformity.** For a class of decisions, the cost of debating it exceeds the cost of any
particular answer, so the value is entirely in everyone doing the same thing. Prevents formatting
diffs that hide semantic diffs, and the same argument every quarter with a new hire. Costs: you will
sometimes follow a convention you think is worse. *Honesty requirement: a B0 rule may never be dressed
up as a safety rule. The correct answer to "why 100 columns" is "because it is 100 everywhere and the
agreement matters more than the number".*

**B1 Bounded execution.** Everything in reality has a limit. Code that does not declare its limit has
one anyway, chosen by whatever runs out first, discovered at the worst time. Prevents infinite loops,
unbounded queues, stack exhaustion, OOM kills, retry storms. Costs: you have to pick the number, and a
bound set wrong is an outage of a different shape. An unbounded system fails globally, late, and far
from the cause; a bounded one fails locally, early, and at a boundary you wrote.

**B2 Short defect distance.** Detection belongs as close to the cause as the language allows, and the
ladder from cheap to expensive is compile error, assertion, test, production. Prevents silent
corruption, swapped argument pairs, truncating conversions, an error surfacing three services from its
origin. Costs: assertions cost cycles and cost availability *on purpose*, trading a correctness bug for
a liveness bug. Sharpest form is the type system, because it moves detection to compile time and the
cost is paid once at the declaration.

**B3 Local reasoning.** Correctness review is bounded by human working memory, which holds a handful
of items. Code requiring more than that will not be reliably verified regardless of who reviews it.
Prevents reviewed-but-wrong code, the compound condition with one unhandled case, the abstraction
whose behaviour lives in four files. Costs: more functions, more names, and a real risk of making
things worse by splitting badly. *This is the benefit most often mistaken for aesthetics. It is a
claim about a cognitive limit, and it predicts that a reviewer will miss a case in a five-clause
condition.*

**B4 Determinism.** A failure you can reproduce on demand is a bug; one you cannot is a rumour.
Reproducibility is the precondition for every other debugging technique. Prevents heisenbugs, flaky
tests, incidents that close as "could not reproduce". Costs: clocks, seeds, and IDs have to be plumbed
through code that has no other use for them.

**B5 Defect discovery leverage.** An assertion is an oracle that every future execution reuses for
free. Bugs found equals assertion density times input diversity, so investing in both multiplies
rather than adds. Costs: density is real work, and a wrong assertion is a false crash in production.

**B6 Predictable cost.** For systems with tail latency or safety requirements, variance is worse than
the mean. Code whose cost is visible in the source can be budgeted; code whose cost depends on the
allocator, the GC, or the branch predictor cannot. Costs: less convenient code, fewer abstractions,
restrictions on the parts of Go that are pleasant precisely because they hide cost.

**B7 Change safety.** Most code is read and modified by someone who did not write it, and the
assumptions in the author's head are not in the file. Rules that make an invalidated assumption fail
loudly at build time protect the majority author, who is the second one. Costs: rigidity. Some changes
now require touching more places, which is the mechanism working and will still feel like friction.

**B8 Blast radius control.** A defect's cost is proportional to how much of the system could have
caused it and how much it can reach. Confinement shrinks the suspect list and the damage together.
Costs: plumbing instead of convenient globals, and reimplementing what a dependency would have given
you free.

## The five questions

A rule is legitimate when it answers all five. Which one it fails tells you what kind of bad rule it
is.

| Question | If it cannot answer |
| --- | --- |
| Which benefit, by ID | **Preference.** Delete it, or admit it is B0 |
| What failure mode, concretely enough to write a test for | **Preference.** A rule with no bug behind it is taste |
| What is the chain from rule to mechanism to benefit, with no step assumed | **Folklore.** Possibly right, with the reason lost, which is dangerous because nobody can extend it to a case it does not literally cover |
| What does following it cost | **Dogma.** A rule claiming zero cost has not been examined |
| What observation would retire it | **Religion.** Unfalsifiable, therefore unimprovable |

A worked example of the folklore failure, from the development of this document. An earlier draft
split the codebase into a verified core and an unverified shell, on the grounds that the core could
then be tested in isolation. When it became clear that testing the core in isolation leaves the bugs
in the shell and at the boundary, the split was restructured into three tiers rather than retired.
The conclusion had outlived its rationale and was being defended on inertia. The correct response was
to delete it, which this specification did.

## Turning judgment into checks

Many rules here look like proxies for a property rather than the property itself. That is deliberate,
and there are seven techniques, worth knowing because you will need them when adding rules.

1. **Nominalize the property.** An analyzer cannot know two assertions concern the same invariant.
   Name the invariant, declare it once, reference it at both sites, and the check becomes reference
   counting.
2. **Move it into the type system.** The compiler is the cheapest analyzer you will ever run.
3. **Require the artifact of the thinking.** You cannot check that someone did the arithmetic. You can
   require the arithmetic in the source and then evaluate it.
4. **Shrink the domain until the relationship is syntactic.** Relatedness between two names anywhere is
   unknowable; between two parameters in one list it is obvious.
5. **Ban the anti-pattern instead of requiring the virtue.** You cannot require a good name; you can
   forbid `data`, `manager`, and `helper`.
6. **Replace the predicate with a budget and a ratchet.** A per-package number that may only decrease
   beats a principle everyone agrees with.
7. **Measure it at run time.** Some properties are decidable, just not statically.

**The test a proxy must pass:** satisfying it must require the same thinking the original demanded.
Otherwise you have built a ceremony and trained the team to satisfy the ceremony instead of the
property. A doc comment on every function fails this test, because it produces `// increment i`. An
exact assertion count per function fails, because it produces `assert.Ok(true, "")`. Error-path
coverage and mutation score pass, because neither can be raised without exercising a path that was
previously unexercised.

---

# Part II: Structure

## Where restrictions live

> **Status: replaced 2026-10-12.** This section was the scoping model: effect sets computed for every
> function, and restriction sets declared per package with `//tiger:restrict` (`TS-P01`, `TS-P02`,
> `TS-P03`, and `TS-K03`'s closed dispatch). ADR-0024 removed the package declarations, and ADR-0026
> removed effect sets and frames. The retired rules are listed under "Retiring a rule".

Go's import granularity is the package, so a restriction on what code may import is package-scoped.
Restrictions are not declared per package. The `TS-X03` layer file lists, for each package, what it
may import beyond the pure standard library, and only the pure standard library is free; that is
how `TS-D01` limits a package. A module without the file has no import limits. Reflection,
`unsafe`, and cgo are banned everywhere by `TS-S12`, with `unsafe` permitted only in
`internal/sys`.

Facts about a single function are not computed into effect sets and are not pinned, except loop
variants (`TS-V01`). When a change makes a struct or free function call something new, `tiger
changed` reports it, and a reviewer decides whether the call belongs there.

## Pins: the analyzer infers, a declaration locks

> **Status: adopted 2026-08-12, narrowed 2026-10-12.** ADR-0026 removed effect and frame pins with
> the `effects` and `frames` analyzers; `tiger changed` (Part V) now shows a reviewer what a
> function newly calls. Pins cover loop variants and function contracts. The model was chosen by
> deliberation against declarations-first and computed-only-first alternatives; the deciding
> evidence was the history of annotation systems. The survivors (Rust lifetimes, TypeScript,
> NullAway) all let inference do the bulk of the work and required annotation only at boundaries,
> while the systems that front-loaded annotation cost before the tool gave value (checked
> exceptions, JML, .NET Code Contracts) died of it.

A **pin** is a declaration tiger proves. There are three: a loop variant (`//tiger:variant`,
`TS-V01`), a precondition (`//tiger:requires`, `TS-V03`), and a postcondition (`//tiger:ensures`,
`TS-V03`). A pin the code does not satisfy is a blocking finding.

A loop variant has a computed baseline: where tiger can synthesize one, no annotation is needed. The
lifecycle:

1. **Unpinned.** The analyzer synthesizes the variant on every run and prints nothing for it.
   `tiger check --show-facts` prints it in pin syntax, the exact line you would paste above the
   loop, and `tiger pin` writes it. Fact output and pin are one format.
2. **Pinned.** The analyzer compares pin and code on every run. Agreement is silence.
   Disagreement fails the check, showing the pin and the reason in the same format. The two exits
   are to change the code or change the pin, and a pin edit is visible in the diff, so a reviewer
   rules on the new promise. A pin cannot go stale: a wrong pin is a failed check, not a lie in a
   comment.

Three rules govern pins:

- **A variant pin attaches to its loop,** wherever the loop lives, because the termination
  obligation belongs to the loop and cannot be checked from an enclosing signature. A contract pin
  attaches to a function.
- **Pins are exact.** Tiger checks a pin in both directions and never accepts one it cannot verify.
- **A variant must be found.** Where the analyzer synthesizes one, the lifecycle above applies.
  Where it cannot, the loop needs a pinned variant or a counter cap that reports when it is hit
  (`TS-V01`, `TS-S02`). That is the only way a pin stops being optional.

What is not pinnable: invariants, surface fault sets, open enums. These have no computed baseline;
they are intent, not observation. You state them first and the analyzer enforces their
consequences. Facts become machine; intent stays human.

## The prerequisite: an assert package

Go has no `assert`. This is the single largest gap between TigerStyle and idiomatic Go, and nothing
else works until you close it. Go gives you `panic`, which the community treats as a smell, and
`error`, which is for something else entirely.

Draw the line clearly and keep it drawn.

> An error is an expected operating condition the caller must handle. An assertion failure means the
> code is wrong, and the only correct response to wrong code is to crash.

A disk returning `ErrNotExist` is an error. A function receiving a negative length is a bug, and
returning an error for it launders the bug into a caller who can do nothing about it either. Assert it
and die.

The companion `assert.go` is the whole package: about a hundred lines, no dependencies, enabled in
production. That last part is the point. Assertions you compile out are assertions that only ever
protected your test suite.

Two Go-specific traps. Variadic arguments are boxed at the call site before the callee tests anything,
so a formatted assertion allocates on every call including the ones that pass; the package therefore
provides no formatted variant, and formatting lives behind an `if` that only runs on failure. And
inlining has a budget, so the failure path is a separate `//go:noinline` function.

## Declarations at a glance

The full declaration vocabulary, gathered in one place. This is the layer that gets reviewed.

Source directives share one namespace, `//tiger:<verb>`, following Go's own directive convention
(`//go:build`, `//go:generate`); the prefix matches the analyzer binary, `tiger`, so seeing a
directive tells you what to run. (The prefix was renamed from `//tigerstyle:` — the project is
defined as not-a-style-guide, and the word `style` does not belong inside every declaration.)
Three kinds with different lifecycles: **pins** (a claim tiger proves, blocking once written, see
above), **intent declarations** (nothing to infer; stated first, enforced after), and **escape
hatches** (loosen one rule at one site, always with a reason, counted by `TS-D06`).

| Declaration | Form | Kind | Rule |
| --- | --- | --- | --- |
| Invariant | `inv.HeaderChecksum`, declared in one package | intent | TS-A07 |
| Precondition | `//tiger:requires len(target) == HeaderSizeBytes` | pin | TS-V03 |
| Postcondition | `//tiger:ensures result.Checksum == checksum(result.Payload)` | pin | TS-V03 |
| Loop variant | `//tiger:variant len(pending)` | pin | TS-V01 |
| Surface fault set | fault cases enumerated in the interface | intent | TS-I05 |
| Layer graph | checked-in `tiger.layers.yaml` | intent | TS-X03 |
| Open enum | `//tiger:openenum <reason>`, counted by `TS-D06` | escape | TS-S08 |
| Batched IO | `//tiger:batched <reason>` | escape | TS-M10 |
| Deviation | `//tiger:<rule-id> <reason>` | escape (gated, does not ship before the first heuristic rules + TS-D06 ratchet) | TS-L09 |

> **Status: amended 2026-08-12.** Escape hatches are gated by an **admission test**: a per-site
> escape exists only where the rules eliminate something reality requires — where
> no compliant rewrite exists. Inconvenience never qualifies, because the primary consumer
> is an AI agent, and an agent offered a cheaper path than conforming will take it. A directive's
> reason is shape-checkable (verb known, reason present) but never truth-checkable, so an escape
> whose legitimate cases have compliant rewrites is a suppression in costume.
> `//tiger:bounded` fails the test and is withdrawn (see `TS-S02`): every terminating loop can
> state an explicit cap and assert on exhaustion, and forever-loops have the `TS-S03` shape.
> `//tiger:batched` passes: an external system that only accepts per-item IO is a fact of the
> world no rewrite removes. The deviation form ships, if ever, together with the first heuristic
> rules and the `TS-D06` ratchet — mechanism and control in one release. Every escape directive
> in a tree additionally counts against its package's `TS-D06` budget on every run and prints
> when the package goes over: shape is machine-checked, truth is human-reviewed, and adding an
> unverified claim is a visible budget change.
>
> **Amended 2026-10-12.** `//tiger:batched` waives `TS-M10` only. ADR-0014 removed its waiver of
> the `TS-S02` loop bound. `//tiger:openenum` carries a reason and is counted the same way, because
> tiger cannot check that an enum really is open (ENG-179). The `hot`, `wire`, and `owner` verbs
> are removed with the rules that used them (ADR-0028).

---

# Part III: The rules

Enforcement levels: **auto** (off-the-shelf linter), **custom** (an analyzer you write, all specified
in Part V), **partial** (heuristic, expect false positives), **runtime** (test or CI gate), **review**
(human or AI judgment, and the reason is always that the declaration is a claim about the world).

Severity is one of two words, used precisely (ADR-0012: a rule either blocks or it is not a rule;
warnings are golangci-lint's job):

- **Blocking.** The CI check fails on the pull request. The default unless stated.
- **Advisory.** Counted per package against the `TS-D06` ratchet's budget file: under budget the
  findings print nothing, and a package over its row fails the check with the counted findings
  printed as blocking lines. Reserved for the standing notices this specification names (every
  escape directive, every skipped test) and for the time-boxed trial of a heuristic rule
  (ADR-0006): start advisory, ratchet per package, then promote or remove.

A synthesized loop variant (`TS-V01`) is a computed fact, not a rule, and carries no severity. It
prints only under `tiger check --show-facts`, in pin syntax, and is what `tiger pin` freezes; a pin
turns the fact into a blocking contract.

**Promotion.** An advisory rule moves to blocking, and a blocking rule moves back to advisory, by a
one-line severity edit in the registry (`internal/rules/rules.go`) — never inside the analyzer, which
stays severity-free (ADR-0002). Promotion requires trial runs on at least two real codebases showing
every finding actionable (no shape the trial judges a false positive) and no open tuning item left
against the rule; demotion is allowed under the same evidence bar, run in reverse. Either way, the
evidence is linked in the promoting or demoting pull request, not restated in the registry itself. See
ADR-0005 for the wave that first drew this line, and the `Assert Adoption Playbook` for the adjacent
question of when a rule that is gated on a codebase precondition (rather than tuning) is worth
building at all.

## Safety and control flow

**`TS-S01` No recursion.**
Why. Bounded execution. A recursive function's stack depth depends on data, and data comes from
outside. Go grows stacks to 1GB by default, so you do not get a clean early crash, you get a process
that quietly eats a gigabyte and dies far from the cause.
Enforce. `norecursion` (custom, SSA static call graph, catches mutual recursion).

**`TS-S02` Every loop has an upper bound.**
Why. Everything in reality has a limit. A loop that does not state its limit is asserting that some
input will always be well-formed. Bound it and an infinite hang becomes a fast, loud failure at the
boundary. See `TS-V01` for the stronger form.
Enforce. `boundedloop` (custom). A `for` with no condition, or a condition that states no bound, is a
finding, and no directive waives it. A bound is a constant, a `len`, a sound counter, or a variant
`TS-V01` verified. Where none exists, the compliant forms are an explicit counter cap and the
`TS-S03` event-loop shape.

A counter is sound only when its comparison is a top-level `&&` term of the condition, not inside
`||` or behind `!`. It pairs `<` or `<=` with `++` or `+=`, or `>` or `>=` with `--`, never `!=`.
Nothing in the body writes the counter or its limit, whether through a helper, a method, a pointer,
or a map insert when the limit is `len(m)`. The counter arrives at the limit before it wraps around the
counter type's range, so `for i := uint8(0); i <= 255; i++` is not sound. A step by `+=` or `-=` is a
constant of at least 1. A limit may call only `len`, `cap`, `min`, `max`, conversions, and declared
functions or methods such as `tuple.Len()`; a function value could return something larger on every pass.

A counter cap is a page size, and needs no report, only when its limit is not a constant and every
other way out of the loop drains a stream whose type is declared outside the module (`rows.Next()`,
`scanner.Scan()`, `decoder.More()`). Reaching a page size means the page is full. Any other way out,
an internal condition or a `break`, makes the counter a safety cap, and the loop must assert or
return an error when the counter reaches the limit, whether the limit is a constant, a parameter, or
a field. A `len` or `cap` limit is a bound the data sets, so it is never a safety cap. A `return` from
the body is a result, not a silent stop, so it does not make a safety cap either. The report is an
`if` after the loop that compares the counter with the limit, or reads a variable the loop assigns,
and then asserts, panics, or returns an error; a bare assert or panic statement after the loop counts
too. The finding's message says so.

Every loop limit is bounded where it enters the program. The analyzer traces a limit back through
parameters, fields, and function results, across packages. Every value written into it must be a
constant, a length, another bounded limit, or a value clamped against a declared maximum, such as
`min(config.Spins, spinsMax)` or an `if count > objectsMax` that rejects. A function that returns a
clamped value carries the bound to its callers. A clamp caps the upper end only, so a limit becomes
unbounded again when it is negated, when anything but a non-negative constant, length or unsigned
value is subtracted from it, when it is multiplied by or combined with an exclusive or against a
value that may also be negative, when a remainder has a divisor that may be negative, or when a
quotient has both a dividend and a divisor that may be negative. A quotient with either one known to
be non-negative, such as `len(items)/step`, stays bounded. Wrap-around at the bottom of the type is
not modelled. A parameter the function writes carries the written value as well as the caller's. A
call that passes `math.MaxInt`, the type's
maximum, or a constant at least half of it is a finding, on integer types of 32 bits or wider, where
no loop reaches half the range. A running total (`n += len(x)`, `n++`) is as bounded as what is added,
because every loop tiger accepts ends. Standard library packages and `_test.go` files are not traced.
Three more things are recorded misses: a limit read through a declared getter such as `q.Len()` while
the body grows `q` through another method; a struct field changed through a global pointer or an
interface method that the loop never passes the struct to; and a cap whose report is any failing `if`
on a variable the loop assigns.

`for ctx.Err() == nil` on a `context.Context` parameter is the event-loop shape, the same as a
`select` on `ctx.Done()`. On a context from `context.Background()` or `context.TODO()` it is not.
Ranging over a standard-library iterator such as `slices.All` or `maps.Keys` is bounded; ranging
over any other iterator is a finding, the same as ranging over a channel.

A store loop restates the limit its query already declares, as in
`for count := 0; count < opts.Limit && rows.Next(); count++`. A loop that filters rows before keeping
them has two limits. The page limit counts kept rows and is clamped to a declared `pageMax`, and a limit of zero or less returns an empty page. The scan
limit counts every row read and is a declared `scanMax`, related by `const _ = uint(scanMax -
pageMax)`; it is a safety cap, so the loop returns an error when it reaches `scanMax`. A
whole-partition scan either declares a maximum partition size, capped with an assert, or pages and
checks `ctx` on every pass.

Known misses, each with a test: a limit passed through a function value
or an interface method is not traced; a constant below the threshold, such as `1 << 40`, passes as
a limit; and a standard-library stream over an endless source inside the program counts as an
outside stream. Ranging over a slice while appending to it ends, so it is a correctness bug for
tests, not for this rule. Counting every row read against the page limit of a filtered loop returns
a short page and passes; the filtered-page pattern covers it.

Patterns: capped worklist, page over an outside stream, clamp a limit, filtered page.
*(Amended 2026-08-12: the `//tiger:bounded <reason>` escape is withdrawn — it fails the escape
admission test, since a reason is shape-checkable but not truth-checkable and every terminating
loop can state a cap. See "Declarations at a glance".)*
*(Amended 2026-10-12: ADR-0014 supersedes the 2026-08-17 amendment that let `//tiger:batched`
waive this rule on cursor-shaped loops. No directive waives it. ADR-0013 and ADR-0014 are built: the
counter, cap, limit, `ctx.Err()`, iterator and store-loop requirements above (ENG-183).)*

**`TS-S03` An unbounded event loop selects on a stop case and asserts progress.**
Why. Some loops genuinely run forever. Those get an explicit termination path and an assertion that
they are still making progress, so a stuck loop reports itself instead of pinning a core in silence.
Enforce. `boundedloop` (custom) for the select shape; the progress assertion is `review`. A stop case
is a receive from `ctx.Done()` on a `context.Context` that does not trace, within the function, to
`context.Background()` or `context.TODO()`, or a receive from or send on a channel the package
itself closes or sends on. For a struct field, the close or send happens in a method of the owning
type that an exported method can reach. A channel's name and its `struct{}` element type do not
count. `TS-S23` adds that the stop case is the loop's only exit. The message names one shape as the
fix: set a flag, close a stop channel once, let the loop's only exit be its stop case, have the loop
close `done`, start it with `wg.Go`, and have every waiter select on `done`. Pattern: shutdown loop.
Known misses: a `close` in dead code counts, and a `close` outside a method of the owning type, such as
a free function or another type's method, is not seen, so that loop is flagged. The goleak harness
(`TS-C03`) and shutdown edge tests are the runtime backstop.
*(Amended 2026-10-12: ADR-0020 replaces the 2026-08-17 amendment that recognized a shutdown channel
by its name or `struct{}` element type. Built (ENG-177).)*

**`TS-S04` Hard limit of 70 lines per function.**
Why. The discontinuity between a function you can see and one you must scroll is real. Go functions
run longer than Zig ones because `if err != nil` costs three lines each time, so 70 is tighter here
than it looks. Push the `if`s up into a parent that owns control flow and the `for`s down into leaves
that own none. The bound is B3; the number is B0.
Enforce. `funlen` (auto).

**`TS-S05` Cyclomatic complexity at most 10, cognitive at most 15, nesting depth at most 3.**
Why. Length alone can be gamed. These catch the wide flat function and the deeply nested one.
Enforce. `cyclop`, `gocognit`, `nestif` (auto).

**`TS-S06` One logical operator per condition.**
Why. `if a && b || c` asks the reader to verify a truth table in their head. Nested `if`/`else` makes
each case visible and gives you somewhere to put the `else` you were about to forget.
Enforce. `compoundcond` (custom). Forbids mixing `&&` with `||`.

**`TS-S07` Split compound assertions.**
Why. `assert.Ok(a && b, ...)` tells you the pair failed. Two assertions tell you which one.
Enforce. `compoundcond` (custom).

**`TS-S08` Every switch over a closed set is exhaustive and ends in `assert.Unreachable`.**
Why. Adding a variant should break every switch that has not thought about it. Go's compiler will not
do this, because `iota` constants are just integers. The default arm covers the value that came off
the wire and was never in your enum.

A type marked `//tiger:openenum` — the directive lives in the type declaration's doc-comment group —
declares a deliberately extensible vocabulary instead of a closed one: a switch over it still needs a
`default` arm, but the default is a legitimate catch-all, not the value that should never occur, so
`assert.Unreachable` is not required and exhaustiveness is not expected. Recognition is same-package
only — `compoundcond` sees directives on type declarations in the package under analysis, so a switch
in a different package from the marked type is a documented known miss.

Enforce. `exhaustive` (auto) plus `compoundcond` for the default arm (custom); a type marked
`//tiger:openenum` waives `compoundcond`'s default-arm-must-`assert.Unreachable` check for switches
over it. The two enforcement halves have independent opt-outs: `exhaustive`'s own escape is
`//exhaustive:ignore` on the switch, unrelated to `openenum`. Marking the type governs only
`compoundcond`'s custom check — a type marked `//tiger:openenum` with no `//exhaustive:ignore` on its
switches still trips the auto rule's exhaustiveness finding.
*(Amended 2026-08-17: `//tiger:openenum` intent declaration added, see "Declarations at a glance".)*
*(Amended 2026-10-12: `//tiger:openenum <reason>` carries a reason and counts against its package's
`TS-D06` budget, because tiger cannot check that an enum really is open. A missing reason is a
`TS-L09` finding. Built in ENG-339.)*

**`TS-S09` No `goto`, no labeled break or continue.**
Why. Simple explicit control flow. A labeled break is usually a loop that wanted to be a function.
Enforce. `nogoto` (custom).

**`TS-S10` No `init()` functions.**
Why. Hidden control flow that runs before `main`, in an order no reader tracks. Initialization you
cannot see is initialization you cannot assert on.
Enforce. `gochecknoinits` (auto).

**`TS-S11` No package-level mutable state.**
Why. A global with extra steps. No owner, defeats deterministic testing, makes parallel tests share
state they never agreed to share. Load-bearing for the chain in Part I.
Enforce. `gochecknoglobals` (auto). Constants and `var Err... = errors.New(...)` allowed.

**`TS-S12` No `unsafe`, no `cgo`, no reflection.**
Why. `unsafe` removes the one memory guarantee Go gives you. `cgo` costs cross-compilation, a
predictable stack, and half your tooling. Reflection moves errors to run time and allocates doing it.
All three break the chain in Part I. The sanctioned exception is `unsafe.Sizeof` in a compile-time
assertion, which executes nothing and reads no memory.
Enforce. `depguard` plus `gocritic` (auto), everywhere; `unsafe` is permitted only in
`internal/sys`, and `reflect` is banned in every package.

**`TS-S13` Declare variables at the smallest scope, at the point of first use.**
Why. A variable declared thirty lines before use is thirty lines in which the wrong variable can be
picked up. Distance in space is distance in time, and that gap is where check-then-use bugs live. See
`TS-Q02` for the structural fix.
Enforce. `ineffassign`, `wastedassign` (auto). Partial — the dead-assignment half only.
*(Amended 2026-08-21: `declusedistance` removed. Across tiger, querator and git-server it produced
500 advisories. Three in five named a declaration whose first use sits inside a deeper block — an
accumulator before its loop, where "move the declaration down" is a reset bug or a compile error.
One in eight counted a multi-line composite literal's own body as the distance. Textual distance
cannot tell a hoisted variable from a necessary one; the rule is human review.)*

**`TS-S14` No shadowing.**
Why. `err` shadowed inside an `if` is the most common way a Go program silently discards a failure.
Enforce. `govet` with `shadow` (auto). See `TS-K05` for the stricter form.

**`TS-S15` Struct literals name every field.**
Why. Positional literals break silently when a field is inserted. Naming every field, including ones
set to their zero value, is the Go form of passing options explicitly. A zero value you typed is a
decision; one you omitted is an accident waiting for someone to change the struct.
Enforce. `exhaustruct` for config and wire types, `govet composites` everywhere (auto).

**`TS-S16` No magic numbers. Every limit is a named constant with a `Max` or `Min` suffix.**
Why. A limit you can find is a limit you can change, assert on, and reason about. A `4096` buried in a
slice expression is none of those.
Enforce. `mnd` (auto).

**`TS-S17` Assert relationships between constants at compile time.**
Why. A design invariant checked before the program runs is worth ten checked at runtime. Go has no
`comptime`, but it has constant overflow, and that is enough.

```go
// Fails to compile unless the header is exactly 128 bytes.
const _ = uint(unsafe.Sizeof(Header{}) - HeaderSize)
const _ = uint(HeaderSize - unsafe.Sizeof(Header{}))

// Fails to compile unless *Replica satisfies Member.
var _ Member = (*Replica)(nil)
```

Enforce. `review`. *(Amended 2026-10-12: the `wiretypes` check on `//tiger:wire` types is removed
with the `wire` verb, ADR-0028.)*

**`TS-S18` No naked `panic` outside the assert package.**
Why. One crash path, one message format, one place to change the behaviour.
Enforce. `forbidigo` on `^panic$` (auto) plus `paniccheck` (custom) for the assert exemption. `revive`
`deep-exit` covers `os.Exit` and `log.Fatal`.

**`TS-S19` Integer conversions are checked or provably safe.**
Why. `int32(n)` where `n` came from `len()` is a truncation waiting for a large input. Go converts
silently, which is what made `int` versus `usize` a rule in the original.
Enforce. `gosec` G115 plus `unconvert` (auto).

**`TS-S20` Type assertions are always checked.**
Why. The one-result form panics on failure, far from whoever produced the wrong type.
Enforce. `forcetypeassert` and `errcheck` with `check-type-assertions` (auto).

**`TS-S21` Every limit constant participates in a compile-time relational assertion.**
Why. Whether `BatchMax = 8189` is correct is arithmetic about your hardware and no analyzer will know.
Whether you believe it relates to anything else is checkable, and a limit that relates to nothing is a
limit nobody reasoned about.
Enforce. `limitrelate` (custom). Every `Max`/`Min` constant must appear in at least one `const _ =
uint(...)` expression in its package.

**`TS-S22` Write a limit as an expression of its named inputs.**
Why. A limit written as `8189` hides where it came from, so nobody can tell it went stale when an
input changed. Written as `(WriteBufferSize - HeaderSizeBytes) / EntrySize`, it updates itself.

```go
const WriteBufferSize = SectorSize * SectorsPerWrite

const BatchMax = (WriteBufferSize - HeaderSizeBytes) / EntrySize
```

Enforce. `review`. *(Amended 2026-10-20: the `derivation` analyzer and the derivation comment are
retired, ADR-0031. `TS-S22` fired no findings in the ENG-191 trials, and it read free prose, which
this specification no longer lets change a verdict. The maxim stays.)*

**`TS-S23` An event loop's only exit is its stop case.**
Why. A `return` or a break-out elsewhere in a `TS-S03` loop ends the loop without the shutdown
handshake, so waiters block on a `done` channel nobody closes. This is the shape behind ENG-160.
Enforce. `boundedloop` (custom). In a `TS-S03` loop, no `return` or break-out may sit outside the
recognized stop case. It is a syntax check, so it is exact. It enters as an advisory trial under
ADR-0006, measured on querator and git-server, before it blocks. It also flags a correct
handshake design, so its message names the `net/http` shape, closing a stop channel, as the fix. Pattern: shutdown loop. Built (ENG-177).
The stop case here is a case on a live `ctx.Done()` or a channel the package closes. A case on a
channel the package only sends on, which carries requests as easily as a shutdown handshake, counts
only when it ends in a `return`. Without that, a request channel that `Submit` sends on would count
as a stop case and exempt the return that skips shutdown, which is the ENG-160 shape.

## Assertions and invariants

**`TS-A01` Average at least two assertions per function.**
Why. This is the rule that carries most of the safety, and the one teams quietly drop. The density
target stops assertions clustering in the three functions someone was worried about that week.
Enforce. `assertdensity` (custom), per package and per file. Blocking at package level, advisory per
function.

**`TS-A02` Assert every precondition on entry and every postcondition before return.**
Why. A function must not operate blindly on data it has not checked. Its purpose is to raise the
probability the program is correct, which it cannot do if it trusts its inputs. See `TS-V03` for the
modular form that moves this to compile time.
Enforce. `assertdensity` (custom). Partial.

**`TS-A03` Assert the invariant on both sides of every boundary.**
Why. Validate before writing to disk and again after reading back. Two independent checks on the same
property catch the case where one of them was wrong. Enforced through the invariant vocabulary below,
because "the same property" is not a thing an analyzer can see in two boolean expressions.
Enforce. via `TS-A07` and `TS-A08`.

**`TS-A04` Assert the negative space.**
Why. Bugs live where data crosses from valid to invalid. Asserting only what you expect leaves the
transition unguarded.
Enforce. via `TS-A09`, `TS-T03`, and `TS-T09`.

**`TS-A05` Assertions never have side effects.**
Why. They must be removable in principle even though you never remove them. An assertion that mutates
means the program behaves differently under test than in production.
Enforce. `review`. *(Amended 2026-10-12: the `effects` analyzer that was to enforce this through
`TS-F04` is removed, ADR-0026.)*

**`TS-A06` Distinguish index, count, and size.**
Why. The usual suspects. An index is 0-based, a count is 1-based, a size is a count times a unit, and
every conversion between them is an off-by-one waiting to happen.
Enforce. via `TS-Q01`, which makes it a compile error.

### The invariant vocabulary

Stop passing strings to assertions. Declare the invariant, and the properties an analyzer could never
see become reference counting and set equality.

```go
// Package inv declares every invariant the storage engine enforces.
package inv

type ID string

const (
	HeaderChecksum    ID = "header-checksum"    // Header checksum matches the payload.
	HeaderSize        ID = "header-size"        // Header occupies exactly HeaderSizeBytes.
	SequenceMonotonic ID = "sequence-monotonic" // Sequence numbers never decrease within a term.
	BatchFitsBuffer   ID = "batch-fits-buffer"  // Batch length never exceeds the write buffer.
)
```

```go
func encodeHeader(target []byte, h Header) {
	assert.Invariant(inv.HeaderSize, len(target) == HeaderSizeBytes)
	assert.Invariant(inv.HeaderChecksum, h.Checksum == checksum(h.Payload))
}

func decodeHeader(source []byte) Header {
	assert.Invariant(inv.HeaderSize, len(source) == HeaderSizeBytes)
	h := ...
	assert.Invariant(inv.HeaderChecksum, h.Checksum == checksum(h.Payload))
	return h
}
```

The ID's value is entirely static — it is an address the analyzer counts (`TS-A07`), compares
(`TS-A08`), and matches to a violating test (`TS-A09`); at run time `assert.Invariant` behaves
exactly like `assert.Ok`. The underlying type is a string rather than an int for two reasons: the
failure message names the invariant with no generated code, and the assert package accepts any
`~string` ID via generics — coupled by shape, not by import — so `assert` keeps zero dependencies
and `inv` stays project-owned.

**`TS-A07` Every declared invariant is asserted in at least one function outside test files.**
Why. A declared invariant nobody asserts is a design document that lies. Whether it is asserted on
both sides of a boundary is `TS-A08`'s question; whether a test can make it fail is `TS-A09`'s. A
property with one production site (a sequence number checked only where it is appended) is still an
invariant, and keeps its violating test.
Enforce. `invariantrefs` (custom). Functions in `_test.go` files do not count. A whole-program rule:
the const is declared in one package and asserted from the packages importing it, so the finding is
decided in the finish step only `tiger check` runs (ADR-0010); the golangci-lint plugin never
reports it.

**`TS-A08` Symmetric boundary functions assert the same invariant set.**
Status. **Unimplemented.** No `invariantsymmetry` analyzer exists and nothing enforces this rule;
removing one side's assertion passes `tiger check`. Kept for review under ENG-182, which decides
whether naming-convention pairing is worth building or the id is retired like `TS-F06`.
Why. Once invariants are named, "the same property on both sides" is set equality.
Enforce. `invariantsymmetry` (custom, not built). Pairs by naming convention (`encodeX`/`decodeX`,
`marshalX`/`unmarshalX`, `writeX`/`readX`, `putX`/`getX`).

**`TS-A09` Every invariant has a test that violates it.**
Why. The negative space made concrete. If no test can make the invariant fail, either the invariant is
unreachable or the assertion is wrong, and you want to know which.
Enforce. `invariantnegative` (custom). Requires `assert.Violates(inv.X, func(){...})` in a
`_test.go` file. A whole-program rule, reported only by `tiger check` (ADR-0010).

Three things fall out that the string version never gave you. Grep an invariant and get every site
defending it. The invariant list becomes a design document that cannot go stale, because a deleted
assertion fails `TS-A07`. And a fake invariant costs a declaration, a call site, a symmetric
counterpart where a boundary exists, and a negative test, which is more work than thinking of a real one.

## Quantities and types

**`TS-Q01` Domain quantities are named types. Conversions live in one package and are named.**
Why. The original treats index, count, and size as distinct types and relies on naming discipline to
keep them apart, because Zig will not. Go will. A named type cannot be assigned to another without a
conversion, so the discipline becomes a compile error instead of a convention.

```go
type Index int   // 0-based position.
type Count int   // 1-based quantity.
type Bytes int64 // Size in bytes.

// CountFromIndex converts a 0-based position to a 1-based quantity.
// This is the one place the plus one lives.
func CountFromIndex(i Index) Count {
	assert.Ok(i >= 0, "negative index")
	return Count(i) + 1
}
```

Enforce. Compiler, plus `quantitycast` (custom) forbidding direct conversions outside the quantity
package.

**`TS-Q02` Parse, do not validate. Exported functions take domain types, not bare primitives.**
Why. The mechanizable form of "check variables close to where they are used". The check-to-use gap
exists because a checked value and an unchecked value have the same type, so the check cannot travel
with the value. Give the checked value its own type whose only constructor asserts, and the gap closes
structurally. A function taking an `AccountID` cannot be called with an unvalidated string no matter
how far apart the lines are.
Enforce. `domaintypes` (custom). Exported functions may not take `string`, `[]byte`, or unsized
integers except through an allowlist.

**`TS-Q03` Units are types, not name suffixes.**
Why. `latencyMsMax` is a good name and `TS-N03` still applies, but a `Millis` type means the compiler
catches the nanosecond you passed. `time.Duration` is the proof this works.
Enforce. Compiler, plus `quantitycast` (custom).

## Errors

**`TS-E01` Every error is handled. No exceptions.**
Why. The USENIX analysis of catastrophic failures in distributed data-intensive systems found the
overwhelming majority were caused by incorrect handling of non-fatal errors the software had already
explicitly signalled. Go hands you every error as a value, which makes this the one rule the language
is on your side for. Best-evidenced rule in the document.
Enforce. `errcheck` with `check-blank: true` (auto).

**`TS-E02` An error discarded with `_` is a finding, whatever comment sits next to it.**
Why. A discarded error is a failure nobody sees, and tiger cannot tell a reason from `// x`, so a
comment cannot be what clears the rule. Handle the error.
Enforce. `errignore` (custom). A discard inside a `defer` statement's call, or inside a function
literal passed to `t.Cleanup`, is not a finding; every other discard is, including non-deferred
cleanup such as `_ = resp.Body.Close()`. *(Amended 2026-10-12: cleanup exemption, ADR-0017.
Amended 2026-10-20: the comment exemption is removed, ADR-0031.)*

**`TS-E03` Wrap once with `%w`, compare with `errors.Is` and `errors.As`.**
Why. String comparison breaks the first time someone reformats a message. Double wrapping produces
messages nobody can read.
Enforce. `errorlint`, `wrapcheck` (auto).

**`TS-E04` Sentinel errors are `ErrFoo`, error types are `FooError`, messages lowercase and unpunctuated.**
Why. Consistency, and because wrapped messages concatenate; a capitalized message reads wrong mid-chain.
Enforce. `errname`, `staticcheck` ST1005 (auto).

**`TS-E05` Never return both a nil value and a nil error.**
Why. It forces every caller to invent a third case, and half of them get it wrong.
Enforce. `nilnil` (auto).

**`TS-E06` Minimize return arity.**
Why. Every return value is a branch at the call site, and branches propagate up the chain. Preference
order: nothing, `T`, `(T, bool)`, `(T, error)`. Never `(T, bool, error)`.
Enforce. `returnarity` (custom, at most two results, at most one non-error) plus `ireturn` (auto).

**`TS-E07` Close errors on writable resources are checked.**
Why. On a writer, `Close` is where the flush happens, so `defer f.Close()` on a file you wrote to
discards the one error that mattered.
Enforce. `errcheck` (auto) plus `review` for deferred-close-with-named-return.

**`TS-E08` No naked returns.**
Why. A named result returned implicitly is a value the reader must reconstruct from the whole function.
Enforce. `nakedret` with `max-func-lines: 0` (auto).

## Concurrency

This area has no counterpart in the original, because Zig gave TigerBeetle a single-threaded event
loop and static allocation. Go gives you a scheduler and a `go` keyword that is two characters long,
which is the most dangerous ergonomics in the language.

**`TS-C01` Prefer a single owner goroutine over shared memory with locks.**
Why. The same instinct as the control plane and data plane split. One goroutine owns a piece of state,
everyone else sends it messages, and the whole class of data race disappears rather than being
defended against.
Enforce. via `TS-C11` and `TS-C13`, with `-race` as the runtime backstop.

**`TS-C02` Every goroutine starts through a group that waits for it.**
Why. A goroutine with no owner is a leak you will find in production, at a memory number nobody can
explain. Start it through a group whose `Wait` is reached, never with a bare `go` mid-function.
Enforce. `nogoroutine` (custom). Only `errgroup.Group.Go` and `sync.WaitGroup.Go` start a goroutine;
a bare `go` statement is always a finding, and there is no supervisor allowlist. The group's `Wait`
must be reached on every success path after the start: in the same function, or, for a group held in
a struct field, in an exported method of the owning type. A path that returns a non-nil error is not a
success path. A `Shutdown` that returns early because its own context
expired is exempt on that path. The message names the shutdown-loop shape as the fix (see
`TS-S03`). Patterns: goroutine owner, shutdown loop.
*(Amended 2026-10-12: built, ADR-0020 and ENG-177. A group received as a parameter is the caller's
to wait on, and `nogoroutine` does not follow it.)*

**`TS-C03` Goroutine leaks fail the test suite.**
Why. Checkable at run time and not at compile time, so check it at run time, and check statically
that the run-time check exists.
Enforce. `goleak` in `TestMain` (runtime), plus `nogoroutine` (custom) for the harness: a package
that starts goroutines through `go`, `wg.Go`, or `errgroup.Go` and has no goleak harness in
`TestMain` is a finding. goleak alone misses a goroutine that exits just after `Shutdown`, so the
static `TS-C02` and `TS-S03` checks stay. *(Amended 2026-10-12: the harness check is built, ADR-0020 and
ENG-177. It reads the package's `_test.go` files from disk, so a `TestMain` in the `_test` package counts.)*

**`TS-C04` Every channel has an explicit capacity, and the capacity is a named constant.**
Why. Put a limit on everything. An unbuffered channel is a bound of zero and that is a fine answer; an
unbounded queue is not an answer at all. Go will not let you build an unbounded channel, which is a
gift, so do not rebuild one out of a slice and a mutex.
Enforce. `mnd` (auto), `queuebound` (custom) for slice-backed queues.

**`TS-C05` Every blocking receive or send has a stop case.**
Why. Shutdown that hangs is shutdown that gets `SIGKILL`ed, and a process killed mid-write is a
process that lost data.
Enforce. `selectctx` (custom). A select needs a `default` or a stop case, with the same definition
as `TS-S03`: a receive from `ctx.Done()` on a `context.Context` that does not trace, within the
function, to `context.Background()` or `context.TODO()`, or a receive from or send on a channel the
package itself closes or sends on (for a struct field, in a method of the owning type reachable from
an exported method). A channel's name or `struct{}` element type does not count, and a bare receive
or send gets no exemption by name. Pattern: shutdown loop.
*(Amended 2026-10-12: ADR-0020 replaces the 2026-08-17 amendment that recognized a shutdown channel
by name or `struct{}` element type. Built (ENG-177).)*

**`TS-C06` No mutex held across a channel operation or a call that can block.**
Why. It is how you build a deadlock without noticing.
Enforce. `review`, with `-race` and the `TS-C03` goleak harness as runtime backstops.
*(Amended 2026-10-12: the `effects` analyzer that was to enforce this through `TS-F05` is removed,
ADR-0026.)*

**`TS-C07` `context.Context` is the first parameter, never stored in a struct, and `context.TODO` never ships.**
Why. Cancellation has to propagate or the bounds everywhere else are decorative.
Enforce. `containedctx`, `contextcheck`, `noctx`, `revive` (auto).

**`TS-C08` No `time.Sleep` in production code.**
Why. Sleep is a guess about someone else's timing. Use a channel, a timer you own, or a condition you
can assert on.
Enforce. `forbidigo` (auto) bans the direct call. A helper three frames down that sleeps is not
caught statically; `tiger changed` reports a struct's new call into `time` for review.

**`TS-C09` Run at your own pace. Do not spawn work in direct reaction to an external event.**
Why. Straight from the original, doubly true in Go where the natural shape is one goroutine per
request. Accept into a bounded queue, then drain at a rate you control. Batching, back pressure, and a
bound on work per unit time all fall out of the same decision.
Enforce. `nogoroutine` (custom) structurally; the design is `review`. `_test.go` files are not
checked: a concurrency test fans out on purpose, and a single-worker queue would defeat it. *(Amended
2026-10-12: test-file exemption, ADR-0019.)*

**`TS-C11` No exported method returns a pointer, slice, or map that aliases an internal field.**
Why. This is where single ownership actually leaks in Go. `func (s *State) Entries() []Entry` hands
out a mutable view of your state and nothing in the type system says so. Returning a pointer to a
field is the syntactic form of aliasing, and unlike general alias analysis it is a five-line check.
Load-bearing for the chain in Part I.
Enforce. `escapecheck` (custom).

**`TS-C12` Channel types used across goroutines are declared in one file per package.**
Why. Centralizing the communication surface makes the topology reviewable in one place and puts the
capacity constants next to each other, where a mismatched pair is visible.
Enforce. `chandecl` (custom).

**`TS-C13` One concurrency paradigm per package.**
Why. A package sharing state through channels *and* mutexes has two mental models, and every reader
must work out which applies to the field in front of them. This is the checkable half of `TS-C01`,
because "which paradigm" is set membership even though "is this correct" is not.
Enforce. `paradigm` (custom). A package with a `sync.Mutex` field may not also have a channel field,
absent a directive.

## Termination and contracts

**`TS-V01` Every unbounded loop has a verified variant: synthesized where the analyzer can, pinned where it cannot.**
Why. `TS-S02` checks a loop has a bound and cannot check the bound is ever reached, so a `for` with a
decreasing condition and a body that never decrements passes. A variant is a ranking expression
required to strictly decrease on every back edge and be bounded below, which is the standard
termination argument. A variant must be *found*. Where the analyzer synthesizes one, the loop is
proven with no annotation, the synthesized variant prints under `--show-facts` like any computed
fact, and pinning it is a paste; the pin attaches to the loop, wherever the loop lives. Where
synthesis fails, the termination argument exists only in the author's head, so the pin is required:
mandatory because the fact is missing, not because precision is claimed. This is the
artifact-of-the-thinking technique — the analyzer cannot find your ranking, so you write it down and
the analyzer checks it.

```go
// The analyzer synthesizes len(pending) for this loop unaided; the pin locks it.
//tiger:variant len(pending)
for len(pending) > 0 {
	entry := pending[0]
	pending = pending[1:] // Analyzer verifies the variant strictly decreases here.
}
```

The analyzer synthesizes a variant for these shapes:

- a fixed step, such as `i--`, `i += 2`, or `s = s[1:]`
- division by a constant greater than 1, or a shift by a constant of at least 1, under a strict
  `> 0` condition, as in `for size > 0 { size >>= 7 }`
- slicing by the length of a part the analyzer can show is not empty, as in
  `data = data[len(chunk):]` where `chunk` is `data` or `data[:max]` with `max` a constant above 0
  and `chunk` is not reassigned before the slice
- a struct field in the condition, such as `len(q.Items) > 0`, when no call in the body receives
  `q` or a pointer to it
- a condition joined with `&&` where one part shrinks, as in `for x > 0 && y > 0 { x--; y-- }`; a
  negated condition does not count

A proof is sound: variables are matched by identity, not by name, so a shadowing copy does not
count. Growth through a pointer or a closure made before the loop defeats a proof, and so does a
bound at the counter type's maximum (`i <= math.MaxInt`).

Enforce. `variant` (custom, AST and `go/types`). Blocking when a loop has neither a synthesized nor a pinned
variant, and when a pinned variant fails to decrease. A loop whose termination argument is beyond
the analyzer's predicate language takes a counter cap that reports when it is hit (`TS-S02`).
`TS-S02` accepts a loop whose variant this rule verified, whether synthesized or pinned.
*(Amended 2026-10-12: the four shapes after the fixed step, the soundness fixes, and `TS-S02`
accepting a verified variant are built, ADR-0013. ADR-0014 closes the ENG-183 clash with
`//tiger:batched`, which no longer waives a loop bound; that is built.)*

**`TS-V02` Functions are total.**
Why. Total means it returns for all inputs satisfying its preconditions. With `TS-S01` and `TS-V01`
every function terminates, and a total function is one whose worst case you can state.
Enforce. `variant` (custom) for termination, through `TS-S01` and `TS-V01`. Whether a function can
panic or block without bound is `review`. *(Amended 2026-10-12: the `effects` half that excluded
`panic` and unbounded `block` is removed, ADR-0026.)*

**`TS-V03` Preconditions are declared and discharged at call sites.**
Why. `TS-A02` asserts preconditions inside the callee, which is a runtime check. Declaring them lets
the analyzer verify at each call site that the condition is established by a dominating assertion, by
a type, or by a preceding check, which moves detection a rung up the B2 ladder. This is what makes the
assertion layer modular rather than local.

```go
//tiger:requires len(target) == HeaderSizeBytes
//tiger:ensures result.Checksum == checksum(result.Payload)
func decodeHeader(target []byte) Header
```

Enforce. `contracts` (custom, abstract interpretation over a restricted predicate language: nil-ness,
integer ranges, length relations, invariant IDs). Partial by nature. **Unproven obligations degrade to
a runtime assertion rather than a build failure**, which is the right default because it never blocks
on the analyzer's incompleteness.

## Surfaces and injectability

A surface is where the code meets something it doesn't control, and there are two. The inbound surface is the exported
functions and methods a caller reaches. The outbound surface is the set of
interfaces the domain calls to reach the world, the store, the clock, the network, and the rules in
this section govern it. Unqualified, "surface" in this section means the outbound one.

Co-equal with the rule catalogue, not a testing detail. The rules above make properties checkable;
these make the checked properties describe the running system rather than a model of it.

The distinction that matters is between injection and description. Injection means the code takes a
`Storage` interface and calls `s.Write(...)`; IO is swappable but still performed, and every test
needs a fake whose fidelity you must also be right about.
Description means the code returns commands as values and something else interprets them. Prefer
description where the shape allows it. Require injection everywhere else.

**`TS-I01` Every nondeterministic input is injectable.**
Status. The direct half is enforced: `TS-T01`'s bans stop a function reading the clock or a random
source directly. A helper that reads them, or a direct IO call, is not caught statically. When a
change makes a struct or free function call into `time`, `os`, `net`, or a database driver for the
first time, `tiger changed` reports it, and review decides whether the call should have been
injected. No `surfaces` analyzer exists and none is planned; the earlier form of this rule, which
required a declared surface, is withdrawn, and so is its enforcement through effect pins
(ADR-0026).
Why. Time, randomness, IO, scheduling, and identity are the inputs that make a run unrepeatable. A
function that reads them directly cannot be handed a substitute, so no test can control the run.
Injectability is the requirement; whether a given test substitutes the dependency or uses the real
one is the coder's judgment per test, and tiger does not make it. Tiger does not require a fake,
mock, or simulation to exist.
Enforce. `forbidigo` (auto) for direct reads of the clock, and, when the module has a `TS-X03`
layer file, that file for imports of randomness by a package whose entry doesn't name them;
`review` with the `tiger changed` report for the rest.

**`TS-I02` Every surface has a production implementation and a simulated one.**
Status. **Deleted.** Fails the five-question test: its failure mode (a surface with one
implementation) is exactly the condition under which `TS-I03`'s conformance obligation cannot run,
its enforcement is the same implementation counting `surfaces` performs for `TS-I03`, and its
marginal cost is zero — every cost it has is `TS-I03`'s cost. Two IDs, one check. The existence
requirement is folded into `TS-I03`; the default-fidelity claim moved to `TS-I07`. ID retired, not
reused.

**`TS-I03` Where a surface has a simulated implementation, both implementations pass one conformance suite.**
Status. **Not enforced by tiger.** A simulated implementation is optional (ENG-154 decision, see
`TS-I01`); this rule applies only where the coder chose to write one. No `surfaces` analyzer exists.
Why. If the fake and the real diverge, you have verified a world that does not exist. One suite run
against both is the only mechanism that keeps them interchangeable.
Enforce. `review`, plus CI running the suite against both implementations where both exist
(runtime).

**`TS-I04` The simulated implementation is at least as adversarial as the specification permits.**
Why. A fake that behaves better than reality hides the bugs you built the simulator to find. Inject
every fault the spec allows, not the ones you have happened to observe.
Enforce. `review` of the fault set, plus a CI check that every fault case in the interface is exercised
by the suite (runtime).

**`TS-I05` A surface interface must be able to express every fault its specification permits.**
Status. **Not enforced by tiger.** A design discipline for the interface, decided in review. No
`surfaces` analyzer exists.
Why. **This is the rule that decides whether any of the rest is worth anything.** If a real disk can
produce a torn write, a misdirected write, or an `fsync` that lies, and your `Storage` interface cannot
represent those, the simulator is structurally blind to an entire bug class and will run green
forever. Interfaces get designed around their failure modes rather than their happy-path ergonomics,
which makes them uglier and is the point. Whether the fault set matches reality is irreducibly
`review`.
Enforce. `review`.

**`TS-I06` Adapters contain no logic.**
Status. **Deleted.** Tiger cannot tell an adapter from any other implementation without a surface
declaration, which does not exist, and "no logic beyond error mapping" is a design judgment: a retry
or a reconnect inside a wrapper is logic, and whether it belongs there is the coder's call. ID
retired, not reused.

**`TS-I07` Fidelity is configuration, not code.**
Status. **Not enforced by tiger.** No default configuration is mandated; which implementation a test
receives is the coder's choice per test (ENG-154 decision). No `surfaces` analyzer exists.
Why. Where both a simulated and a real implementation exist, the same suite runs against either with
the difference being injection config. This is what stops the simulated and real paths diverging into
two test suites that check different things.
Enforce. `review`.

## Memory and performance

Go's GC means "no dynamic allocation after startup" cannot be stated, let alone enforced. What
survives is the reason behind it: allocation is unpredictable latency, and unpredictable latency is
what you were trying to avoid.

**`TS-M01` Preallocate with a known capacity.**
Why. `append` into a nil slice in a loop is a growth curve, a copy per doubling, and garbage. When you
know the size, say it.
Enforce. `prealloc`, `makezero` (auto).

**`TS-M03` Pass structs larger than 64 bytes by pointer.**
Why. A by-value parameter copies, and a copy in a loop is a copy per iteration. Go has no `const`, so
the immutability the original got for free is simply lost; the copy avoidance is the part that
survives.
Enforce. `gocritic` `hugeParam` and `rangeValCopy` (auto).

**`TS-M04` Initialize large structs in place through an out pointer.**
Why. Same as the original minus the pointer stability argument, which Go's non-moving GC already
gives you. `func (r *Replica) Init(...) error` beats returning a large value, and it is mandatory for
anything containing a mutex, which cannot be copied at all.
Enforce. `govet copylocks` (auto) for locks, `outptr` (custom) for size.

**`TS-M05` `Put` is preceded by zeroing the value.**
Why. Buffer bleeds. Go zeroes every `make`, so the classic case is gone, but `sync.Pool` and `buf[:0]`
reuse bring it straight back. Zero on release, not on acquire, so the leak cannot outlive the owner
who knew how much was sensitive.
Enforce. `poolzero` (custom). Only `*b = T{}` before `Put` counts as a reset; a `Reset` method call
does not, since an empty `Reset` passes any name check. Exact. Pattern: pool reuse.
*(Amended 2026-10-12: built, ADR-0020 and ENG-177; `poolzero`'s false finding on `*b = T{}`
after `pool.Get` is fixed.)*

**`TS-M07` No `fmt.Sprintf` where `strconv` will do.**
Why. Each formatting call is an allocation and a dynamic dispatch. `strconv` over `fmt` is usually a
straight win.
Enforce. `perfsprint` (auto), everywhere. *(Amended 2026-10-12: the hot-path scope and the `alloc`
effect are removed with the `hot` verb, ADR-0028; this is the rule as it ships.)*

**`TS-M09` Batch across every boundary you cross.**
Why. Amortize network, disk, and syscall costs. A design decision made at design time, when you cannot
yet measure. Sketch the four resources and their bandwidth and latency before writing the code, because
after implementation the cheap wins are gone.
Enforce. via `TS-M10` for the anti-pattern; the design is `review`.

**`TS-M10` No IO inside a loop body.**
Why. You cannot check that a design batches, but you can check for its opposite, which has a shape. A
`Write`, `Query`, or syscall inside a `for` is the anti-batching pattern, and also every N plus one
query ever written.
Enforce. `ioinloop` (custom), unless annotated `//tiger:batched <reason>`. The annotation counts
against its package's `TS-D06` budget on every run and prints when the package goes over, so an
escape is never free. This is the only rule `//tiger:batched` waives (ADR-0014).

## Abstraction

**`TS-X01` No speculative interfaces.**
Why. Abstractions are never zero cost and every one adds a leak risk. An interface added for a
substitution nobody makes has paid the cost and bought nothing.
Enforce. `review`. Whether an interface is the right seam is a design judgment. A shape rule cannot
tell a speculative interface from one that narrows a capability, such as git-server's `RefReader`,
and a fix that deletes the interface damages the second kind. *(Amended 2026-10-12: the
`singleimpl` check on interfaces with one implementation is removed, ADR-0016.)*

**`TS-X02` No pass-through methods.**
Why. A method whose body is one forwarding call with unchanged arguments is indirection that exists to
be counted, not used. Each one lengthens the chain a reader must follow to find code that does
something.
Enforce. `passthrough` (custom). Budget of zero.

**`TS-X03` The package import graph is declared and enforced.**
Why. The control plane and data plane split cannot be verified, but its main consequence can. The
direction of every dependency is a fact you write down once and check forever. Layering is the
enforceable residue of architecture. A package that does IO almost always imports the package that
does it, so a package whose list leaves out `net`, `database`, `os` and the drivers keeps that IO
behind an interface passed in. A few pure standard library packages reach the file system or the
network on their own (`path/filepath.Glob`, `html/template.ParseFiles`, `log/syslog`,
`crypto/tls`), so the list is a strong default, not a proof.
Enforce. `layers` (custom), against a checked-in layer file. `tiger.layers.yaml` sits at the module
root. Its keys are packages, `.` for the root package, `./dir` for one package, and `./dir/...` for
a directory and everything below it; keys may not overlap. Each key lists what that package may
import beyond the pure standard library, in three spellings, each matched exactly. `./pkg` is one
package of this module, not a package below it. `module/path` allows that path and every path below
it on a `/` boundary. `net`, `os`, `database`, `syscall`, `plugin`, `io/ioutil`, `crypto/rand` and
`math/rand` are the standard library that leaves the process or is nondeterministic, and naming one
allows its whole subtree. The rest of the standard library, every package under the GOROOT that
`go env GOROOT` reports, is always allowed; a dotless third-party module brought in by `replace` or
`go.work` is a module like any other. A nested module, a directory with its own `go.mod`, is not
part of this module: its packages need no entry, and an import of one names its module path.
`_test.go` files are not layered, because surface tests cross layers. The analyzer reports each
import the package's entry does not allow, at the import, and a package with no entry at its package
clause. `tiger check` also reports, from the module's directories, an entry that matches no package
directory and a package with no entry that the analyzer did not report (one outside the patterns
passed, or built only for another platform); a directory is a package when it holds a non-test Go
file, whatever its GOOS or GOARCH constraints; a file marked
`//go:build ignore` does not count, because no platform builds it, and the legacy `// +build` form
is not read. A finding in a generated file is kept for this rule, because a generated file can import the IO the
layer file keeps out. Those checks need the module's
directories, so the golangci-lint plugin does not run them, and stock golangci-lint without the
plugin reports nothing for this rule. Matching is exact, and the rule is tiger's own, because
depguard's prefix match against the nearest allow entry cannot express it (ADR-0029).
*(Amended 2026-10-14: built as the layers analyzer, not generated depguard rules, ADR-0029.)*

## Canonical form

An intermediate representation has one spelling per meaning. Go has several for most things, and every
alternative spelling is a diff a human must read to confirm it means nothing.

**`TS-K01` One spelling per construct.**
Why. If a construct has one legal form, any diff touching it is semantic by construction, and a
reviewer never spends attention confirming a rewrite was cosmetic.
Rules. No `else` after a terminating branch. Error checks in the `if err != nil` form only, never
inverted. `for` over a range where a range is possible. Composite literals always keyed. No
parentheses beyond what precedence requires. Single-form nil checks.
Enforce. `canonical` (custom) plus existing auto-fixers. Auto-fixed on commit.

**`TS-K02` One import path per package, one alias per import path, module-wide.**
Why. Aliasing the same package differently across files defeats cross-file mechanical transforms and
makes grep lie.
Enforce. `canonical` (custom). Auto-fixed.

**`TS-K04` Declaration order is total and deterministic.**
Why. A partial order leaves choices. A total order makes file layout a function of its contents, so a
tool can regenerate it and two implementations of one interface are diffable line by line.
Enforce. `canonical` (custom). Auto-fixed. Subsumes the older const/var/type/func ordering.

**`TS-K05` No shadowed identifier anywhere in a function, including across scopes.**
Why. `TS-S14` bans shadowing in nested scopes. Unique names within a function body make every
identifier occurrence resolvable without scope analysis, which is what lets a rename or an
extract-function transform be textual and still sound.
Enforce. `canonical` (custom).

## Mechanical refactoring

These follow from the rules above. Tiger does not prove that a refactor preserves behavior: the
rule that would have, `TS-R01`, rested on computed effect sets and frames and is retired
(ADR-0027).

**`TS-R02` Public surface changes are diffed against the previous release.**
Why. Every exported signature, contract, and invariant forms a surface. A machine-readable diff tells you the semantic version, decides whether downstream must act,
and catches the accidental widening nobody meant to publish.
Enforce. `surfacediff` (custom, checked-in surface file, CI diff).

**`TS-R03` Generated and hand-written code are indistinguishable and interchangeable.**
Why. The property that matters most in the near future, and the one a conventional style guide never
has to think about. If the code is canonical and contract-checked, a
machine-generated diff faces exactly the same gate as a human one, and "who wrote this" stops being
the basis on which trust is assigned. Trust attaches to what the artifact provably satisfies.
Enforce. Follows from `TS-K01` through `TS-K05`, plus a CI check that generated files carry no
weakened directives.

## Naming

**`TS-N01` `MixedCaps`. Acronyms keep their case.**
Why. `snake_case` does not survive contact with Go. The standard library, struct tags, and every tool
assume `MixedCaps`, and exported-ness is encoded in the first letter. `VSRState`, `HTTPServer`, `id`
never `Id`.
Enforce. `staticcheck` ST1003, `revive` var-naming (auto).

**`TS-N02` No invented abbreviations. A short allowlist and nothing else.**
Why. The original bans abbreviations outright; Go's own convention wants short names in short scopes.
The compromise that holds is a closed vocabulary. `i`, `n`, `ok`, `err`, `ctx`, `buf`, `r`, `w`, `b`,
`t` are allowed because every Go reader knows them. `cfg`, `mgr`, `svc`, `tmp` are not, because you
invented them this morning.
Enforce. `varnamelen` with an explicit allowlist plus `noabbrev` (custom).

**`TS-N03` Units and qualifiers go last, sorted by descending significance.**
Why. `latencyMsMax`, not `maxLatencyMs`. Then `latencyMsMin` lines up under it and everything about
latency sorts together. Big-endian naming, so alphabetical order becomes semantic order.
Enforce. `unitsuffix` (custom).

**`TS-N04` Infuse names with meaning.**
Why. Name the lifetime and the ownership, not the type. `alloc Allocator` is fine; `gpa` and `arena`
tell the reader whether cleanup is their problem.
Enforce. Review (human).

**`TS-N05` Related names have the same length.**
Why. `source` and `target` beat `src` and `dst` because `sourceOffset` and `targetOffset` line up in
the slice expression underneath, and the eye catches an asymmetry the brain would skim past.
Enforce. Review (human).

**`TS-N06` A helper is prefixed with the name of its caller.**
Why. The name carries the call history, so `readSector` and `readSectorRetry` sort together and read
in order.
Enforce. Review (human).
*(Amended 2026-08-21: `declorder` no longer enforces this. The suggested rename compounds down a
call chain — each level's prefix becomes the next level's suggestion, so `checkEventLoop` yields
`checkEventLoopSelectHasTerminationCase` and the better-decomposed file is punished harder. It
produced 392 advisories across the three measured codebases, 284 of them against tiger's own
analyzers, burying the escape-hatch and skipped-test advisories that exist on purpose.)*

**`TS-N07` Two or more parameters of the same type means you need an options struct.**
Why. Go has no named arguments, so `Copy(src, dst string, retries, timeout int)` is a call site where
the compiler cannot help you and a swapped pair is a silent bug.
Enforce. `sametypeparams` (custom). Flags adjacent identical types and functions over four parameters.
A leading `testing.TB`, `*testing.T`, or `*testing.B`, and a `context.Context` that comes first or
right after it, do not count toward the four; the adjacent-same-type half is unchanged.
*(Amended 2026-10-12: the leading-parameter exemption, ADR-0018.)*

**`TS-N08` A boolean parameter is a lie. Use a named type.**
Why. `Save(true)` means nothing at the call site. `Save(SyncImmediate)` means something.
Enforce. `sametypeparams` (custom).

**`TS-N09` Prefer nouns to participles.**
Why. `replica.pipeline` can be a heading in a design doc. `replica.preparing` must be rephrased first,
and derived names like `pipelineMax` compose where `preparingMax` does not.
Enforce. via `TS-N14`.

**`TS-N10` No stutter, no context-dependent overloading.**
Why. `replica.ReplicaState` reads badly. Worse, reusing a term that already means something else in
your domain costs a whole team a whole meeting.
Enforce. `revive` (auto) for stutter; overloading is `review` against a domain glossary.

**`TS-N11` Money and other exact quantities are integers, never floats.**
Why. Binary floating point cannot represent a tenth.
Enforce. `nofloat` (custom), applied to the packages a `tiger.yaml` entry lists for it.
*(Amended 2026-10-12: it was scoped by `TS-P01`, which ADR-0024 removed.)*

**`TS-N14` Exported identifiers do not end in a present participle.**
Why. Enforces `TS-N09`. A trailing `-ing` is a suffix check, so nouns-over-participles turns out to be
almost free.
Enforce. `participle` (custom), allowlisting genuine nouns like `Encoding` and `Logging`.

## Layout and comments

**`TS-L01` `gofmt` is not negotiable. `gofumpt` on top.**
Why. Zero-config formatting is the single best thing about Go tooling. Do not fight it, configure it,
or discuss it.
Enforce. `gofumpt` (auto).

**`TS-L02` Lines at most 100 columns, tab counted as 4.**
Why. Two files side by side, nothing behind a scrollbar. Pure B0; the number is a Schelling point and
arguing about it is the most expensive thing you can do with it.
Enforce. `lll` (auto).

**`TS-L03` Declaration order is `const`, `var`, `type`, `func`.**
Why. A file read top-down should introduce its vocabulary before using it.
Enforce. `decorder` (auto). Subsumed by `TS-K04` where canonical form is adopted.

**`TS-L04` Important things near the top. Entry points first.**
Why. A file is read top-down on the first pass. `main` first, then the type the file is about, then
its constructor, then its methods, exported before unexported.
Enforce. Subsumed by `TS-K04` (`canonical`, chain 6, not yet shipped).
*(Amended 2026-08-17: `declorder` removed as an enforcer — it covers `TS-L05` only, deliberately not
TS-L04, since the spec already marks it subsumed by `TS-K04` and that belongs to `canonical`, out of
this wave.)*

**`TS-L05` Struct order is fields, nested types, constructor, methods.**
Why. Same reason, one level down. Complex nested types get promoted to top level.
Enforce. `declorder` (custom). Advisory.
*(Audited 2026-08-21: five findings across tiger, querator and git-server, each a constructor or
method sitting above the type it belongs to. Low volume, and the move it asks for is the right
one.)*

**`TS-L06` Comments are sentences.**
Why. Comments are prose describing the code, not scribbling in the margin. Trailing comments may be
fragments.
Enforce. `godot` (auto).

**`TS-L07` Every exported identifier has a doc comment starting with its name.**
Why. It is what `go doc` prints, and the only documentation most callers read.
Enforce. `revive` exported rule (auto).

**`TS-L08` Always say why.**
Why. Code says what. A comment repeating the what is noise; one giving the reason gives the reader the
criteria to evaluate your decision and to change it correctly later. The most important rule here and
the least mechanizable.
Enforce. `review`. *(Amended 2026-10-20: no longer applies, ADR-0031. Comments in production code are
banned rather than judged, and `TS-L12` is dropped. ENG-411 removes this rule and `TS-L13`.)*

**`TS-L09` Every escape-hatch directive carries a reason.**
Why. An escape without a reason is a rule silently deleted. Applies to `//nolint`,
`//tiger:batched`, `//tiger:openenum`, and the `//tiger:<rule-id> <reason>` deviation form when
that ships. Pins and the other intent declarations are exempt: they carry expressions, not excuses,
and tightening a contract needs no justification. *(Amended 2026-08-12: `//tiger:bounded` removed
from the list — the escape itself is withdrawn under the admission test; see `TS-S02`.)* Beyond the
reason check, every escape directive, and every `//tiger:openenum`, counts against its package's
`TS-D06` budget on every run and prints when the package goes over, so adding one is a visible
budget change for as long as it exists.

`tiger check` never honors `//nolint`. An adopter who wants `//nolint` gone turns the ban on in
`tiger.yaml`:

```yaml
nolint:
  forbid: true
  reason: agents must fix findings, not silence them
```

With the entry on, every `//nolint` in a checked package is a blocking finding, reported at the
file's `package` line, where a `//nolint` on the original line cannot cover it. Under the
golangci-lint plugin, a file-wide `//nolint:tiger` or an `exclusions` rule silences tiger's rules
whether or not the entry is on, which is one reason CI runs `tiger check`. A counted
`//nolint:<linter>` waiver is not part of tiger.
Enforce. `nolintlint` with `require-explanation` (auto) plus `directives` (custom): blocking on
a missing reason, an unknown verb, and, with `nolint.forbid` on, any `//nolint`; advisory on each
escape present. *(Amended 2026-10-12: the `nolint.forbid` entry and the `openenum` reason, ADR-0023
and ENG-179, are built in ENG-339.)*

**`TS-L10` Blank lines group acquisition with its `defer`, and no `defer` inside a loop.**
Why. Put acquire and release next to each other and a leak becomes visible. A `defer` in a loop is an
unbounded queue of cleanup, which violates `TS-S02`.
Enforce. `deferdistance` (custom). Blocking on the loop case, advisory on distance.

**`TS-L11` Imports grouped standard, external, internal.**
Why. Mechanical, so let the machine do it.
Enforce. `gci` (auto).

**`TS-L13` Commit bodies contain a `Why:` section.**
Why. A pull request description is not in the repository and is invisible in `git blame`, so the
reason must be in the commit or it is gone.
Enforce. CI check on message shape (runtime). *(Amended 2026-10-20: no longer applies, ADR-0031.
ENG-411 removes it.)*

## Dependencies and tooling

**`TS-D01` The pure standard library, and nothing else, unless the layer file allows more.**
Why. Every dependency is a supply chain, a safety surface, a performance unknown, and an install-time
cost, amplified through everything built on top. Go's standard library makes this genuinely
achievable, which is not true of most languages.
Enforce. `layers` (custom), the `TS-X03` analyzer: with a layer file, a package imports the pure
standard library only unless its entry names more. Without the file nothing is enforced. No finding
prints `TS-D01`; every one is a `TS-X03` finding, so see `TS-X03` for how it is enforced. *(Amended
2026-10-12: the per-package allowlists from `TS-P01` are removed, ADR-0024. Amended 2026-10-14:
enforced by the `TS-X03` analyzer instead of `depguard`, ADR-0029.)*

**`TS-D02` Dependencies are vendored and pinned, including the toolchain.**
Why. Reproducible builds. `go.mod` pins the toolchain, `vendor/` pins the rest, CI verifies both.
Enforce. `go mod verify` and a vendor diff check (runtime).

**`TS-D03` Scripts are Go programs, not shell.**
Why. Cross-platform, type-checked, and it runs the same on everyone's machine instead of hitting a
Bash version difference. Write `cmd/ci/main.go`, not `scripts/ci.sh`.
Enforce. CI check for `*.sh` outside an allowlist (runtime).

**`TS-D04` Generated code is committed and verified.**
Why. If `go generate` output is not in the tree, the build depends on a tool version nobody pinned.
Enforce. CI regenerates and diffs (runtime).

**`TS-D05` `TODO` and `FIXME` reference a tracker ID that exists and is open.**
Why. A dated comment is a decision to defer. Deferring is allowed; forgetting is not.
Enforce. `godox` plus a CI call to the tracker (runtime).

**`TS-D06` Per-package budgets may only decrease.**
Why. What a zero-technical-debt policy looks like on a codebase that already has some. Check in a file
of numbers per package: total complexity, directive count, skipped tests, assertion density,
allocation counts on benchmarks. A global threshold gets raised at 2am before a release; a ratchet
cannot be raised without a commit that says so. **Track the directive count as a first-class metric**,
because forced directives are how a rule set degrades into a style guide with extra steps.
Enforce. `tiger check` compares each package's advisory counts to its rows in `tiger.budget.yaml`
and fails on overrun; `tiger budget --write` lowers rows to the current counts and never raises
one; the budget file is raised only by a hand edit in review (custom, driver-enforced).

**`TS-D07` Skipped tests are counted on every run.**
Why. A skipped test is a test that passes. Visibility beats paperwork: a count recomputed from the
code on every run cannot go stale, while a tracker reference can outlive its issue. The count is
advisory against the `TS-D06` budget, so a new skip prints once its package goes over.
Enforce. `skipcheck` (custom).

## Testing and determinism

**`TS-T01` Core logic is deterministic. Time, randomness, and IDs are injected.**
Why. This is what makes simulation testing possible, and simulation testing is what found the bugs
that made TigerBeetle worth imitating.
Enforce. `forbidigo` (auto) bans direct reads of the clock. When the module has a `TS-X03` layer
file, it denies `math/rand` and `crypto/rand` to a package whose entry doesn't name them; without
the file, randomness is not checked. A helper that reads them is not caught statically;
`tiger changed` reports a struct's new call into `time` or a random source for review, as `TS-I01`
describes. *(Amended 2026-10-12: the `time` and `rand` effects are removed, ADR-0026.)*

**`TS-T02` Map iteration order never reaches an output.**
Why. Go randomizes it deliberately. Sort the keys before you serialize, log, or hash.
Enforce. `maporder` (custom). A range over a map is a finding unless its body matches an allowlist of
shapes that cannot depend on order, such as summing, counting, or writing into another map. A
finding means the loop's shape is outside the allowlist, not that the output varies.

- **Plain sorts.** A map loop that appends into a local slice passes when the slice's first use
  after the loop is `sort.Strings`, `sort.Ints`, or `slices.Sort` on string or integer elements.
  Floats are excluded, because -0 and +0 compare equal and print differently.
- **Full coverage.** A comparator sort passes only when its comparisons together cover every field
  of the element, using `cmp.Compare`, `strings.Compare`, or `bytes.Compare` on a whole array, so
  the comparator returns 0 only for identical elements. Pointer, interface, bool, float, and
  `time.Time` fields never count as covered. A key type's `Compare` method that passes the same
  check counts too, as in `slices.SortedFunc(maps.Keys(m), K.Compare)`. Any other comparator sort
  fires, and its message names `slices.Sorted(maps.Keys(m))` followed by `slices.SortStableFunc`.
- **The `maps` escape is closed.** A `maps.Keys`, `maps.Values`, or `maps.All` result fires unless it
  goes to `slices.Sorted`, a full-coverage `slices.SortedFunc`, `maps.Collect`, `maps.Insert`, or a
  range checked like a map range. `slices.Collect(maps.Keys(m))` is a finding.
- **No reads of carried state.** A map write gated by a counter, such as copying the first three
  entries visited, fires.

Five shapes pass despite depending on order: last writer wins (`inverse[v] = k` when two keys share
a value), a map write whose value calls a function (`labels[k] = fmt.Sprint(v)`),
`golang.org/x/exp/maps.Keys`, `reflect.Value.MapKeys`, and a range over a type parameter constrained
to a map. `TS-T11` can catch them on the paths the tests exercise, though two runs sometimes visit entries
in the same order. Pattern: sorted map output when
the key has no order.
*(Amended 2026-10-12: plain sorts, full coverage, the closed escape, and the carried-state check are
decided and not built, ADR-0021.)*

**`TS-T03` Every parser, decoder, and state machine has a fuzz target.**
Why. Tests must be exhaustive, with invalid data as well as valid, and with valid data going invalid.
A fuzzer covers the negative space you did not think of. It proves the presence of bugs and never
their absence, so it is the last line of defence and not the first.
Enforce. `fuzzcoverage` (custom).

**`TS-T04` No `time.Sleep` in tests. Use `testing/synctest`.**
Why. A sleep in a test is a race you decided to lose slowly. `synctest` gives you a fake clock and a
deterministic scheduler.
Enforce. `forbidigo` scoped to `_test.go` (auto).

**`TS-T05` `-race` on every CI run. Seeds are logged and reproducible.**
Why. A failing simulation you cannot replay is a rumour. Every simulation run logs a seed sufficient
to replay it exactly.
Enforce. CI (runtime).

**`TS-T07` Error-path branch coverage is measured separately with its own floor.**
Why. The evidence behind `TS-E01` says catastrophic failures come from mishandling errors the software
already signalled, which makes error paths the code most worth covering and least likely to be
covered. An aggregate coverage number hides this, because error branches are a small fraction of
blocks and a large fraction of risk.
Enforce. CI, parsing the coverage profile and restricting to error-guarded blocks (runtime).

**`TS-T08` Mutation score floor.**
Why. The closest thing to a machine check for "tests must test exhaustively". Coverage says a line
ran; a mutation score says the test would have noticed if the line were wrong. **This is also the
anti-Goodhart oracle**: it is the one metric here that cannot be raised without the tests genuinely
improving, so a rising conformance score against a flat mutation score is a fire.
Enforce. Nightly mutation run (runtime).

**`TS-T09` Every parser has a rejection corpus, and every file in it is asserted to be rejected.**
Why. Turns "test with invalid data" from an intention into a file count, and it grows every time the
fuzzer finds something.
Enforce. `corpuscheck` (custom).

**`TS-T10` Table-driven tests have named cases.**
Why. A case with a `name` field explains its own intent in the failure output, which is where the
reader will be standing when they need it.
Enforce. `tablename` (custom).

**`TS-T11` The suite runs twice and the output is diffed.**
Why. `TS-T01` bans the sources of nondeterminism it knows about and `TS-T02` checks map iteration
against an allowlist with five known misses; neither proves the result. Running twice with the same seed and diffing does prove it,
for the paths the suite covers, and it costs one CI job rather than one analyzer. Any diff is either a
nondeterminism you did not know about or a test logging a timestamp.
Enforce. CI (runtime). Not built.

**`TS-T12` Tests live in the external `_test` package.**
Why. A test file in `package foo` reaches every unexported name, so it pins internals a caller never
sees and breaks on every refactor. `package foo_test` can only use the exported API. `export_test.go`
and any other file that re-exports internals gets no exception, and neither does `package main`, since
`main_test` can import and test it. The rule is on by default with no setting to turn it off.
Enforce. `testpackage` (custom). Every `_test.go` file must declare a `_test` package. `tiger check` also
reads the package clause of build-constrained and generated test files in the packages it analyzed, which
the loader skips. Files under
`testdata/` are not checked, because the Go tool never loads them as packages.

---

# Part IV: Traceability

Every rule, the benefits it serves, and its class. **Sub** means the rule prevents a named failure
mode. **Coord** means the value is in unanimity and the specific answer is arbitrary. **Mixed** means
the rule is substantive and its particular form is convention.

Retired and deleted IDs stay in the table, so a reference to one in an old commit still resolves.
Orphans, meaning rules citing no benefit: zero. That is the point of maintaining the table. A rule
that cannot fill the third column does not go in.

| ID | Rule | Benefits | Class |
| --- | --- | --- | --- |
| TS-P01 | Retired, package declarations removed (ADR-0024) | — | Retired |
| TS-P02 | Retired, package declarations removed (ADR-0024) | — | Retired |
| TS-P03 | Retired, package declarations removed (ADR-0024) | — | Retired |
| TS-S01 | No recursion | B1, B6 | Sub |
| TS-S02 | Every loop has an upper bound | B1 | Sub |
| TS-S03 | An unbounded event loop selects on a stop case and asserts progress | B1, B2 | Sub |
| TS-S04 | Hard limit of 70 lines per function | B3, B0 | Mixed |
| TS-S05 | Cyclomatic complexity at most 10, cognitive at most 15,... | B3, B0 | Mixed |
| TS-S06 | One logical operator per condition | B3, B2 | Sub |
| TS-S07 | Split compound assertions | B2 | Sub |
| TS-S08 | Every switch over a closed set is exhaustive and ends i... | B7, B2 | Sub |
| TS-S09 | No goto, no labeled break or continue | B3 | Sub |
| TS-S10 | No init() functions | B3, B4 | Sub |
| TS-S11 | No package-level mutable state | B8, B4 | Sub |
| TS-S12 | No unsafe, no cgo, no reflection | B8, B6, B2 | Sub |
| TS-S13 | Declare variables at the smallest scope, at the point o... | B3, B2 | Sub |
| TS-S14 | No shadowing | B2 | Sub |
| TS-S15 | Struct literals name every field | B7 | Sub |
| TS-S16 | No magic numbers. Every limit is a named constant with... | B1, B7, B3 | Sub |
| TS-S17 | Assert relationships between constants at compile time | B2, B7 | Sub |
| TS-S18 | No naked panic outside the assert package | B2, B3 | Sub |
| TS-S19 | Integer conversions are checked or provably safe | B2 | Sub |
| TS-S20 | Type assertions are always checked | B2 | Sub |
| TS-S21 | Every limit constant participates in a compile-time rel... | B1, B7 | Sub |
| TS-S22 | Write a limit as an expression of its named inputs | B1, B7, B6 | Sub |
| TS-S23 | An event loop's only exit is its stop case | B1, B2 | Sub |
| TS-A01 | Average at least two assertions per function | B5, B2 | Sub |
| TS-A02 | Assert every precondition on entry and every postcondit... | B2, B5 | Sub |
| TS-A03 | Assert the invariant on both sides of every boundary | B2, B5 | Sub |
| TS-A04 | Assert the negative space | B5 | Sub |
| TS-A05 | Assertions never have side effects | B4, B2 | Sub |
| TS-A06 | Distinguish index, count, and size | B2 | Sub |
| TS-A07 | Every declared invariant is asserted in at least one fu... | B2, B5, B7 | Sub |
| TS-A08 | Symmetric boundary functions assert the same invariant set | B2 | Sub |
| TS-A09 | Every invariant has a test that violates it | B5 | Sub |
| TS-Q01 | Domain quantities are named types. Conversions live in... | B2, B7 | Sub |
| TS-Q02 | Parse, do not validate. Exported functions take domain... | B2, B7, B3 | Sub |
| TS-Q03 | Units are types, not name suffixes | B2 | Sub |
| TS-E01 | Every error is handled. No exceptions | B2 | Sub |
| TS-E02 | An error discarded with _ is a finding, whatever comm... | B2, B7 | Sub |
| TS-E03 | Wrap once with %w, compare with errors.Is and errors.As | B2, B7 | Sub |
| TS-E04 | Sentinel errors are ErrFoo, error types are FooError, m... | B0, B7 | Mixed |
| TS-E05 | Never return both a nil value and a nil error | B3, B7 | Sub |
| TS-E06 | Minimize return arity | B3 | Sub |
| TS-E07 | Close errors on writable resources are checked | B2 | Sub |
| TS-E08 | No naked returns | B3 | Sub |
| TS-C01 | Prefer a single owner goroutine over shared memory with... | B8, B4, B3 | Sub |
| TS-C02 | Every goroutine starts through a group that waits for it | B1, B8 | Sub |
| TS-C03 | Goroutine leaks fail the test suite | B1 | Sub |
| TS-C04 | Every channel has an explicit capacity, and the capacit... | B1, B6 | Sub |
| TS-C05 | Every blocking receive or send has a stop case | B1 | Sub |
| TS-C06 | No mutex held across a channel operation or a call that... | B1, B6 | Sub |
| TS-C07 | context.Context is the first parameter, never stored in... | B1, B8 | Sub |
| TS-C08 | No time.Sleep in production code | B4, B6 | Sub |
| TS-C09 | Run at your own pace. Do not spawn work in direct react... | B1, B6 | Sub |
| TS-C10 | Retired with the `owner` verb (ADR-0028) | — | Retired |
| TS-C11 | No exported method returns a pointer, slice, or map tha... | B8 | Sub |
| TS-C12 | Channel types used across goroutines are declared in on... | B3, B1 | Sub |
| TS-C13 | One concurrency paradigm per package | B3 | Sub |
| TS-F01 | Retired, replaced by `tiger changed` (ADR-0026) | — | Retired |
| TS-F02 | Retired, replaced by `tiger changed` (ADR-0026) | — | Retired |
| TS-F03 | Retired with effects and the `hot` verb (ADR-0026, ADR-0028) | — | Retired |
| TS-F04 | Retired, TS-A05 is review (ADR-0026) | — | Retired |
| TS-F05 | Retired, TS-C06 is review (ADR-0026) | — | Retired |
| TS-F06 | Deleted, superseded by TS-F01 + TS-T01 | — | Deleted |
| TS-F07 | Retired, replaced by `tiger changed` (ADR-0026) | — | Retired |
| TS-F08 | Retired, TS-C02 decides ownership at the start site (ADR-0020, ADR-0026) | — | Retired |
| TS-V01 | Every unbounded loop has a verified variant: synthesize... | B1 | Sub |
| TS-V02 | Functions are total | B1, B6 | Sub |
| TS-V03 | Preconditions are declared and discharged at call sites | B2, B3 | Sub |
| TS-I01 | Every nondeterministic input is injectable | B4, B5 | Sub |
| TS-I02 | Deleted, folded into TS-I03 | — | Deleted |
| TS-I03 | Where a surface has a simulated implementation, both pas... | B4, B2 | Sub |
| TS-I04 | The simulated implementation is at least as adversarial... | B5 | Sub |
| TS-I05 | A surface interface must be able to express every fault... | B5, B4 | Sub |
| TS-I06 | Deleted, not decidable without a surface declaration | — | Deleted |
| TS-I07 | Fidelity is configuration, not code | B4, B7 | Sub |
| TS-M01 | Preallocate with a known capacity | B6 | Sub |
| TS-M02 | Retired with the `hot` verb (ADR-0028) | — | Retired |
| TS-M03 | Pass structs larger than 64 bytes by pointer | B6 | Sub |
| TS-M04 | Initialize large structs in place through an out pointer | B6 | Sub |
| TS-M05 | Put is preceded by zeroing the value | B8, B2 | Sub |
| TS-M06 | Retired with the `wire` and `hot` verbs (ADR-0028) | — | Retired |
| TS-M07 | No fmt.Sprintf where strconv will do | B6 | Sub |
| TS-M08 | Retired with the `hot` verb (ADR-0028) | — | Retired |
| TS-M09 | Batch across every boundary you cross | B6 | Sub |
| TS-M10 | No IO inside a loop body | B6 | Sub |
| TS-M11 | Retired with the `hot` verb (ADR-0028) | — | Retired |
| TS-X01 | No speculative interfaces | B3, B6 | Sub |
| TS-X02 | No pass-through methods | B3 | Sub |
| TS-X03 | The package import graph is declared and enforced | B8, B3 | Sub |
| TS-K01 | One spelling per construct | B7, B0 | Mixed |
| TS-K02 | One import path per package, one alias per import path,... | B7, B0 | Mixed |
| TS-K03 | Retired, package declarations removed (ADR-0024) | — | Retired |
| TS-K04 | Declaration order is total and deterministic | B0, B7 | Mixed |
| TS-K05 | No shadowed identifier anywhere in a function, includin... | B2, B7 | Sub |
| TS-K06 | Retired with the `wire` verb (ADR-0028) | — | Retired |
| TS-R01 | Retired, tiger does not prove refactors (ADR-0027) | — | Retired |
| TS-R02 | Public surface changes are diffed against the previous... | B7 | Sub |
| TS-R03 | Generated and hand-written code are indistinguishable a... | B7, B2 | Sub |
| TS-N01 | MixedCaps. Acronyms keep their case | B0 | Coord |
| TS-N02 | No invented abbreviations. A short allowlist and nothin... | B3 | Sub |
| TS-N03 | Units and qualifiers go last, sorted by descending sign... | B3, B0 | Mixed |
| TS-N04 | Infuse names with meaning | B3 | Sub |
| TS-N05 | Related names have the same length | B3, B0 | Mixed |
| TS-N06 | A helper is prefixed with the name of its caller | B3, B0 | Mixed |
| TS-N07 | Two or more parameters of the same type means you need... | B2, B7 | Sub |
| TS-N08 | A boolean parameter is a lie. Use a named type | B3, B2 | Sub |
| TS-N09 | Prefer nouns to participles | B3, B0 | Mixed |
| TS-N10 | No stutter, no context-dependent overloading | B0 | Coord |
| TS-N11 | Money and other exact quantities are integers, never fl... | B2 | Sub |
| TS-N14 | Exported identifiers do not end in a present participle | B3, B0 | Mixed |
| TS-L01 | gofmt is not negotiable. gofumpt on top | B0 | Coord |
| TS-L02 | Lines at most 100 columns, tab counted as 4 | B0, B3 | Coord |
| TS-L03 | Declaration order is const, var, type, func | B0 | Coord |
| TS-L04 | Important things near the top. Entry points first | B0, B3 | Coord |
| TS-L05 | Struct order is fields, nested types, constructor, methods | B0 | Coord |
| TS-L06 | Comments are sentences | B0 | Coord |
| TS-L07 | Every exported identifier has a doc comment starting wi... | B3, B0 | Mixed |
| TS-L08 | Always say why (no longer applies, ADR-0031) | B7, B3 | Sub |
| TS-L09 | Every escape-hatch directive carries a reason | B7, B2 | Sub |
| TS-L10 | Blank lines group acquisition with its defer, and no de... | B2, B3 | Sub |
| TS-L11 | Imports grouped standard, external, internal | B0 | Coord |
| TS-L13 | Commit bodies contain a Why: section (no longer applies, ADR-0031) | B7 | Sub |
| TS-D01 | The pure standard library, and nothing else, unless the lay... | B8, B6 | Sub |
| TS-D02 | Dependencies are vendored and pinned, including the too... | B4, B8 | Sub |
| TS-D03 | Scripts are Go programs, not shell | B4, B0 | Mixed |
| TS-D04 | Generated code is committed and verified | B4 | Sub |
| TS-D05 | TODO and FIXME reference a tracker ID that exists and i... | B7 | Sub |
| TS-D06 | Per-package budgets may only decrease | B7, B3 | Sub |
| TS-D07 | Skipped tests are counted on every run | B5 | Sub |
| TS-T01 | Core logic is deterministic. Time, randomness, and IDs... | B4 | Sub |
| TS-T02 | Map iteration order never reaches an output | B4 | Sub |
| TS-T03 | Every parser, decoder, and state machine has a fuzz target | B5 | Sub |
| TS-T04 | No time.Sleep in tests. Use testing/synctest | B4 | Sub |
| TS-T05 | -race on every CI run. Seeds are logged and reproducible | B4, B5 | Sub |
| TS-T06 | Retired, tests may carry comments but need none (ADR-0031) | — | Retired |
| TS-T07 | Error-path branch coverage is measured separately with... | B5, B2 | Sub |
| TS-T08 | Mutation score floor | B5 | Sub |
| TS-T09 | Every parser has a rejection corpus, and every file in... | B5 | Sub |
| TS-T10 | Table-driven tests have named cases | B3 | Sub |
| TS-T11 | The suite runs twice and the output is diffed | B4 | Sub |
| TS-T12 | Tests live in the external _test package | B3 | Sub |

## Distribution

| Benefit | Rules citing it |
| --- | --- |
| B2 Short defect distance | 37 |
| B3 Local reasoning | 36 |
| B7 Change safety | 26 |
| B0 Uniformity | 22 |
| B1 Bounded execution | 17 |
| B4 Determinism | 17 |
| B6 Predictable cost | 16 |
| B5 Defect discovery | 15 |
| B8 Blast radius | 10 |

Of 143 rule IDs, 122 are live: 100 substantive, 13 substantive in a conventional form, and 9 pure
convention. The other 21 are retired (18) or deleted (3). Short defect distance and local reasoning
carry the document between them, which is a fair description of what TigerStyle is once the poetry
is stripped off. B0 is cited by 22 rules, so roughly one rule in six exists for agreement rather
than correctness. Better to know that number than to pretend it is zero.

## The nine coordination-only rules

`TS-N01`, `TS-N10`, `TS-L01`, `TS-L02`, `TS-L03`, `TS-L04`, `TS-L05`, `TS-L06`, `TS-L11`, plus the
formatting halves of the 13 `Mixed` rules.

These are conventions. Their entire value is that everyone follows them, they are enforced by tooling
so nobody has to think about them, and arguing about them is the most expensive thing you can do with
them. If someone wants to change one, the only valid argument is "the new answer is also unanimous",
never "the new answer is better".

Two consequences. Never defend a coordination rule with a safety story, because someone will
eventually notice and then distrust the rules that do have safety stories. And never spend review time
on them, since a tool does it.

## Thresholds are calibrated, not derived

Nine numbers here are calibration points rather than consequences of any argument: 70 lines, 100
columns, cyclomatic 10, cognitive 15, nesting 3, 64 bytes for pointer passing, two assertions per
function, four parameters, and a dupl threshold of 150 tokens.

The distinction that matters: **the existence of each bound is justified** by B1 or B3 or B6, and the
particular number is a starting point from someone else's codebase. Changing a number needs data from
your codebase and is a normal thing to do. Removing a bound needs an argument against the benefit and
is not.

Anyone who says "70 is arbitrary" is correct and has not made an argument. Anyone who says "so there
should be no limit" has made one, it is against B3, and they need to take it up with the working
memory claim.

---

# Part V: Implementation

## The custom analyzers

All are `golang.org/x/tools/go/analysis` passes. Ship them as one binary via `multichecker` for local
use (`tiger check ./...`), and as a golangci-lint module plugin (`.custom-gcl.yml` plus
`golangci-lint custom`) so they run in the same pass as everything else. Releases ship the `tiger`
binary alongside the plugin, because patterns and `tiger changed` need it. The plugin skips the
whole-program rules (`TS-A07`, `TS-A09`), `TS-X03`'s whole-module checks, and the budgets, and
golangci-lint's `//nolint:tiger` and `exclusions` silence tiger's rules under it, so CI runs
`tiger check`.

**Output contract:** the `variant` analyzer prints a synthesized loop variant in pin syntax, the
exact `//tiger:variant` line a developer would paste. Report and pin are one format, so adopting a
pin is a paste, comparing a failed pin to reality is a two-line diff, and `tiger pin` writes the
declaration mechanically.

**Acceptance contract:** there are no effort estimates — implementation is AI-driven, and the
binding constraint is functional acceptance, which every analyzer defines before it merges. Per
rule it enforces: an `analysistest` corpus in which the rule's failure-mode example (from its Why)
fires the diagnostic and the compliant rewrite stays silent — the functional requirement,
executable. For pin-enforcing analyzers, a test asserting report output is byte-identical to pin
syntax. And no silent scope cuts: where coverage is narrower than the rule claims (heuristic,
advisory, partial predicate language), the corpus contains the known miss, marked as such. An
analyzer without its corpus does not merge, no matter how plausible its implementation looks.

Status is what exists today: **built**, **built; changes decided** (the analyzer exists and an ADR
changes what it checks), or **not built**.

| Analyzer | Rules | Approach | Status |
| --- | --- | --- | --- |
| `contracts` | TS-V03 | Abstract interpretation over nil-ness, integer ranges, length relations, invariant IDs | built |
| `variant` | TS-V01, V02 | AST and `go/types`; synthesizes variants for fixed steps, constant division and shifts, slicing by a part shown non-empty, struct fields no call receives, and `&&` conditions; matches variables by identity; verifies pinned ones at loop head and back edges; its verified loops are the result `boundedloop` reads | built (ADR-0013) |
| `surfaces` | TS-I01, I03, I05, I07 | Not planned. TS-I01's direct half is `TS-T01`'s bans, the rest is review with `tiger changed`; I03, I05, I07 are review | not built |
| `canonical` | TS-K01, K02, K04, K05 | AST, with auto-fix | not built |
| `surfacediff` | TS-R02 | Serialize the exported surface, diff against a checked-in file | not built |
| `invariantrefs` | TS-A07 | Group `assert.Invariant` calls by constant, count referencing functions | built |
| `invariantsymmetry` | TS-A08 | Pair functions by naming convention, compare invariant sets | not built (ENG-182) |
| `invariantnegative` | TS-A09 | Require `assert.Violates` per declared invariant | built |
| `norecursion` | TS-S01 | `buildssa`, static call graph (`ssalib.StaticCalls`), report any strongly connected component that is a cycle | built |
| `boundedloop` | TS-S02, S03, S23 | AST, classify every loop; unclassifiable loops are findings and no directive waives TS-S02, `//tiger:batched` included (ADR-0014). Checks counter soundness and cap reporting, accepts a loop whose variant `variant` verified, and treats a range over a non-standard-library iterator as a finding | built (ADR-0013, ADR-0014) |
| `limitflow` | TS-S02 (limit) | `go/types` and cross-package facts; traces each loop limit through parameters, fields, locals and function results back to where it enters the program; a value must be a constant, a length, another bounded limit, or clamped against a declared maximum | built (ADR-0013) |
| `compoundcond` | TS-S06, S07, S08 | AST, count binary logical operators per condition | built |
| `assertdensity` | TS-A01, A02 | AST plus `types`, count assert calls per function and package | not built |
| `quantitycast` | TS-Q01, Q03 | `types`, forbid direct conversions outside the quantity package | not built |
| `domaintypes` | TS-Q02 | `types`, exported signatures may not take bare primitives | not built |
| `sametypeparams` | TS-N07, N08 | `types`, adjacent identical types, bool params, arity; a leading `t` or `ctx` is not counted | built |
| `participle` | TS-N14 | Suffix check plus allowlist | built |
| `unitsuffix` | TS-N03 | AST plus token dictionary and a position rule | not built |
| `noabbrev` | TS-N02 | AST plus dictionary; the dictionary is the work | not built |
| `escapecheck` | TS-C11 | AST, returns of `&x.field`, `x.slice`, `x.map` from exported methods | not built |
| `paradigm` | TS-C13 | `types`, mutex fields versus channel fields per package | not built |
| `chandecl` | TS-C12 | AST, channel type declarations per package | built |
| `selectctx` | TS-C05 | AST, every blocking select needs a `default` or a stop case: `ctx.Done()` on a context that is not `Background` or `TODO`, or a channel the package closes or sends on | built |
| `nogoroutine` | TS-C02, C03, C09 | AST, only `errgroup.Group.Go` and `sync.WaitGroup.Go` start goroutines, with `Wait` reached; a goleak harness in `TestMain` where goroutines start; `_test.go` files skipped for TS-C09 | built |
| `queuebound` | TS-C04 | AST, slice-backed queues with unbounded append | not built |
| `ioinloop` | TS-M10 | `types`, IO calls inside loop bodies | built |
| `poolzero` | TS-M05 | `buildssa`, the `sync.Pool.Put` argument must be zeroed with `*b = T{}` | built |
| `outptr` | TS-M04 | `types`, constructors returning oversized value types | not built |
| `limitrelate` | TS-S21 | AST, every Max/Min constant appears in a relational assertion | built |
| `passthrough` | TS-X02 | AST, body is a single forwarding call | not built |
| `layers` | TS-X03, D01 | AST, each non-test import judged exactly against the package's `tiger.layers.yaml` entry; stale entries and packages the run did not load are checked by `tiger check` over the whole module | built (ADR-0029) |
| `adapters` | TS-I06 | Deleted with TS-I06 | deleted |
| `errignore` | TS-E02 | AST plus `types`, `_ =` on error is a finding, except inside `defer` or `t.Cleanup`; no comment clears it | built |
| `returnarity` | TS-E06 | `types`, result count and composition | built |
| `paniccheck` | TS-S18 | AST, panic outside the assert package | built |
| `nogoto` | TS-S09 | AST, `BranchStmt` with Goto or a label | built |
| `nofloat` | TS-N11 | `types`, float types in the packages a `tiger.yaml` entry lists | not built |
| `declorder` | TS-L05 | AST, declaration and method order (TS-L04 not shipped, see TS-L04) | built |
| `deferdistance` | TS-L10 | AST plus token positions | built |
| `directives` | TS-L09 | AST, every escape-hatch directive and `//tiger:openenum` carries a reason; with `nolint.forbid` on, every `//nolint` is a finding | built (ADR-0023, ENG-179) |
| `maporder` | TS-T02 | `buildssa`, allowlist of order-safe loop bodies; collect-then-sort with plain sorts or full-coverage comparators; the `maps` escape closed | built (ADR-0021) |
| `fuzzcoverage` | TS-T03 | `packages`, parsers must have a Fuzz function | not built |
| `corpuscheck` | TS-T09 | Directory presence plus a test that iterates it | not built |
| `tablename` | TS-T10 | AST, table test literals require a name field | built |
| `testpackage` | TS-T12 | AST, every `_test.go` file declares a `_test` package | built |
| `skipcheck` | TS-D07 | AST, every `t.Skip` is counted against the budget | built |

The `effects` and `frames` analyzers exist today and are removed by ADR-0026. The `restrictions`,
`closedworld`, and `singleimpl` analyzers are removed (ADR-0024, ADR-0016). `refactor`, `ownership`, `hotpath`, and `wiretypes` were
never built and are dropped with their rules (ADR-0027, ADR-0028).

Most rows are AST or `types` checks and are cheap. The `buildssa` analyzers and `contracts` carry the
cost, and they are where a false positive is hardest to fix.

## `tiger changed`

`tiger changed <base>` compares the code at a base revision, such as `main`, with the working tree.
For each struct, or each free function, it lists the types and functions it calls now that it did
not call at the base. It replaces effect and frame pins (ADR-0026). It is a report for review, human
or agent, not a rule, so it never fails a run.

- A change is reported on the struct where it was written, not propagated to its callers. Work handed
  over a channel shows on the struct at the receiving end when it makes the new call.
- A call through an interface is named by the interface, never by a guess at the implementation, so
  the report needs no table of library effects.
- Pure helpers are filtered by the import graph, with no list of packages. A method or interface
  counts when its package reaches `syscall`, and a free function counts when its own body does. That
  keeps `os`, `net`, `time`, pgx, and badger, and drops `strings`, `sort`, and `fmt`.
- A new method on a type the struct already calls is not reported. Generated files are skipped, and
  `context.Context` is shown.

Known blind spots, each pinned by a test: a write through `io.Writer` is silent,
because the `io` package never reaches the operating system; moving a call into a separate free
function reads as one edge removed and one added; and a renamed type reads as all its edges removed
and re-added. `tiger changed` ships this report as text, markdown or json (ENG-203).

## Patterns

Tiger has two parts: blocking rules for what it can check exactly, and a collection of patterns for
the parts of a correct shape no rule can check (ADR-0025). A pattern exists only where a rule stops
short and a real bug lived in that gap: a trial finding, a filed ticket, or a bug an experiment
reproduced. Every pattern:

1. is runnable code, a small Go package with the right shape next to the broken shape it replaces,
   and tests that demonstrate both; the broken shape passes `tiger check`, and its test shows the bug
2. passes `tiger check` with zero findings and runs its tests under `-race` in CI
3. names the rules it backs up and the part those rules can't see
4. is named by the messages of those rules
5. gives reviewers a short checklist of yes-or-no questions for the parts tiger can't check

Patterns live under `patterns/<name>/` in tiger's root module. The `tiger` binary embeds them:
`tiger pattern <name>` prints a pattern's card, code, and tests, `tiger pattern --rule <code>` lists
the patterns behind a rule, and `--checklist` prints only the reviewer questions. Each rule's registry
entry names its patterns, and the driver appends ``see `tiger pattern <name>` `` to every finding from
that rule; the message length cap counts it. A meta-test fails if a pointer names a pattern that does
not exist or a pattern names a rule that does not exist.

The seed patterns are the shutdown loop, the goroutine owner, pool reuse, the capped worklist, a page
over an outside stream, clamping a limit where it enters the program, sorted map output when the key
has no order, and the filtered page, plus `patterns/ledger`. All nine are built under `patterns/`; a
rule backed by several patterns points at `tiger pattern --rule <code>` to keep its message under the
length cap.

## `tiger golangci` and `tiger.yaml`

The rules marked auto run in golangci-lint from a config tiger generates. `tiger golangci --init`
writes that config for a project that has none, `tiger golangci --print` writes it to stdout for
merging into an existing one, and `tiger golangci` audits the project's config against it. The audit
compares exactly, so a looser value and a stricter one both fail.

`tiger.yaml` carries the facts a repository tells the analyzers about its world. It is versioned,
and each entry is a value, a reason a reviewer accepted, and an optional list of packages it applies
to, keyed by analyzer and setting. An entry widens what an analyzer detects or corrects its
dictionary; no entry names a function, file, or site to exempt. `nolint.forbid` (`TS-L09`) and the
`nofloat` package scope (`TS-N11`) are entries.

Tiger keeps no baseline of existing findings and does not check only the lines a branch changed
(ADR-0022). `tiger check` checks the whole tree, and a codebase adopts tiger by fixing what it
reports. golangci-lint's own new-code filters are the adopter's choice, and tiger does not recommend
them.

## What does not port from Zig

| Original rule | Why it does not port | What replaces it |
| --- | --- | --- |
| `snake_case` for functions, variables, files | Go encodes export visibility in the first letter and the ecosystem assumes `MixedCaps`. Fighting it breaks `go doc`, struct tags, and every linter | `TS-N01`, with descriptive naming kept via `TS-N02` |
| 4 spaces of indentation | `gofmt` uses tabs and will not be configured. The argument was visual distance, and a tab renders at whatever width the reader chose | `TS-L01` plus `.editorconfig` |
| Braces on every `if` | Go requires them | nothing needed |
| Single-line `if (a) assert(b)` | `gofmt` splits composite statements, so the form is not expressible | `assert.Implies(a, b, msg)` |
| Explicitly-sized types everywhere | `len`, `cap`, and slice indices are `int` by language definition. Banning `int` means casting at every index, adding the bugs you were avoiding | `int` in memory, fixed-size types mandatory across wire, disk, and FFI, plus `TS-S19` |
| All memory statically allocated at startup | Go has a GC and no allocator control. Not expressible, and pretending otherwise produces cargo cult | `TS-M01`, `TS-C04`. Same goal, different mechanism |
| Pass arguments over 16 bytes as `*const` | Go has no `const` qualifier and no immutable reference | `TS-M03` for copy cost only. The immutability guarantee is lost |
| In-place init for pointer stability | Go's GC does not move heap objects and escape analysis handles most of it | `TS-M04`, narrowed to large structs and mutex-containing types |
| Explicitly pass library options | Go has no default arguments. The equivalent failure is the zero value you did not think about | `TS-S15` |
| Compile-time assertions via `comptime` | Go has no compile-time execution. You get constant overflow and interface satisfaction checks | `TS-S17`, which covers sizes and constant relationships but not arbitrary predicates |
| `@divExact`, `@divFloor`, `div_ceil` | No such builtins, and `/` truncates toward zero, so `-7/2` is `-3`. Signed division is a sharper trap in Go | A `mathx` package with `DivExact`, `DivFloor`, `DivCeil`, plus an analyzer banning bare `/` on signed integers |
| Functions run to completion without suspending | Go has no async coloring. The scheduler preempts any goroutine at any instruction, so "does not suspend" is not a property a function can have | `TS-C01`, `TS-C11`. The reachable version is ownership, not suspension |
| Callbacks go last | Go's conventions are fixed: `ctx` first, `error` last | `TS-C07` |
| Follow the Zig style guide | Different language | Effective Go, Go Code Review Comments, Google Go Style Guide, in that order |
| Write scripts in Zig | Substrate | `TS-D03` |

## What no amount of tooling fixes

Six items, in a codebase where every rule above is enforced. They are not shrinking further, and they
are all the same kind of thing.

| What | Why no machine will do it | Where it lives |
| --- | --- | --- |
| Is this the right invariant | The analyzer proves the code satisfies `inv.SequenceMonotonic`. Whether monotonic sequence numbers are what the protocol needs is a claim about the protocol | The `inv` package |
| Are the limit's inputs true | `TS-S22` asks that `BatchMax` be written from `SectorSize`. Whether the disk has 512-byte sectors is a fact about the world | The constants file |
| Is the contract right | `TS-V03` proves callers establish the precondition. Whether the header should be that size is design | Contract annotations |
| Is the fault model complete | `TS-I05` checks every declared fault is expressible and tested. Whether real disks fail in ways you did not declare cannot be answered from inside the model | Surface interfaces |
| Should this struct make this call | `tiger changed` shows the struct started calling the database. Whether it should is design | Review of the `tiger changed` report |
| Is this the right product | No comment | Not a code review question, and never was |

All six are instances of one thing.

> A machine can prove code satisfies a specification. Nothing can prove the specification is the right
> one, because rightness is a relation between the specification and the world, and the code contains
> no representation of the world.

This is not a limitation to engineer around. It is the correct final resting place for human
attention, and it is where a reviewer's time was always worth the most and least often spent.

## The honest limits

**The specification gap is irreducible.** Stated above. Everything else here is engineering.

**Goodhart applies with force.** Once conformance is the merge criterion, conformance is what gets
optimized. The defence is the proxy test from Part I plus one genuine external oracle, and `TS-T08` is
that oracle.

**False positives are the real failure mode.** An analyzer wrong two percent of the time on a blocking
rule, across a thousand merges, is twenty forced directives, and directives are how a rule set degrades
into a style guide with extra steps. `TS-D06` tracks the count; treat every one as a bug report
against the analyzer rather than a resolved issue.

**Cost is exponential in the tail.** Off-the-shelf linters reach roughly 80 percent mechanization in an
afternoon. The promoted rules reach 96 percent for about 40 analyzers. The last few percent costs an
abstract interpreter and a loop-limit trace across packages. Tiger declined an effect system, a
points-to analysis, and a refactoring prover (ADR-0026, ADR-0027). Whether the
tail is worth it depends entirely on how expensive your defects are, which is why this is a good idea
for a database and a bad idea for most software.

**You are hiring for a language nobody knows.** Onboarding is longer and some good engineers will not
want to work this way. Adopting by chain is the mitigation and it is a partial one.

## Adoption

The chains only pay when complete, so build them in dependency order rather than by ease.

**Stage 0, the free stuff.** Every `auto` rule with no false positives: `gofumpt`, `godot`, `errcheck`,
`errorlint`, `nakedret`, `gochecknoinits`, `decorder`, `lll`. Mechanical, mostly auto-fixed, one
afternoon.

**Stage 1, the assert package.** Add it, then apply `TS-A01` and `TS-A02`. Density on legacy code is
a project; tiger keeps no baseline that would let the project wait (ADR-0022).

**Stage 2, structural rules.** `funlen`, `cyclop`, `nestif`, `gochecknoglobals`, `depguard`. Real
refactoring, so run advisory and ratchet per package.

**Chain 1, ownership.** `TS-S11`, `TS-S12`, `TS-C11`, `TS-C13`. Completing this makes "which
functions could have written this value" a short list a reviewer can name. Highest value, and the
prerequisite for everything else.

**Chain 2, surfaces.** `TS-I01` through `TS-I07`. Independent of chain 1 and the thing that makes the
test suite worth trusting. Do it early; `TS-I05` in particular is worth more than any three analyzers.

**Chain 3, layering.** `TS-D01`, then the `TS-X03` layer file. A package that does IO almost always
imports the package that does it, so once each package has an entry, most IO stays behind
interfaces passed in.
`tiger changed` reports the calls the layer file cannot see.

**Chain 4, termination.** `TS-S01`, `TS-S02`, `TS-V01`, `TS-V02`. Cheap, self-contained, makes
worst-case behaviour a stated property.

**Chain 5, contracts.** `TS-A07` through `TS-A09`, then `TS-V03`. Most expensive and last, because it
depends on the invariant vocabulary being mature. Do not start here.

**Chain 6, canonical form.** `TS-K01` through `TS-K05`. Cheap and auto-fixable, can run in parallel,
and pays off in review: a diff in canonical form is semantic by construction.

## Diagnostics

A diagnostic is one line with two halves:

```
<path>:<line>:<col>: TS-XXX: <body>
```

The prefix `path:line:col: TS-XXX:` is a parsing contract. The position is rendered by the CLI
from `token.Position`; the rule code is the first thing each analyzer writes into its message.
Structured output and the pull-request summarizer key on it, and no wording change touches it.

`TS-D06` lines are produced by the `tiger` driver, not by an analyzer, and their position names
the budget file: `tiger.budget.yaml:<line>:` when the row exists, where `<line>` is the row's
line in the file, and `tiger.budget.yaml:` alone when the package has no row. The counted
findings that overran print as ordinary lines under the `TS-D06` line, with their own positions
and no marker; under budget they do not print at all. `TS-D06` lines print after every other
blocking finding.

Two `TS-X03` lines are also produced by the driver, from the module's directories rather than a
loaded package, and they keep the full prefix and sort by position with the analyzers' findings.
An entry that matches no package directory sits at the entry's key,
`tiger.layers.yaml:<line>:1:`. A package with no entry that the analyzer did not report sits at
the package's first non-test Go file, `<dir>/<file>.go:1:1:`.

The body is prose for a reader who has never seen a `//tiger:` directive. It has three parts in
a fixed order, separated by ` — `:

1. **What the code does.** Concrete and local: "this select waits on `jobs` and `results`".
2. **What tiger expected.** The gap in plain words: "but nothing in it ends the wait on shutdown".
3. **The edit.** A change the reader can make without opening this document: "add
   `case <-ctx.Done(): return ctx.Err()`".

Parts 1 and 2 may merge when one clause carries both ("goto jumps to a label" already says what
tiger expected). Part 3 is never omitted from a blocking finding. The `--show-facts` channel
prints facts, not findings: what tiger found, "nothing to fix", and the directive that would
freeze the fact, so the line ends in text that can be pasted above the loop.

Before:

```
TS-V01: variant len(data) not monotone: ranking not strictly decreasing on back edge — pin a valid variant
```

After:

```
TS-V01: //tiger:variant len(data) doesn't provably shrink on every pass (len(data) has no step tiger recognizes) — add a cap (for tries := 0; tries < max; tries++) that fails when hit, or name an expression that does shrink
```

The checklist an author runs before committing a message:

1. Does the body open with what the code does, in words a Go programmer with no tiger context
   understands?
2. Does it name the gap without a banned word? The list: pin, pinned, computed, fact, facts,
   effect set, frame condition, lattice, closed set, back edge, dominating, synthesized, ranking,
   allowlist. Matched case-insensitively at word boundaries. Directive verbs (`variant`,
   `requires`, `ensures`, `openenum`, `batched`) appear only as the literal directive spelling. *(Decided and not built: the meta-test's verb list in `internal/rules/meta_test.go` still names
   the retired `effects`, `frame` and `restrict` and lacks `openenum`.)*
3. Does it end with an edit the reader can make now? A comment to add, a call to remove, a name
   to change, a snippet to paste. Never a restatement of the rule.
4. Is every directive spelled `//tiger:<verb> <args>`?
5. Is there exactly one `TS-` code, in the prefix? Where a message once pointed at a sibling
   rule, it names the shape in words instead.
6. One line, under 160 characters of body if you can, under 240 always? A code snippet stays
   only when the edit is the snippet.
7. If a why clause is present, would the edit look arbitrary without it? Otherwise the reasoning
   lives here and in the reference documentation, not on the finding line.

Items 2, 4, 5, and 6 are enforced mechanically: `internal/rules/meta_test.go` runs every rule's
corpus and fails on any emitted message that breaks them, naming the analyzer, the message, and
the invariant, and `internal/golangci` applies the same checks to every `tiger golangci` audit
line over a fixture config. The banned list in that test and the list above change together.
Items 1, 3, and 7 are checked by a blind reader: someone given the message text alone writes
what the code did and the edit, and both must match the author's intent.

## Adding a rule

Fill this in. A blank line means the rule is not ready, and which line is blank tells you what kind of
bad rule you were about to write.

```
Rule.        One sentence, imperative.
Benefit.     Which of B0 through B8, by ID.
Chain.       Rule, to mechanism, to benefit, to goal. No step assumed.
Failure.     The bug it prevents, concrete enough to write a test for.
Cost.        What following it costs. "Nothing" is not an answer.
Falsifier.   What observation would retire this rule.
Enforcement. Tool and level, or the honest admission that it is review.
Threshold.   If it contains a number, is that number derived or calibrated.
Proxy test.  If it is a proxy, does satisfying it require the same thinking?
Pattern.     If the rule stops short of a correct shape and a real bug lived in the gap, the
             pattern that covers it (see "Patterns").
```

## Retiring a rule

A rule leaves the same way it arrived, on evidence.

Retire it when its falsifier fires. Retire it when its analyzer has produced no true positive in a
quarter, because a rule nobody violates is a rule nobody needs enforced, and a rule producing only
false positives is teaching the team to ignore the tooling. Retire it when the language changes
underneath it, which happens more than a style guide expects: several rules here exist because of Go's
loop variable semantics before 1.22, its lack of generics before 1.18, or its lack of a fake clock
before `testing/synctest`. The correct response to a language improvement is a smaller specification.

Keep the rule count in review. A document that only grows is a document that has stopped being read.

`TS-F06` and `TS-I02` are the worked examples. Both were deleted for failing the five-question test —
one as another rule plus a review judgment, one as another rule's existence precondition under a
second ID — and their IDs are retired rather than reused, so that a reference to either in an old
commit still resolves.

Eighteen more were retired, each recorded in an ADR, and none of the IDs is reused:

| IDs | Why | ADR |
| --- | --- | --- |
| `TS-P01`, `TS-P02`, `TS-P03`, `TS-K03` | Package-level restriction declarations and closed dispatch are removed; imports stay with `TS-D01` and `TS-X03`, reflection with `TS-S12` | ADR-0024 |
| `TS-F01`, `TS-F02`, `TS-F07` | Effect and frame pins missed every call through an interface or a library; `tiger changed` replaces them | ADR-0026 |
| `TS-F03`, `TS-F04`, `TS-F05`, `TS-F08` | They had no enforcement but the removed `effects` analyzer; `TS-A05` and `TS-C06` are review, and `TS-C02` decides goroutine ownership at the start site | ADR-0026, ADR-0020 |
| `TS-M02`, `TS-M06`, `TS-M08`, `TS-M11`, `TS-K06`, `TS-C10` | They applied only to code marked `//tiger:hot`, `//tiger:wire`, or `//tiger:owner`, and those verbs are removed | ADR-0028 |
| `TS-R01` | Proving a refactor preserves behavior rested on computed frames | ADR-0027 |

`TS-X01` keeps its ID as a review maxim; its single-implementation check is removed (ADR-0016). The
built analyzers behind retired rules are removed in their build tickets; the `restrictions`,
`closedworld`, and `singleimpl` analyzers and the `restrict`, `hot`, `wire`, and `owner` verbs are gone.
