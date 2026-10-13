# 15. No language model in the verdict

Date: 2026-10-12

## Status

Accepted

## Context

ENG-191 asked whether some of tiger's goals need review by a language model
(`docs/features/ENG-191-due-diligence-validate-tigers-approach/decision.md`, section 1 and
call 1). The only place a model could plug into `tiger check` is the free-text reason a developer
writes on a waiver, such as:

```go
//tiger:batched rows arrive from a Postgres cursor; the table size is the bound
for rows.Next() {
```

Tiger never interprets that text. ADR-0003 already records why: a reason can be shape-checked
(the verb is known, the reason is present) but never truth-checked. Tiger checks that the text
after `batched` is not empty and that the loop has a cursor's shape. When both hold, the waiver
applies and counts against the package budget on every run. It prints only when the package
goes over its budget. The same input
gives the same result on every run.

That leaves false reasons unchecked. `experiments/annotations` puts the same `//tiger:batched`
reason on a cursor that returns 50 rows and on one that never runs out, and tiger reports the same
thing for both. A model reading the reason is the obvious proposal for closing that gap.

A model inside the pass/fail verdict makes the verdict nondeterministic. The same commit could pass
on one run and fail on the next. An agent could also rephrase a reason until the model agrees. A
green run is the one result an agent cannot argue with today, and a model in the verdict would make
it negotiable.

## Decision

Tiger stays deterministic. No language model takes part in `tiger check` or in any other pass/fail
result tiger produces.

A waiver reason stays a claim that pull request review reads, outside the verdict. That reviewer
can be a person or a model.

## Consequences

- The same commit gives the same verdict on every run, so a failing run is always a fact about the
  code and never about a model's mood.
- False waiver reasons are not caught by tiger. Each waiver stays in the code, where review reads
  it, and counts against its package's budget, so adding one is a visible change.
- A model reviewing the pull request gets every benefit a model in the verdict would, without
  making the verdict negotiable.
- The pressure to make tiger read reasons is relieved by having fewer of them. ADR-0014 removes the
  loop-bound waiver that `//tiger:batched` carried, replacing a reason tiger could not check with a
  counter it can.
