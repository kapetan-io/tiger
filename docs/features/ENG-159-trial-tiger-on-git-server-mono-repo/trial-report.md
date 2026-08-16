# ENG-159 — Trial: wave-1 tiger on git-server (mono-repo)

Wave-1 tiger (`tiger check`) ran against the `git-server` service in the
kapetan mono-repo at `321da03` — a real git smart-HTTP server: ~17.3k lines
of hand-written Go (pkt-line transport, v0/v2 negotiation, pack ingest with
delta resolution, an in-memory storage backend, a read-only graph API, and a
large cross-adapter conformance suite), plus duh-rpc generated code produced
by the service's own codegen step. The mono-repo was cloned to scratch at the
pinned commit; `task duh-gen` + `task proto-gen` ran first so
`./services/git-server/...` loaded and vetted cleanly before any finding was
trusted. Every finding class was verified against the analyzer's own
implementation and corpus by independent review passes; a false positive on a
blocking rule was treated as an analyzer bug and fixed in the analyzer with a
corpus case, never suppressed (correctness constraint 7).

Headline: the first run reported **712 blocking + 10 advisory** findings.
After fixing one analyzer defect the same tree reports **711 blocking +
10 advisory**. Output was byte-identical across repeated runs, before and
after the fix. Where the querator trial (ENG-148) found seven analyzer
defects and cut its count 17%, this trial found exactly one — every ENG-148
fix held on a second, differently-shaped codebase: no findings in generated
files (the fresh duh/buf output all carries the standard header), no
func-literal defer confusion, no `-allow` plural mismatch, no non-blocking
select or bare-`ctx.Done()` regressions.

## Analyzer bugs found and fixed

1. **participle: "Nothing" is not a participle.** TS-N14 flagged
   `TestEmptyPackIngestsNothing` — but "nothing" is an indefinite pronoun
   ("no" + "thing"), the same non-derived `-ing` coincidence that already
   put `ring`, `king`, `thing`, and `sibling` on the noun allowlist. The
   allowlist matches exact tokens, so listing `thing` did not cover its
   compounds; `nothing`, `something`, `anything`, and `everything` were all
   latent false positives on a blocking rule. All four are now allowlisted,
   with corpus cases proving each stays silent.

Two suspected defects were investigated and rejected:

- **TS-S08 on `storage/errors.go:64`** (`PackReason.String()`'s
  `default: return "bad pack"`) fires on a type its own doc comment declares
  an *open* vocabulary. The analyzer's closed-set test is syntactic (the
  package declares constants of the type) and has no signal for "extensible
  by design" — but the spec's own rationale ("adding a variant should break
  every switch that has not thought about it") directly opposes the code's
  open-enum intent, so this is rule-design friction of the same class as
  querator's catch-all dispatch sites, not an implementation bug. The rule
  needs vocabulary for a deliberately open enum; see the ranking below.
- **TS-S03 appeared to report 26 findings; it reported zero.** Every TS-S02
  message ends "…or use the ctx.Done() event-loop shape (TS-S03)", so a
  naive per-rule grep tally double-counts. git-server has no event loop
  missing a `Done` select at all. Worth remembering for any tooling that
  tallies findings by substring rather than message prefix.

## Findings per rule (after the fix)

