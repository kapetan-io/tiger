# 34. TS-M10 decides IO from a list of standard library calls

Date: 2026-11-24

## Status

Accepted
Amends ADR 0005

## Related decisions

Amends [ADR-0005](0005-heuristic-rules-enter-advisory-ioinloop-blocks-as-exact.md). For the standard
library, the IO list replaces its allowlist of unconditionally-IO packages. The packages a
repository adds work as ADR-0005 describes. ADR-0005's other decisions stand: TS-M10 blocks, and
every unprovable call is silent.

[ADR-0026](0026-tiger-changed-replaces-effect-and-frame-pins.md) removed an effect table because a
table of library methods never covers the next driver. The IO list is narrower than that table. It
classifies every exported call in a few standard library packages, it decides one rule's verdict on
direct calls only, and third-party clients are added by package, not by call.

## Context

TS-M10 blocks a loop that calls IO once per item. A blocking rule must not fire on compliant code
(ADR-0012), so the classifier must never call a pure call IO.

Deciding by package fails that test. `os`, `net`, `net/http` and `database/sql` each hold both IO and
pure calls. `net.SplitHostPort` parses a string, `os.Getpid` returns a number the process holds,
`sql.Rows.Scan` copies a row already fetched, and `sql.Named` wraps a value. Classified by package,
each one blocks a build when called inside a loop. Exempting them one by one means keeping a list of sites or functions to excuse, and
the config file never names a function to exempt (ADR-0003).

Deciding by reading the callee's body fails in both directions. `http.Client.Do` and `sql.DB.Query`
reach no system call through direct calls, because the work goes through an interface. A call graph
that resolves interfaces marks `fmt.Sprintf` as IO.

The standard library's API changes only when a Go release adds to it, and Go's compatibility promise
means it never removes a call. A list of its calls can be kept complete.

## Decision

For `os`, `net`, `net/http` and `database/sql`, the IO list classifies every exported function and
method as IO, no IO, or unproven. TS-M10 reports a call in a loop body only when the IO list
classifies it IO.

- **IO** means the call touches a file, a socket, a database connection or another process every
  time it runs.
- **No IO** means it computes in memory or returns state the process already holds.
- **Unproven** means the call site cannot tell. An example is an interface method that an in-memory
  value can implement, or a call whose cost depends on the database driver. An unproven call is
  silent. When a call could be IO or unproven, it is unproven.

`syscall` is not classified in full, because its API differs per platform. A short list of its raw
read, write, file and socket calls counts as IO, and every other `syscall` call is silent.

The IO list classifies every exported call in the four packages for the Go release tiger builds
with. A gap fails tiger's own build, never an adopter's, because it is a gap in tiger's list, not in
their code.

A repository widens detection the way ADR-0005 allows, by naming a whole package in
`ioinloop.packages`. Every call into that package counts as IO, except a method on a map, slice or
basic-typed value. Naming a standard library package there replaces the IO list for that package. There
is no list of calls to exempt.

## Consequences

- Pure standard library calls in a loop do not block a build and need no exception.
- Some IO calls are silent: interface methods an in-memory value can implement, driver-dependent
  `database/sql` calls, and most of `syscall`. ADR-0005 accepts these as known misses, documented as
  such in the Rule Reference.
- In the four listed packages, a method on a map, slice or basic-typed receiver is reported when
  the IO list classifies it IO, such as `http.Dir.Open`. The receiver exemption applies only to
  packages a repository names.
- Moving tiger to a new Go release costs a line in the list for each call it adds.
- IO behind a third-party client is found only when a repository names its package.
