# 33. Remove the participle naming rule

Date: 2026-11-22

## Status

Accepted

## Context

TS-N14, a blocking rule in this tool, rejects any exported name whose last word ends in "ing" (a
present participle). It rests on the maxim that a name should
be a thing, not an activity: `Pipeline` composes into `pipelineMax`, and `Preparing` does not.

A suffix cannot tell an activity from a state or an ordinary noun. In trials on two real Go codebases,
nearly every finding was a state or a noun, such as `Draining`, a state the codebase's own glossary
defines, and `Keyring`. The rule's only remedy, renaming to a noun, breaks a name the domain already uses, and its
only escape is a growing list of exempt words.

## Decision

We will remove TS-N14 and never reuse its ID. TS-N09, "Prefer nouns to participles", stays as a maxim
enforced by review.

## Consequences

- No agent is pushed to rename a domain state, and no project keeps a config entry to silence the rule.
- A name that is a true activity is no longer reported by the tool, so only review can catch it.
- A future rule that judges a name by its suffix has the same blind spot.
