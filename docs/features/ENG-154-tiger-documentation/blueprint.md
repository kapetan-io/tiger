# Tiger End-User Documentation Blueprint

## Objective

Ship the two documents an adopting team reads before and during their first `tiger check`:

1. **`docs/Tiger Explainer.html`** — a newcomer explainer for readers who do not know what a
   linter or an AST is. It leads with what tiger is for and what it caught before teaching any
   mechanism.
2. **`docs/Tiger Rule Reference.md`** — one entry per shipped custom rule and one table row per
   auto rule, carrying the reasoning ADR-0009 deliberately removed from the finding lines.

The framing both documents lead with: tiger defines a Go subset — TigerStyle / NASA
Power-of-10 discipline — so AI-written implementations either conform or produce blocking
violations the AI must fix, and so silent changes to assertions and claims surface to the
developer. Binary verdicts and change detection. Tiger is not a linter; warnings about possible
issues are golangci-lint's job.

## Existing document audit (build upon or scrap)

The ticket requires a per-document decision with rationale. Audited on main at 8d138de:

**`docs/Tiger Go Documentation.html` — scrap; harvest three assets.** Four feature commits
stale: it presents the removed "reported" tier as a first-class severity, defines advisory as
"never affects the exit code" (today an overrun fails the run — ADR-0011), shows an
`[advisory]` trailer in its sample transcript, states TS-A07's superseded two-function
threshold, and never mentions `tiger pin`, `tiger.yaml`, `tiger.budget.yaml`, or the finish
step. Its rule catalogue covers 53 of 142 rule IDs, and a full section is internal status
bookkeeping. Rewriting its spine costs more than starting over. Harvested into the new
explainer: the "A day in Tiger Go" narrative arc (write code → run → read report → pin), the
four-rules-of-the-analyzer table, and the visual system (pills, callouts, dimmed-terminal
blocks). The file is deleted in this PR.

