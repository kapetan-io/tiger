# 30. tiger changed reports every added and removed call, by function

Date: 2026-10-15

## Status

Accepted

Amends [ADR-0026](0026-tiger-changed-replaces-effect-and-frame-pins.md).

## Context

ADR-0026 defined `tiger changed` as the types and functions each struct or free function calls at
head and did not call at base. It did not report a new method on a type the struct already called,
and it applied the syscall-reach filter to every callee.

That rule was run on 6 commits: 5 from querator, a queue service, and 1 from git-server. On querator
it printed nothing for 3 of the 5 commits, although each changed behavior. Only 1 of 24 changes
inside an existing caller-to-callee pair was a new method on a type already called, so the case the
exception was built for was rare. Removed calls are design changes too, and an additions-only rule
cannot show them.

Four more forces shaped the output.

- The primary reader is a person reading a PR comment who has never seen tiger. An empty report must
  not read as proof that nothing changed.
- Recording every call inside a package was about 85% noise (140 folded lines). Keeping only calls
  to another owner whose own body calls another package added 4 main-list lines, 3 of them signal.
- Module packages are the architecture a reviewer checks. The syscall-reach filter is meant to drop
  pure helpers in outside libraries. Applied to module packages, it hides the calls that show which
  module packages now depend on each other.
- On a pull request, CI's default checkout is a synthetic merge commit that appears nowhere in the
  PR, so a SHA from it identifies nothing a reviewer can find.

## Decision

`tiger changed` reports, for each function, every call added and every call removed between the
merge base of `<base>` and HEAD and the working tree.

- A call is one (function, callee) pair. A new call on a target the function already called is
  reported, which removes ADR-0026's exception for new methods on types already called.
- A callee is a function, type or interface in another package, or a function of a different owner
  in the same package whose own body calls another package. Calls between methods of one type and
  calls through an interface the package declares are not reported.
- The syscall-reach filter applies only to packages outside the module. Every module package counts.
- The unit a change is reported under is the function (`pkg.Type.method` or `pkg.function`), not the
  struct. The owner is still where the call is written.
- A brand-new package prints as a summary of the packages it uses. A removed package prints as one
  line. An owner replaced by one with identical callees prints as one `Renamed:` line, so ADR-0026's
  "a renamed type reads as all its edges removed and re-added" holds only when the renamed type's
  calls changed.
- A package that does not load at a revision is left out of both sides and named in the report.
- Head is named by its commit, never by a branch, and CI checks out the pull request's head commit.
- Every report ends with what was compared and what tiger cannot see.
- Report lines follow ADR-0009's plain-language and banned-word rules. Its diagnostic prefix, body
  structure and length cap do not apply, because the report is not a diagnostic.
- `tiger changed` filters no `tiger check` findings and does not replace it. CI keeps running
  `tiger check` on the whole tree (ADR-0022).
- The output format is `format.md` on ENG-203, as amended by the blueprint "tiger changed output
  format".

## Consequences

- The report is longer. The 6 measured commits printed at most 11 main-list lines each, and a
  high-churn repository may print long lists. The PR comment is capped below GitHub's comment limit;
  the terminal output is not capped.
- A renamed type whose calls also changed still reads as removed plus added, and a call moved into a
  new helper reads as one removed and one added.
- A package that fails to load, often generated code that git leaves out of the base revision, is
  invisible to the report, though the footer names it.
- The output is a byte-for-byte contract, so changing any wording is a breaking change for agents
  that parse it.
