# 26. tiger changed replaces effect and frame pins

Date: 2026-10-12

## Status

Accepted

Amended by [ADR-0030](0030-tiger-changed-reports-every-added-and-removed-call.md): the report lists
every call added and removed by function, and the syscall-reach filter applies only to packages
outside the module.

Amends [ADR-0007](0007-tiger-pin-never-rewrites-an-existing-pin.md) and
[ADR-0008](0008-pin-placement-lives-in-one-writer-not-in-suggested-fixes.md), which now cover
variant pins only. Amends [ADR-0012](0012-a-rule-either-blocks-or-does-not-exist.md), whose
computed facts no longer include effect sets or frames.

## Context

Effect and frame pins exist so a reviewer learns when a pull request changes the architecture, not
only the code. The explainer's example: if the architecture allows database access only through a
`Store` interface and an agent adds a database call somewhere else, the reviewer should see it.
That goal stays.

Pins cannot meet it.

- Tiger drops calls through interfaces and calls into third-party libraries. A probe showed tiger
  computing `func Save(s Storage, e string) error { return s.Write(e) }` as having no effects and
  accepting `//tiger:effects none` on it, while a test passed a `Disk` and a file was written. On
  querator, all 276 functions on the request path, from the HTTP handlers to the Postgres, Mongo
  and Badger stores, compute as doing no IO.
- `fmt.Fprintf` counts as disk IO, so a pure SHA-1 hash fails a pin. Frames miss a write through a
  value receiver's pointer field and a map `delete` through the receiver.
- Fixing effects needs either a table mapping every library's methods to effects, which never
  covers pgx or the next driver, or closed dispatch, which ADR-0024 removed.
- Pinning an interface asks the wrong question: `Save` does disk IO with a `Disk` and none with a
  test double, so only concrete code has one true answer.
- A pin is a comment the agent edits in the same commit as the code.

A deliberation weighed three designs on querator and git-server. Reach pins caught 23 real
behavior changes over 10 querator commits with no table, but pins high in the call stack ran to
about 1,300 characters and the agent still edits them. Effects through the signature caught 4 of
6 IO changes but needed a package-level declaration ADR-0024 removes. Removing pins left the goal
unmet. If the goal is a report of what changed between the base branch and the pull request, tiger
can compute both sides itself and needs no pins.

## Decision

`//tiger:effects` and `//tiger:frame` are removed, with TS-F01, F02 and F07, the effect table, the
`effects` and `frames` analyzers, and the effects and frames half of `tiger pin`.

`tiger changed <base>` replaces them. For each struct, or free function, it lists the types and
functions it calls now that it did not call at the base revision.

- A change is reported on the struct where it was written, not propagated to its callers. Work
  handed over a channel shows on the struct at the receiving end when it makes the new call.
- An interface call is named by the interface, never by a guess at the implementation, so the
  report needs no table.
- Pure helpers are filtered out using the import graph, with no list of packages. A method or
  interface counts when its package reaches `syscall`, and a free function counts when its own body
  does. That keeps `os`, `net`, `time`, pgx and badger and drops `strings`, `sort` and `fmt`.
- A new method on a type the struct already calls is not reported. Generated files are skipped and
  `context.Context` is shown.

The report is for review, human or agent. It is not a rule, so ADR-0012 does not apply to it.
`//tiger:variant` stays. `//tiger:requires` and `//tiger:ensures` are not part of this decision.
ENG-203 settles the output format.

## Consequences

- A reviewer sees which struct started calling something new, and there is no annotation an agent
  can edit to hide it. The `tiger changed` report printed 5
  lines over 10 querator commits, 2 of them real: commit aeb014c, where the daemon starts
  constructing the Postgres stores. On git-server's head commit, the mixed filter and skipping
  generated files bring 173 lines down to 36 hand-written ones.
- Nothing blocks. The report informs review and does not fail a build.
- ADR-0007's insert-only writer and ADR-0008's single placement writer now apply only to variant
  pins.
- Known blind spots, each pinned by a test: a write through `io.Writer` is silent, because the `io`
  package never reaches the OS; moving a call into a separate free function reads as one edge
  removed and one added; a renamed type reads as all its edges removed and re-added.
- The `effects` analyzer was the planned enforcement for TS-F03, F04, F05, F08, A05 and C06, none
  of them built, and for the helper-depth halves of TS-C08 and T01. forbidigo enforces the direct
  half today. The specification gives each rule review as its
  enforcement or removes it.
