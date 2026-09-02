# Tiger, explained

> Tiger defines a restricted, deterministic dialect of Go and checks that your code
> stays inside it. You run one command and get a verdict. Either the code conforms,
> or you get a list of the exact edits that make it conform. No warnings, no judgment
> calls, and no LLM anywhere in the loop. The same code gets the same verdict, every
> run, on every machine.

`tiger check` · binary verdict · change detection

## Your agent writes faster than you can read

Coding agents produce diffs faster than we can read them. Nobody reviews
their way out of that. We built tiger to hold AI-written code to a strict set of
proven rules, rules that make the code deterministic and remove whole classes of bugs
and security issues before a human ever looks at the diff.

Tiger defines a restricted dialect of Go and enforces it as a CI gate. The
restrictions come from NASA's Power of Ten, the rules JPL uses for flight software
(code that has to work the first time, because nobody pushes a patch to a spacecraft),
and from TigerBeetle's TigerStyle, which tiger is named for. Each rule bans a
construct that produces the bugs humans are bad at spotting. A loop with no provable
bound is a denial of service waiting on hostile input. An invariant that lives in a
comment instead of an assertion is a promise nobody checks. A goroutine with no owner
outlives shutdown and leaks. Tiger enforces each rule with its own analyzer, so
conformance is a verdict, not an opinion.

This matters now because agents write more and more of the code. An agent is fast,
tireless, and perfectly happy to hand you a loop with no exit, or a `time.Now()`
buried deep in your business logic, where every run reads a different clock and a
failure you saw once can never be reproduced in a test. A human reviewer scanning an
agent-sized diff can't hold the implementation in their head, so the rules have to
govern what an implementation can and cannot do, not just how it looks.

With tiger the implementation conforms, or `tiger check` fails the build. And the
agent can't talk its way past the failure. There is no `//nolint` equivalent to
silence a finding, and no warnings to scroll past, a finding either blocks the merge
or tiger doesn't report it. The cheapest way for an agent to get the build passing
again is to write the compliant code, so that's what it does.

Tiger's second job is change detection. While it checks, tiger computes each
function's **effects**, the verbs it performs anywhere beneath it (allocate, do IO,
block, read the clock, spawn a goroutine). Freeze a fact like that into a comment
called a pin, and a change that silently breaks the promise fails the build. An agent
can't drift a pinned fact past you.

### What you get in return

- **Deterministic code.** Every source of nondeterminism enters your program through
  a surface you declare; even map iteration order can't leak into an output. A
  failure you can reproduce on demand is a bug, one you can't is a rumour, and a bug
  found once stays found.
- **A deterministic gate.** The check runs no LLM and touches no network, and two
  runs on the same code produce byte-identical output (our own CI runs the check
  twice and diffs the bytes). A gate for a nondeterministic code generator has to be
  like this. A flaky reviewer can be re-rolled until it passes; tiger can't be.
- **Detection moved up the ladder.** A compile error beats an assertion, an assertion
  beats a test, a test beats production. Limit constants carry compile-time guards
  that break when a number changes, and a declared effect set catches a `time.Now()`
  three call frames down, where a package-level import ban sees nothing.
- **Whole-program properties, not local ones.** An effect is transitive by
  construction, so it can't be hidden behind a wrapper function. Pin a function as
  doing no IO and you've made a claim about everything beneath it, checked at every
  call edge, across package boundaries.
- **One gate for generated and hand-written code.** Trust attaches to what the code
  provably satisfies, not to who or what produced it. That matters more every month,
  as more of the code is machine-written.

## What it caught in our own code

We trialed tiger on two real codebases before the rules settled, a distributed queue
of about 42,000 lines of hand-written Go, and a git server of about 17,000. The rules
found ten real bugs between them.

The worst was a remotely triggerable denial of service in the git server. A client
could push two tags whose headers point at each other, then ask the server to resolve
one. The resolving loop had no depth cap and no cycle check, so a single request
would spin a goroutine at 100% CPU forever. The loop-bound rule (`TS-S02`) flagged it
as an unbounded loop, which is exactly what it was.

