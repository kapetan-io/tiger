# 1. Record architecture decisions

Date: 2026-08-12

## Status

Accepted

## Context

Tiger Go is built from a large external specification, and the build is AI-driven across many
sessions. Decisions made in one session are invisible to the next unless they are recorded in the
repository itself.

## Decision

We will record architecturally significant decisions as numbered Architecture Decision Records in
`docs/adr/`, using the Michael Nygard template.

## Consequences

- Future sessions and reviewers can read why the system is shaped the way it is without access to
  the conversations that shaped it.
- Reversing a decision requires superseding its ADR, making drift visible.
- Writing the record adds a small cost to each significant decision.