| Rule | Count | Verdict on real code |
| --- | --- | --- |
| TS-T06 tests state their goal | 207 | The ENG-148 gap at its harshest: **205 of 207 (99%)** are tests with real, often excellent doc comments ("TestX asserts … [spec ID, ADR ref]") that lack the literal `Goal:` token. Only 2 are genuinely undocumented — and one of those has a substantive comment stranded above the `import` block where Go's doc attachment can't see it. |
| TS-N12 empty name tokens | 174 | Fires as designed; git's own vocabulary dominates. `base` (67 — pack delta bases, `merge-base`) and `object` (37 — the object database) are 60% and cleanly `-allow`-able. `data` (31), `info` (15 — half is the literal `info/refs` endpoint), `value` (10), `item` (10), `handler` (3), `common` (1). True signal: `BaseURL` ×8, `value` on generic config/header vars, `treeItem`/`items` where the codebase already coined `TreeEntry`. |
| TS-N07 same-type params / max 4 | 112 | ~54% is test/harness surface. Interface fan-out multiplies single decisions (`SetConfig(repo, key, value string)` ×5 implementers = 10 findings; symmetric `MergeBases(a, b)` ×5). The genuine swap hazards are concentrated in the diff/write path: `diffTrees(…, baseTree, headTree)`, `writeHunk`'s three swappable pairs, `fastForward(ctx, graph, old, tip)`, `connectivity`'s four same-type maps — all silently corrupting on swap. |
| TS-N15 name pairs | 59 | 51 are the bare `out` accumulator idiom (`var out []T; append…`) with no `in` partner in sight — the whole-identifier ban on `out` fires regardless. The 8 `src`/`dst`-family findings (`srcSize`/`dstSize` in delta.go) are the rule's own poster child and worth fixing. |
| TS-E02 discard needs comment | 42 | 100% true by spec; 21 are deferred `Close`/`Stop` cleanup (boilerplate-comment remediation, as in ENG-148). One discard is a real latent trap, listed below (`object.Hash`). |
| TS-E06 return arity | 39 | All structural truths. 10 findings are two internal conventions counted per-implementer — `(OID, ok, err)` ref resolution ×7 and `(bool, int) FilterConfig` ×3 — each one design decision, not N fixes. 14 are test-fixture helpers. None generated, none pinned by external interfaces. |
| TS-S02 loop bound | 26 | **4 real bugs (below)** — all wire-protocol or object-graph loops. 10 are pagination-cursor drain loops in tests (the ENG-148 cursor gap). The rest are provably bounded shapes outside the grammar: compound `&&`/`||` of len-bounds (Myers diff, two-pointer merge — 6), varint loops bounded by a finite buffer (5), a wall-clock test poll. |
| TS-S08 switch ends in Unreachable | 14 | **2 real gaps (below)**. 11 are fail-closed or display-only defaults (auth switches default to deny/`LevelAdmin`). 1 is the open-enum `PackReason` friction above. |
| TS-N13 type echo | 13 | 9 are the proto-mirrored RPC method convention (`ReposList`, `RefsList`, … — "List" is the wire verb `repos.list`, and the methods return `error`, not a list). True positives: `countStr`, `modeString`, `oidStr`, and `pathList` — a field that is actually a plain string. |
| TS-N08 no bool params | 12 | Strong: `writeV0Advertisement(…, pushable bool)` takes bare `true`/`false` in production; `walkTree` mixes named and literal call sites; three `trees_test.go` helpers. Weak: 4 where every call site already passes a named expression. |
| TS-D07 skipped tests (advisory) | 10 | All one shape: cross-adapter conformance capability skips ("adapter cannot inject a server-side fault"). Deliberate architecture, and arguably useful visibility — but not the forgotten-skip failure mode the rule is written for. |
| TS-S06 one operator per condition | 6 | All safe, parenthesized idioms: character-class checks, the published Myers boundary tests, EOF-or-flush loop exits. |
| TS-S18 no naked panic | 4 | All in the conformance MVP adapter — test-only fixture panics never linked into the server binary. Unlike querator (15 + 15 blocked on assert adoption), git-server barely trips the assert-prerequisite rules. |
| TS-N14 participle | 1 | Was 2; the "Nothing" false positive is fixed. The survivor, `…RefPrefixFiltering`, is a genuine gerund — and a borderline allowlist candidate of the `encoding`/`logging` heading class. |
| TS-T10 named table cases | 1 | True by the letter; the table's `guess` field is the `t.Run` subtest name, so failures already self-identify. |
| TS-C02 goroutine via supervisor | 1 | The reference test oracle's `go srv.Serve(ln)` — the plain serve-until-`Close` idiom, self-terminating, test-only. |
| TS-S03 event loop selects Done | 0 | No genuine findings (see the tally note above). |

## Real git-server bugs the rules surfaced

Five genuine defects found by blocking rules, worth mono-repo tickets
(filing them is a separate decision):