| Rule | Real bugs found |
| --- | --- |
| `TS-S02` every loop has an upper bound | 5, including the DoS and a pause/shutdown deadlock |
| `TS-S08` a switch over a fixed set handles every case | 3, all silently dropping data on unexpected values |
| `TS-C05` blocking waits are cancellable | 1, client waits that could never be cancelled |
| `TS-C02` goroutines start through a supervisor | 1, shutdown returning before its goroutines exited |
| `TS-E02` discarding a result needs a justification | 1, a discarded validation error set to corrupt IDs later |

Every one of these bugs had already passed the race detector, the test suite, and
human review. These aren't bugs a sharper reviewer
catches on a better day; they survive good reviewers because nobody re-derives "does
this loop terminate on hostile input?" for every loop in every pull request. A
machine re-derives it on every commit and never gets bored.

When tiger flagged something wrongly in the trials, we treated the false positive as
a bug in tiger and fixed it in the analyzer with a regression test. Zero suppressions
in the target code, across both trials. Hold us to that when you run it on yours.

## Tiger is not a linter

Tiger and golangci-lint do different jobs, and a Tiger Go project runs both. A linter
looks for likely problems and prints warnings. A warning asks a human to decide, and
a human under deadline decides to scroll past; that's how a long-lived codebase ends
up with a warning count nobody reads anymore. It's not a failure of any particular
linter, it's the nature of advisory output.

Tiger's output is never advisory. It checks whether your code is inside the dialect,
and what it flags are the patterns that turn into subtle bugs, the unbounded loop,
the switch that silently drops a case, the wait that can't be cancelled. Every
finding is a verdict. The build fails, and the finding names the edit that fixes it.
If tiger is wrong, that's a bug in tiger; you file it, the fix lands in the analyzer
with a regression test, and nobody writes a suppression comment.

golangci-lint keeps doing what it does today, catching likely bugs and style
issues, and tiger configures it for you (`tiger golangci --init` generates the
config for the rules existing linters already enforce well, so tiger doesn't
reimplement them). Tiger enforces the dialect.

We chose every restriction in the dialect for decidability, not taste. Each one
exists to make some analysis possible. A bounded loop makes termination checkable, a
supervised goroutine makes shutdown traceable, and a package that declares a
restriction like `no-reflect` (no reflection anywhere in the package) hands the
analyzers a smaller world to reason about. No single restriction is worth much on
its own, and completing a chain of them turns an analysis that's intractable on
ordinary Go into a cheap one tiger runs on every commit.

**What you touch.** One command in CI and locally. Four subcommands total (`check`,
`budget`, `golangci`, `pin`). Two committed YAML files. A handful of `//tiger:`
comments in your source.

**Inside (not your concern).** Thirty-plus analyzers walking your code's structure,
computing what each function does, what it writes, and why each loop ends, then
comparing all of it against the rules and your declarations.

## A day in Tiger Go

The whole workflow is four moves. Write code, read the verdict, fix or declare, and
pin what matters.

You write code → `tiger check` → you fix the findings → you pin what matters

### 1. Run the check

Say you write this. It compiles, it's idiomatic Go, and reviewers wave it through
every day.

```go
// Drain consumes entries until none remain.
func Drain(entries <-chan uint32) uint32 {
    var total uint32
    for entry := range entries {
        total += entry
    }
    return total
}
```

```
$ tiger check ./...
drain.go:6:2: TS-S02: this loop ranges over a channel, so it ends only when some other goroutine closes the channel — add a counter cap that fails when the cap is hit, or make it an event loop that selects on ctx.Done()
tiger: 1 blocking
```

That's a real transcript. The finding line names the rule (`TS-S02`), says what the
code actually does, and names two concrete edits that fix it. You never need tiger
vocabulary to act on a finding. When you want the reasoning behind a rule, look its
code up in the [rule reference](Tiger%20Rule%20Reference.md).

A clean run prints nothing and exits 0; silence is the good news.

### 2. Pin what matters

