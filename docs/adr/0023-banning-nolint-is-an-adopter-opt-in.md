# 23. Banning //nolint is an adopter opt-in

Date: 2026-10-12

## Status

Accepted

Superseded in part by [ADR-0031](0031-comments-belong-in-tests-not-code.md): TS-E02 no longer accepts a
bare `//nolint`, or any comment. The `nolint.forbid` opt-in stands.

## Context

Three sources disagreed about `//nolint`. The
specification allows `//nolint` with a reason, enforced by golangci-lint's `nolintlint` under
TS-L09. The explainer said tiger "flags any `//nolint` as a finding". The code does neither.

Under `tiger check`, `//nolint` is not read at all, so it silences nothing by itself. It does
satisfy a rule that only asks for a comment: `_ = os.Remove(path) //nolint` clears TS-E02, which
`// x` also does. Under the golangci-lint plugin, `//nolint:tiger` silences any tiger rule
on that line with no advisory and no count (ENG-178, item 2). `experiments/nolint` shows two more
routes under the plugin: a file-wide `//nolint:tiger` in the package doc comment drops even a
finding at the `package` line, and an `exclusions` rule in `.golangci.yml` drops findings without
touching the code.

The proposal was to make every `//nolint` on a tiger rule a blocking finding, and to count
`//nolint:<linter>` on the other linters as a waiver. Whether a codebase uses `//nolint` is a
choice about how the adopter's team works, not something tiger can decide for them.

## Decision

Tiger does not ban `//nolint` by default. A `tiger.yaml` entry turns the ban on:

```yaml
nolint:
  forbid: true
  reason: agents must fix findings, not silence them
```

With it on, every `//nolint` in a checked package is a blocking TS-L09 finding, reported at the
file's `package` line, where a `//nolint` on the original line cannot cover it. `tiger check`
enforces it.

`tiger check` never honors `//nolint`, with or without the entry. The counted `//nolint:<linter>`
waiver is not adopted. TS-E02 keeps accepting a bare `//nolint` as its comment, because rejecting
it there would close nothing while `// x` still passes.

## Consequences

- An adopter who wants agents to fix findings rather than silence them turns on one entry, and
  `tiger check` blocks every `//nolint`.
- Under the golangci-lint plugin the ban can be bypassed, by a file-wide `//nolint:tiger` or an
  `exclusions` rule, and the docs say so. CI relies on `tiger check`, which sees through both.
- The explainer drops "tiger flags any `//nolint`", and the specification's TS-L09 line describes
  the opt-in.
- With the entry off, `//nolint:tiger` under the plugin still silences a tiger rule with no trace.
  That is the adopter's choice, and `tiger check` still reports the finding.