**`docs/Tiger Go Human Explainer.md` — scrap.** It makes no claim a shipped feature could
contradict — no rule IDs, no CLI, no examples — and its register ("alias analysis," "termination
proofs," undefined, in 6KB) is the opposite of the newcomer goal. Two sentences are lifted:
"questions a reviewer normally answers by reading become questions a tool answers by checking"
and "a change touching no declaration and passing CI merges without a human reading it." The
file is deleted in this PR.

**`docs/Tiger Specification.md` — keep untouched; it is the fact source, not the base.** It is
current: two-state severity correctly stated with ADR citations, removed rules absent, the
cross-package rules at shipped wording. The rule reference reprojects its Part III (Why/Enforce
per rule) into user-facing form; the explainer does not derive from it, because simplifying the
spec's thesis produces a fourth copy of the manifesto. Two minor gaps found — `tiger.yaml` is
never named, and an internal count says 145 rules against a 142-row table — go to a follow-up
ticket, not this PR.

## Mental Model

The doc set divides by reader moment:

- **Explainer** — "should we adopt this, and what am I looking at?" Teaching-first, benefits
  before mechanism, assumes no static-analysis background.
- **Rule reference** — "TS-S02 fired on my code; what does it want and why?" One lookup per
  finding. Quotes the shipped diagnostic and adds the reasoning the message dropped (ADR-0009).
- **Specification** — normative depth for rule authors and the curious. Unchanged by this
  ticket; both new documents link into it.
- **Finding lines themselves** — stay terse and newcomer-plain; they are the entry point that
  sends a reader to the reference.

## Correctness Constraints

### State Invariants

- Every custom-rule **category** in `rules.CustomRules()` has exactly one reference entry, and
  every auto rule in `rules.AutoRules()` has a table row. Category, not RuleID, so the
  split-severity halves (TS-L09/TS-L09-escape, TS-L10/TS-L10-distance) and the analyzer-less
  TS-D06 are covered. Enforced structurally by a meta-test (see Testing); a rule added to the
  registry without a reference entry fails `go test`.
- The severity each entry states matches the registry's. Enforced by the same meta-test.
- TS-N12, TS-N13, and TS-N15 appear nowhere in either new document — not even as deprecated
  entries (ENG-161). Enforced by the meta-test as a forbidden-string check over both files.
- Every quoted diagnostic matches the shipped message text in the analyzer sources/corpora, not
  the spec's Diagnostics section (ENG-163 rewrote every message; the spec section lags).
  **Application-level: verified by hand against corpus `// want` patterns during writing and
  review; not mechanically enforced.**

### Behavioral Constraints

- Neither document describes a "reported" tier, a warning tier, or an `[advisory]` output
  marker. Severity is two-state: a rule blocks, or its findings are counted against
  `tiger.budget.yaml` and print nothing under budget (ADR-0011, ADR-0012). The forbidden-string
  check covers `[advisory]` and "reported" as a severity term where mechanically expressible;
  register/phrasing is review's job.
- The explainer never uses a static-analysis term of art before teaching it (linter, AST,
  directive, budget). Human-verified; the standard is ADR-0009's blind-reader test applied to
  prose.
- The explainer's introduction states benefits and evidence before any mechanism. The trial
  material is presented honestly: the ten bugs came from blocking rules that still ship
  (TS-S02 ×5 including the DoS, TS-S08 ×3, TS-C05, TS-C02, TS-E02); the loud noise numbers
  belong to rules since removed (TS-N12/N13/N15) or re-specced (TS-T06) and are told as the
  ADR-0006 removal story, never as current behavior.

## Acceptance Criteria

- `go test ./...` passes, including the new doc-coverage meta-test.
- `docs/Tiger Explainer.html` exists, standalone (html-explainer skill format), and its
  introduction contains the mission framing and trial evidence before any concept teaching.
- `docs/Tiger Rule Reference.md` exists with: an entry per custom-rule category (38 today), an
  auto-rule table (45 linter bindings today), a directive index, a computed-facts section, and
  a budget-mechanics section.
- Each custom-rule entry carries: plain-language what/why, a firing example, the compliant
  rewrite, severity, directive interactions, and the shipped diagnostic line.
- `docs/Tiger Go Documentation.html` and `docs/Tiger Go Human Explainer.md` are deleted.
- `README.md` links both new documents.
- Grep over both new files finds no TS-N12, TS-N13, TS-N15, `[advisory]`, or "reported" as a
  severity.

## Scope

### In Scope

- The two documents, the deletions, the README links, and the meta-test.
- Documenting everything shipped through ENG-153: `tiger check` / `budget` / `golangci` /
  `pin`, `tiger.yaml`, `tiger.budget.yaml`, the ratchet, `--show-facts`, the finish step,
  the golangci-lint plugin caveats (no budgets, no finish step).

### Out of Scope / Non-Goals

- **Agent-facing rule summary** (a terse CLAUDE.md-style digest for AI agents). Named
  future work; it would derive from the reference, which must exist first.
- Spec edits (the two gaps go to a follow-up ticket). Auto-fix and editor integration
  (did not ship; nothing to document). Per-rule doc URLs in `tiger check` output. Changes to
  any analyzer, message, or the registry.
- User stories: skipped — solo author, doc feature, acceptance criteria already at story
  granularity.
- Success metrics: omitted — no post-ship outcome is measured for internal docs.

## Dependencies and Constraints

- Registry on main (`internal/rules/rules.go`, `auto.go`) is the sole source of rule identity
  and severity. The spec supplies per-rule why; analyzer sources and corpora supply diagnostic
  text and example shapes; the trial reports (ENG-148, ENG-159) and wave report (ENG-149)
  supply the adoption narrative; ADRs 0003/0006/0009/0011/0012 supply the escape-hatch,
  removal, diagnostics, and severity stories.
- Writing register: `writing-style` and `lazy` skills apply to every document;
  `html-explainer` governs the HTML deliverable's format.

---

## Functional

### Explainer structure (docs/Tiger Explainer.html)

1. **Why tiger exists** — the mission framing, then evidence: ten real bugs across two trial
   codebases (~59k hand-written lines), led by the remotely triggerable CPU-spin DoS TS-S02
   caught in git-server; byte-identical output run-to-run; zero suppressions — every false
   positive fixed in the analyzer with a corpus case.
2. **What a linter is, and why tiger isn't one** — teach linting, then reading structure (AST)
   vs. reading text; land the verdict-vs-warning distinction.
3. **A day in Tiger Go** — the harvested narrative arc, updated: write code, `tiger check`,
   read a real finding line (post-ENG-163 wording), fix it, pin a fact, watch the pin catch a
   silent change.
4. **The concepts** — rules and the two-state severity; the budget file and the ratchet
   (lowered by the tool, raised only by hand); directives (pins, intent declarations, the one
   escape hatch and its admission test); computed facts and `--show-facts`.
5. **Adopting tiger** — the trial numbers as narrative (first run 1023/712 blocking findings;
   what got fixed vs. budgeted; the escape-hatch arc: 24 `//tiger:batched` cursor waivers took
   TS-S02 from 37 findings to 11 while the real deadlock kept firing); `tiger golangci --init`,
   `tiger budget --write`, the first-run-fails-until-budgeted expectation; plugin caveats.
6. **What tiger will not do** — no warnings, no per-site suppressions, rules that buried the
   signal were removed (ADR-0006); pointers to the reference and the spec.

### Reference structure (docs/Tiger Rule Reference.md)

1. **How to read this reference** — entry anatomy, the two-state severity model, the finding
   line's prefix/body contract.
2. **Budgets and the ratchet** — one shared section: `tiger.budget.yaml`, silent-under-budget,
   TS-D06 overrun behavior, `tiger budget --write`. The five counted categories link here
   instead of restating it.
3. **Directive index** — verb, kind (escape / pin / intent), rules it affects, from
   `directive.Verbs()` and the analyzer bindings.
4. **Custom rules** — grouped by area (S, C, E, L, M, N, T, F, V, P, K, A, X, D), one anchored
   entry per registry category. TS-D06's entry is authored from the CLI budget fixtures (it has
   no analyzer or corpus).