While checking, tiger also computes facts about your functions, what effects they
have (allocate, do IO, block, panic, read time or randomness, mutate, spawn), and
what they write through their parameters, which tiger calls the function's frame.
Facts aren't findings. They print only when you ask.

```
$ tiger check --show-facts ./...
header.go:34:1: TS-F01: EncodeHeader's effects are alloc — nothing to fix; to make tiger fail the build if they change, add //tiger:effects alloc
```

The fact prints as the exact comment you'd paste. Freezing it is called **pinning**,
and `tiger pin` writes the comment for you.

```
$ tiger pin EncodeHeader
header.go:35: //tiger:effects alloc
header.go:36: //tiger:frame none
```

A pin changes nothing at run time. It changes what CI enforces. Suppose someone (or
some agent) later makes the pinned function block on a channel. The next check fails.

```
drain.go:5:1: TS-F01: this function blocks (channel receive at drain.go:8:2) but its //tiger:effects comment doesn't list block — add block to the comment, or remove that code
```

There are two honest exits. Change the code back, or change the pin. A pin edit is
visible in the diff, so the reviewer rules on the new promise. A pin can never go
quietly stale, because a wrong pin is a failed check, not a lie in a comment.

| Condition | What tiger does | Merge |
| --- | --- | --- |
| No pin; the function's behavior changes | Nothing (the fact just has a new value) | Continues |
| Pin; pin and code agree | Silence | Continues |
| Pin; pin and code disagree | Blocking finding with the reason | **Stops** |
| You edit a pin | Nothing; the edit is visible in the diff | Continues, after review |

## The concepts, in order

### Rules and the two severities

Every rule has a code like `TS-S02` and exactly one of two severities. **Blocking**
findings fail the run. **Advisory** findings are counted, not printed. There's no
warning tier, on purpose. A rule either blocks or it isn't a rule, so nothing tiger
prints can be weighed and set aside.

Advisory exists for exactly two things real codebases carry, an escape directive and
a skipped test. Both are legitimate, both are worth bounding, so tiger counts them
against a per-package budget instead of pretending they're errors.

### The budget and the ratchet

`tiger.budget.yaml` is a committed file with one number per package per advisory
rule, the count that package is allowed to carry. Under budget, nothing prints. Over
budget, the run fails and the counted findings print. `tiger budget --write` records
current counts, and it can only *lower* numbers. Raising one is a hand edit a
reviewer sees in the diff. We call this the ratchet.

### Directives

A directive is a `//tiger:` comment on the line above the thing it describes. The
vocabulary is closed and a typo is a blocking finding, so a directive can't silently
do nothing. There are three kinds.

1. **Pins** (freeze). Freeze a computed fact into a contract. `//tiger:effects`,
   `//tiger:frame`, `//tiger:variant`. Optional, and written for you by `tiger pin`.
