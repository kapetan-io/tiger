# 32. Retire TS-L08 and TS-L13

Date: 2026-11-02

## Status

Accepted

## Context

The Specification listed two rules about where a change's reason is written down. Nothing checks
either.

- TS-L08 ("Always say why") asks authors to explain decisions in comments. No tool can check that a
  comment gives a reason, and [ADR-0031](0031-comments-belong-in-tests-not-code.md) bans those
  comments in production code.
- TS-L13 ("Commit bodies contain a `Why:` section") could be checked from the commit message. No
  analyzer, registry entry or CI step ever did.

Adopters reading the Specification believed tiger held them to both. ADR-0031 marked both as no
longer applying and left their removal to a separate ticket.

## Decision

Both rules are removed under [ADR-0012](0012-a-rule-either-blocks-or-does-not-exist.md) and
[ADR-0006](0006-a-failed-advisory-trial-removes-the-rule.md), from the Specification's "Layout and
comments" section, its traceability table and every reference to them. Their IDs are listed under
"Retiring a rule" and are never reused.

The `fmt.Print` ban in `config/golangci.yml` cited TS-L08. No rule covers structured logging, so the
ban stays and its message drops the ID.

## Consequences

- No analyzer, registry entry or CI step changes, because neither rule had one.
- A commit body with no `Why:` section is not reported, as before.
- Commit messages that explain why remain good practice, and review is where that is judged.