1. **Unbounded annotated-tag peel chain — remotely triggerable CPU-spin
   DoS** (TS-S02, `internal/graphapi/resolve.go:114`): `peelToCommit`
   follows `tag.Object` with no depth cap and no cycle check. Content
   addressing prevents a self-cycle, but `pack.Ingest` persists objects
   before any ref update and validates only *delta* depth, so a pusher can
   land two tags whose `object` headers point at each other (built
   bottom-up), and `resolveRev` accepts any bare hex OID without requiring
   ref reachability. One graph-API request for that OID then spins the
   handling goroutine at 100% CPU forever — no I/O, no `ctx` check, no
   response. Even the acyclic form (a long chain pushed in one pack)
   makes every future read against that tag O(chain-length) with no
   memoization. Fix shape: cap the peel depth (git itself caps at 100),
   assert/error on exhaustion.

2. **Unbounded pkt-line command loop — slow connection exhaustion**
   (TS-S02, `internal/protocol/engine.go:99`): `parseCommand`'s first
   `for{}` reads capability pkt-lines until a delimiter. A client that
   streams well-formed capability lines without ever sending the delimiter
   ties the goroutine and connection slot indefinitely. No count cap, no
   byte cap, no `ctx.Done()` select. The same function's second loop
   (`:129`) accumulates arguments into `cmd.args` until a flush — same
   shape, but additionally grows an unbounded slice (heap exhaustion, not
   just a goroutine pin). Whether the platform's HTTP server applies a
   `ReadTimeout` is unknown from git-server's own code; regardless, the
   protocol layer should state its own limit.

3. **Unbounded ref-update accumulation** (TS-S02,
   `internal/protocol/receive.go:99`): `parseCommands` reads ref-update
   pkt-lines into an uncapped `updates` slice before the pack body's
   `maxInputSize` limit ever applies. Same shape as the argument loop.

4. **Silently discarded hash-format error — latent OID corruption**
   (TS-E02, `internal/object/object.go:24`): `Hash()` hardcodes `sha1.New()`
   but accepts a `format storage.HashFormat` parameter and passes it to
   `storage.NewOID`, whose validation (raw length != format's expected
   length) returns an error that is silently discarded. If SHA-256 support
   (D9) is ever wired up, `Hash` would return a zero-value `OID{}` —
   a corrupted object identifier produced mid pack-resolution
   (`pack.go:234,266`). Currently benign (only SHA-1 is live), but the
   function's signature already promises format-awareness.

5. **`writeHunk` silent content loss on future diff-op tags** (TS-S08,
   `internal/graphapi/diff.go:333`): the `switch op.tag` has no default
   arm at all — not even a fail-loud one. All four current `opTag`
   constants are covered, but a fifth (e.g. a moved-line op) would match
   no case and silently emit nothing for that hunk segment, producing a
   quietly truncated unified diff in the API response.

6. **`packCode` returns invalid type nibble for `TypeInvalid`** (TS-S08,
   `internal/pack/writer.go:66`): the default returns `0`, which is not a
   legal git pack object-type code. `TypeInvalid` is a live zero value
   in this codebase (`object.go:365` assigns it to unresolved tag
   targets), so a code-path bug that lets it reach `Write` would silently
   produce a corrupt pack stream — the failure surfacing far away at the
   git client's unpack step.

## Adoption experience

- **Determinism holds**: `tiger check` twice over ~17.3k LOC (plus
  generated code, excluded by the driver), byte-identical, before and
  after the fix.
- **Generated-file exclusion works end to end**: `task duh-gen` +
  `task proto-gen` produce `internal/generated/server.go`, `client.go`,
  and `api/v1/git-server.pb.go` — all carrying the standard
  `// Code generated … DO NOT EDIT.` header. Zero findings landed in any
  of them. The ENG-148 driver fix (`ast.IsGenerated`) held with no
  regression, and the two driver paths (standalone + plugin) agree on the
  exclusion.
- **TS-S03 message cross-reference confuses grep tallies**: every TS-S02
  message includes "…or use the ctx.Done() event-loop shape (TS-S03)" as
  a hint, so `grep -o 'TS-S03' run.txt | wc -l` returns 26 when the
  real S03 count is 0. Any report tooling that tallies by substring
  rather than the structured prefix `TS-S03:` will double-count. Worth a
  note in the documentation or a switch to a machine-parseable output
  format.