2. **Intent declarations** (declare). State something no analyzer can compute, and
   the code is held to the statement. `//tiger:restrict` (this package forgoes a
   language feature), `//tiger:openenum` (this type's value set is expected to grow).
3. **Escape hatches** (counted). Loosen one rule at one site, with a mandatory
   reason. There is exactly one, `//tiger:batched`, and every use is counted against
   the budget on every run.

Why only one escape hatch? Because an agent will use any comment that gets it past
a failing check. An escape is admitted only when reality, not convenience,
demands it. Some external systems accept one item at a time, and no rewrite of your
code changes that, so `//tiger:batched <reason>` exists. A general "ignore this rule
here" directive doesn't exist and won't. The only paths are the compliant rewrite or
a counted, human-reviewed escape.

## How review divides

Once the declarations exist, review splits into two jobs that get different
treatment.

**Machines check the correspondence.** Does the code satisfy the invariant it names,
stay inside its pinned frame, terminate, cover every case, match its declared effect
set, respect its package restrictions. All of it mechanical, all of it on every
commit.

**Humans and AI review the declarations.** Is this the invariant the protocol
actually needs. Should this function be permitted to touch the disk at all. Is this
limit true of the hardware you deploy on. Questions no analyzer can answer, and the
only ones left.

The declarations are a layer, not a region. They cut across every file, so there's no
"reviewed part" of the tree and no boundary to erode. Which checks apply to a
function follows from its computed effect set, not from where someone filed it.

Declarations are a small fraction of your lines and change far less often than the
code around them, so review attention concentrates where being wrong is most
expensive. A change that touches no declaration and passes CI is a candidate to merge
without a human reading it. A change that touches a declaration goes to someone who
knows the domain, and nothing else about it needs discussing, because the mechanical
questions have already been answered.

## What adoption looks like

The queue's first trial run produced 1,023 blocking findings, and the git server's
produced 712. That's what pointing a strict dialect at
good, existing, hand-written Go looks like, and it's why adoption is a decision about
a codebase's future, not an afternoon's chore. On a fresh codebase where the code is
AI-written against the rules from day one, the count starts at zero and there's
nothing to retrofit.

1. **Wire up the auto rules** (one command). `tiger golangci --init` generates the
   golangci-lint config for the rules tiger delegates. `tiger golangci` audits an
   existing config against that baseline.
2. **Run the check, fix what it finds** (the real work). `tiger check ./...`. Real
   bugs first (our trials found ten). For loops an external system forces on you,
   declare `//tiger:batched` with the reason.
3. **Record the budgets** (one command). `tiger budget --write` records each
   package's current advisory counts. Expect the first check to fail until this runs
   once; a package with findings and no budget row has a budget of zero.
4. **Pin as you go** (ongoing). `tiger pin` on the functions whose behavior matters
   most. Sparse pins are fine, a pin covers everything the pinned function calls.

The escape hatch earned its keep in our trials. The queue had 24 loops that drain a
database cursor, a shape only the backing store bounds. Each got `//tiger:batched`
with its reason. The loop-bound rule went from 37 findings to 11, every waiver became
a counted, reviewable line in the budget, and the one real deadlock among those
findings kept firing.

> **Plugin caveats.** Tiger also runs inside golangci-lint as a plugin, but two
> things need the real CLI. Budgets, because the plugin has no end-of-run step to
> count against them, so it reports every counted finding as an issue. And the three
> whole-program rules that need evidence from every package at once (is each
> invariant asserted somewhere, does a test violate it, does an interface have a
> second implementation). Run `tiger check` in CI; use the plugin for editor
> integration.

## What tiger will not do

No warnings. No per-site suppression comments. No "informational" output on a normal
run. When a rule's findings turned out to be noise on real code, we removed the rule
from tiger entirely rather than demote it to a warning; three naming rules died
exactly that way during the trials. A smaller set of rules people actually fix beats
a large set people learn to scroll past.

Tiger can prove the code satisfies the declarations; nothing can prove the
declarations match the world. Whether the invariant is the right one, whether the
limit is true of your hardware, whether this function should be allowed to touch the
disk at all, those questions live outside the code. That's exactly why the
declarations are the part humans still review.

## Try it

The fastest way to form an opinion about tiger is to point it at code you know well.

```
$ go install github.com/kapetan-io/tiger/cmd/tiger@latest
$ tiger check ./...
```

Expect findings. Remember our numbers, 1,023 on the 42,000-line queue we knew and
trusted. A wall of findings isn't tiger telling you your code is bad, it's the
measured distance between idiomatic Go and the dialect. Read ten of them. If most
name a hazard you'd want fixed, work the adoption steps above. If they don't, no tool
should talk you into it.

To see code that already lives inside the dialect, read
[examples/ledger](../examples/ledger) in the repository. When a finding names a rule
you want the reasoning for, the [rule reference](Tiger%20Rule%20Reference.md) has one
entry per rule, with a firing example and the compliant rewrite. And when you want
the full normative detail, the [specification](Tiger%20Specification.md) is the
authority the reference is drawn from.

---

The trial numbers come from tiger's runs on two real codebases before the rules
settled. Every transcript on this page is live `tiger check` output against
`examples/ledger`.
