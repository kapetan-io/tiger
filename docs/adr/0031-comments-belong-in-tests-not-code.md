# 31. Comments belong in tests, not in code

Date: 2026-10-20

## Status

Accepted

Supersedes the "comment exists" clause of [ADR-0017](0017-a-dropped-error-in-cleanup-needs-no-comment.md),
the sentence of [ADR-0023](0023-banning-nolint-is-an-adopter-opt-in.md) that says TS-E02 accepts a
bare `//nolint`, and the Specification's TS-S22 enforcement line.

Makes one exception to [ADR-0022](0022-tiger-has-no-baseline-and-recommends-no-new-code-filter.md),
described under Decision.

## Context

Hand-written comments in production Go code drift from the code, and nothing catches it. In the
ENG-191 trials one of tiger's ten real bugs (TS-C05) was three client channel waits with no
cancellation, under a doc comment that said they could be cancelled.

Tiger also lets free prose change a verdict in three places.

- TS-E02 accepts any comment on the discard's line or the line above. `// x` and a bare `//nolint`
  both satisfy it, so the 47 cleanup comments ADR-0017 counted documented nothing.
- TS-T06 reports a test function with no doc comment. It fired 42 times across querator and
  git-server and found no bug.
- TS-S22 passes a limit constant when a derivation comment evaluates to its value. TS-S22 fired no
  findings in the trials, and the check read prose.

A history study across tiger, querator and the mono-repo (about 230 comments sampled) found test
comments edited less than production comments (tiger 1.6 against 7.4 edits per 100 comment lines,
querator 14 against 35), with similar stale rates in small samples. Test comments hold reasons that
have no other home, such as "config is process-global; leave nothing installed".

## Decision

A plain comment never changes what tiger's own analyzers report. Only a change to the code, or one
of tiger's own `//tiger:` directives, makes one of their findings go away. The auto rules that
golangci-lint runs still read comments today. TS-L06 (`godot`) and TS-L07 (revive's `exported` and
`package-comments`) are the two that can change with a comment. The `golangci-template` piece of
ENG-408 removes TS-L07 from the template and the auto table.

**Production code carries no prose.** Below the `package` clause of a production Go file, only a
closed set of directives survives: `//go:`, `//tiger:`, `//nolint`, `//line `, `//export ` and
`//extern `. Anything above the `package` clause survives. Test files, generated files and
directories listed in `tiger.yaml` under `comments.allow`, each with a reason, are exempt. Any other
`//word:word` form, such as `//easyjson:json`, is prose. `// nolint` and `///nolint` are banned as
prose in production files, not as nolint, so ADR-0023's opt-in `nolint.forbid` is unchanged. The ban
is on by default, and later tickets build it. It ships no automatic fix, because `tiger pin` is the
only command that writes source (ADR-0008).

**Test files keep comments.** They are allowed and never required. TS-T06 retires. Checking that a
test comment matches its assertions goes on the reviewer checklist, not in tiger (ADR-0015). Three
bugs in the history study met a test comment, and comparing comment to assertion would have caught
one.

**TS-E02 fires on every discarded error outside cleanup.** A discard inside a `defer` call or a
`t.Cleanup` function literal still reports nothing (ADR-0017). The comment exemption is gone.

**TS-S22 retires end to end,** analyzer, corpus, registry entry and specification line, and no
`//tiger:derive` verb is built (ADR-0006). The Specification keeps its maxim as a review rule, write
a limit as an expression of its named inputs. TS-L12 is dropped from the Specification before it was
built. TS-L08 and TS-L13 are marked as no longer applying and a separate ticket removes them.

**`comments.allow` is an exception to ADR-0022.** It is a brownfield adoption path that can grow, and
an agent can clear a finding by adding an entry. Review of the `tiger.yaml` diff is the guard until a
better way to close the gap exists. ADR-0022 stays the goal.

## Consequences

- `_ = f.Close() // reason` reports TS-E02. Code that discarded an error under a comment returns,
  checks or logs it instead.
- A test function with no doc comment exits 0. TS-T06 and TS-S22 are not in the registry.
- The mono-repo's `nocomment` analyzer also strips test comments and so disagrees with this decision.
  Changing it is a separate ticket.
- An instruction such as "Comments explain WHY" produces findings in a repository that runs the
  ban. Changing that instruction is its owner's call.
- A reason after a `//nolint` survives with it. Teams that want none use the `nolint.forbid` opt-in.