- **`-allow` scaling on a second codebase**: git-server needs 2 entries
  (`base`, `object`) to silence 60% of N12. But both tokens are
  genuinely *mixed* — `base` covers 59 domain uses and 8 generic
  `BaseURL` uses, `object` is pure domain — and `-allow` is global
  (token-scoped, not file/package-scoped), so allowing `base` silences
  the 8 true positives alongside the 59 domain uses. The single-flag
  design is adequate for a trial and a single-service run but does not
  scale to a mono-repo where the same token is domain vocabulary in one
  service and vague in another.
- **No wave-1 escape for any rule**: as in ENG-148, the `//tiger:batched`
  directive is validated and reported as a standing advisory, but its
  target rule TS-M10 (`ioinloop`) ships in wave 1.5, so there is still
  no escape for any wave-1 rule. On this codebase the bite is milder
  (no `//tiger:batched` annotations exist), but the 10 pagination-cursor
  S02 findings remain unfixable without an artificial cap or the
  sanctioned cursor-loop grammar.

## Ranked wave-1.5 priorities (updated from ENG-148)

Ranked by what both trials together demonstrate, strongest evidence first:

1. **`ioinloop` (TS-M10)** — confirmed again. Completes the only escape
   hatch; `//tiger:batched` is validated but nothing consumes it.
   git-server has no `//tiger:batched` annotations (querator had 30
   `continue nextBatch` sites), so the urgency comes from the S02
   cursor/iterator gap rather than a direct M10 need — but shipping M10
   unblocks the sanctioned answer for *both* codebases' iterator loops.

2. **Bound-grammar extensions for S02** — elevated from ENG-148's #3.
   git-server's loop population is dominated by shapes wave-1 cannot
   recognize: compound `&&`/`||` conditions hiding a real len-bound (6
   findings — Myers diff, two-pointer merge), varint loops bounded by a
   finite `io.Reader` (3), and cursor/iterator drain loops (10, same
   ENG-148 gap). Together these are 19 of 26 S02 findings (73%). A
   streaming-IO, pagination-heavy, algorithm-dense service makes the
   strongest case yet for widening `boundedCond` to handle monotone
   compound conditions and slice-cursor iterators — or synthesizing a
   decidable "monotone cursor bounded by len/cap of a fixed collection"
   escape.

3. **A second recognized shutdown shape for S03/C05** — unchanged from
   ENG-148's #2. git-server produced zero S03/C05 findings (it uses
   `ctx` idiomatically throughout), so it neither strengthens nor weakens
   the case built by querator's `requestLoop`/`shutdownCh` architecture.
   Still the highest-leverage tuning for codebases that use the
   closed-channel broadcast pattern.

4. **Open-enum vocabulary for S08** — new entry. `PackReason` is the
   first documented open enum either trial hit. The analyzer's
   `closedSetPackage` test (purely syntactic: "does the package declare
   constants of this type?") cannot distinguish a deliberately extensible
   vocabulary from a closed enum that happens to have few members.
   Low-volume (1 finding), but the design question matters: an
   `//tiger:openenum` directive, or inference from a doc-comment
   convention, would resolve it cleanly.

5. **`declorder` / `declusedistance`** — unchanged. Cheap wins on
   proven corpus conventions; git-server's codebase is well-organized
   enough that these would likely produce low noise.

6. **`noabbrev` / `restatement`** — unchanged. The naming-dictionary
   rules are the noise leaders again (N12+N13+N15 = 35% of remaining
   findings), dominated by domain vocabulary and API conventions. A
   per-repo committed allowlist file (with reasons, reviewed like code)
   remains the prerequisite before these rules can gate a real codebase
   without drowning the signal.

7. **`assertdensity` / `invariantsymmetry`** — unchanged, lowest
   priority. git-server trips only 4 S18 findings (all test-only) and
   14 S08 (mostly fail-closed), a far milder assert-adoption gap than
   querator's 30. The case for density/symmetry rules is weaker here.

Supporting infrastructure the two trials together argue for, independent
of analyzer order: per-repo allowlist files for the naming rules,
a machine-parseable output format (or at minimum structured message
prefixes) for correct automated tallying, `tiger golangci`
baseline-merge output, and an open-enum annotation or inference
mechanism for S08.