5. **Auto rules** — one table: rule ID, requirement (one line), enforcing linter, link to the
   linter's docs. All auto rules block; tiger audits their config via `tiger golangci`.
6. **Computed facts** — the three `--show-facts` categories, explicitly not rules (ADR-0012),
   and how `tiger pin` freezes them.

Entry contract (custom rules): ID and title; severity; what the rule requires and why, in
plain words (why sourced from spec Part III, rewritten to the newcomer register); a minimal
firing example with the shipped diagnostic line it produces; the compliant rewrite; directive
interactions; known-miss notes only where the corpus documents a deliberate gap a user would
plausibly hit.

Examples are **curated**: hand-written minimal snippets derived from the corpus shapes, not
verbatim corpus slices (corpora are exhaustive edge-case suites, not teaching code). The corpus
remains the executable truth; the doc example's job is legibility.

## Architecture

Single-file documents in `docs/`, matching the repo's existing convention. Hand-written prose;
no generator. Drift control is mechanical where cheap and human where prose:

- **Meta-test** (new file, `internal/rules`, package `rules_test`): asserts reference coverage
  per category and auto rule, severity-string agreement, and forbidden strings
  (TS-N12/N13/N15, `[advisory]`) over both documents. It reads the docs via a repo-relative
  path, the same pattern the existing corpus meta-tests use for `testdata`. Runs under plain
  `go test ./...`; CI needs no change (no Makefile exists; CI runs `go test ./...` directly).
- **Prose fidelity** (register, quoted diagnostics, teaching order) is review's job, using
  ADR-0009's blind-reader standard.

## Testing

Testing follows the `surface-testing` skill, adapted to a documentation surface:

- The one executable surface is `go test ./internal/rules/...`: the doc meta-test is the
  mechanical gate on coverage, severity agreement, and forbidden strings.
- No external dependencies, no async behavior, no clock. The meta-test's only input is the two
  doc files and the registry it already imports.
- Diagnostic-quote fidelity is not mechanically tested (quotes are prose-wrapped and reworded
  examples would false-positive); it is pinned at review time against corpus `// want`
  patterns. Accepted gap, recorded here deliberately.

## Limitations & Future Work

- **Agent-facing digest** — a registry-derived terse rule summary for AI agent context; derive
  from the reference once it exists.
- **Follow-up ticket** — spec gaps: name `tiger.yaml` and `golangci --print`; reconcile the
  145-vs-142 rule count.
- **Per-finding doc links** — if `tiger check` ever emits doc URLs, the single-file layout
  would need per-rule anchors stable enough to link; anchors are already per rule ID, so the
  cost then is a URL scheme, not a rewrite.
- Quoted diagnostics will drift if messages are reworded; the meta-test will not catch it.
  A future rewording pays a doc pass, the same cost ADR-0009 assigns to corpus and fixtures.

## Open Questions

None. The audit decisions, scope calls, and format decisions were all resolved in the design
discussion (2026-09-01).
