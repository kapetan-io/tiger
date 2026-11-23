# `tiger changed` output format

This is the specification an implementer builds against. It includes the amendments for partial
loads and for naming the head revision.

Decisions of 2026-10-14 this version is built on: one list of changes grouped by the function where
the call is written; no file:line positions in the human formats; the old split between "started
calling" and "changed calls to things already called" is gone.

## 1. Command line

```
tiger changed [-C dir] [--format text|markdown|json] <base>
```

Flags come before `<base>`, as in every tiger command; a flag after `<base>` exits 2.

- `--format` defaults to `text`. The default does not depend on whether stdout is a terminal, so the
  same command always prints the same bytes. No ANSI color in any format.
- There is no `--link` flag: the human formats print no positions, so there is nothing to link.
  Positions are in the JSON format only.
- Output goes to stdout. Exit status is 0 whenever a report is produced, whatever it contains
  ("Nothing blocks"). Exit status 2 when `<base>` does not resolve or either revision fails to load;
  the error goes to stderr as one line written for a newcomer (ADR-0009 body rules, no rule code).
- `<base>` is resolved to `git merge-base <base> HEAD`, the same comparison GitHub's "Files
  changed" tab uses (user decision, 2026-10-14). The base revision is loaded from that commit; base
  sites in JSON are line numbers in that commit's files.

## 2. Vocabulary

- **Owner**: a named type whose method holds a call site, or a free function. Decided in ADR-0026.
  Attribution is to the owner where the call is written.
- **Function**: where a call is written: one method of an owner type, or a free function. Calls
  inside closures belong to the enclosing function.
- **Callee**: either
  1. a function, or one method of a type or interface, in another package that passes the mixed
     filter; or
  2. a function or method of a *different owner in the caller's own package* whose own body calls a
     callee of kind 1, at the revision the line comes from (head for `+`, base for `-`). Only that
     function's own body is read; chains of same-package calls are not followed. A call between
     methods of one owner, and a call through an interface declared in the caller's own package,
     is never a callee of kind 2. (User decision, 2026-10-14; arena `arena-samepkg`, rule R3.)
- **Diff, then filter.** Kind 2 is decided after base and head are compared: a call present at both
  revisions never prints a line, even when its callee started or stopped calling another package.
  That change prints under the callee, where it was written (ADR-0026).
- **Call**: one (function, callee) pair. A call has one or more **sites** (`file:line`), which only
  JSON prints. A call that exists at head has head sites; a call that is gone has base sites.

Every difference between base and head falls in exactly one category:

| Category | Rule | Shown as |
|---|---|---|
| **Call added** | call at head, not at base; its owner's package existed at base; owner not part of a rename | `+` line under its function's header in the main list |
| **Call removed** | call at base, not at head; its owner's package exists at head; owner not part of a rename | `-` line under its function's header in the main list |
| **Renamed** | exactly one owner gone and one owner new in the same package, calling identical sets of callee types and functions (at least one) | one line; their calls are not also listed as added or removed |
| **New package** | owner's package did not exist at base | one package summary; its calls in a folded detail, not in the main list |
| **Removed package** | owner's package does not exist at head | one line |

A call from existing code into a new package is a call added and appears in the main list
(git-server 321da03: `internal.Daemon.OnStart` calling three `internal/graphapi` constructors).

Whether the owner already called the callee's package at base is not shown in the human formats.
JSON carries it as `package_dependency` (section 8).

## 3. Names

- Module packages relative to the directory tiger runs in when they are under it
  (git-server run in `services/git-server`: `internal/graphapi.Mux`), otherwise relative to the
  module root (`platform/libs/go/platform/v1.LoadSettings`). The package at the run directory itself
  prints by its package name, as today (`daemon.setupQueueStorage`, `querator.NewClient`).
- Outside packages in full (`golang.org/x/sync/singleflight.Group.Do`).
- **Function header**: `<pkg>.<Type>.<method>` for a method, `<pkg>.<function>` for a free function
  (`internal.Daemon.OnStart`, `internal/transport.HTTPTransport.authorize`,
  `internal/storage/memory.sortOIDs`).
- **Callee**: `<pkg>.<function>`, or `<pkg>.<Type>.<Method>` for a method on a type or interface;
  one line per method (`internal/storage.OID.String`, `internal/graphapi.NewMux`).

## 4. Ordering (determinism)

All ordering is byte order of the printed names, never map order or discovery order.

- Function headers: byte order.
- Within a header: `-` lines before `+` lines, each by printed callee.
- Renamed: by package, then old name. New and removed packages: by package.
- New-package summary symbol lists: byte order.
- JSON: arrays in the same order; sites by (file, line).

## 5. The list (text and markdown share it)

```
<function header>
<s>   <callee>
<s>   <callee>
<function header>
<s>   <callee>
```

- `<s>` is `+` for a call added, `-` for a call removed, then three spaces, then the callee.
- A header never starts with `+` or `-`, so in a ` ```diff ` fence GitHub colors exactly the call
  lines, and the number of colored lines equals the counts line.
- No positions, no tags. Whether a function is new, removed, or already called the package is not
  shown; a reviewer finds the call by the function name.
- A call moved between two functions reads as a `-` under one header and a `+` under the other
  (38afd35: `internal/store.APIKeys.UpdateLastUsed` removed from
  `internal.AuthCache.Authenticate`, added to `internal.AuthCache.lastUsedLoop`).

## 6. Text format (`--format text`, the default)

```
tiger changed: <base> -> <sha7>[ + uncommitted changes]
Functions in this change that started or stopped calling another package, or a function in their own package that calls one. Check that each one is intended.
<blank>
<counts line>
<blank>
<the list>                         | <empty sentence>
<blank>
[Renamed: ... <blank>]
[New package block <blank>]
[Removed package ... <blank>]
<footer line 1>
<footer line 2>
```

- **Revision**: base is `<ref> (<sha7>)`; when `<ref>` is itself a prefix of the SHA, just `<sha7>`.
  When the merge base differs from the tip of `<ref>`: `<ref> (merge base <sha7>)`. These rules
  apply to base only. Head is `<sha7>` of HEAD, never a branch or ref name, plus
  ` + uncommitted changes` when `git status --porcelain` prints anything.
- **Purpose sentence**: fixed text, line 2, for a reader who has never heard of tiger.
- **Counts line**: `Calls added: N · Calls removed: N` always; then, only when non-zero and in this
  order, `New packages: N`, `Removed packages: N`, `Renamed: N`. Calls added and Calls removed are
  exactly the number of `+` and `-` lines in the main list. Calls written in a new package are
  counted under its package block, not here.
- **The list**: section 5, unindented, no fence.
- **Empty sentence**, in place of the list:
  - every category zero: `No function started or stopped calling another package, or a function in its own package that calls one.`
  - the list is empty but a package was added or removed, or an owner renamed:
    `No function in an existing package started or stopped calling another package, or a function in its own package that calls one.`
- **Renamed**: `Renamed: <old> looks renamed to <new>; it calls the same N types and functions in other packages`.
- **New package** block:
  ```
  New package <pkg>: <O> types and functions, <E> calls
    Uses existing packages: <pkg> (<symbols>); ...      | none
    Uses new packages: <pkg> (<symbols>); ...           | none
    Uses outside packages: <pkg> (<symbols>); ...       | none
    Every call, by function:
      <the list for this package, indented four spaces; + lines only>
  ```
  "Existing" means a module package that existed at base; "new" a module package that did not,
  including skipped generated ones (`internal/generated`). E is the number of `+` lines in the
  block. There is no "Entered from" line: calls from existing code into the new package are in
  the main list.
- **Removed package**: `Removed package <pkg>: <O> types and functions`.
- **Footer**, always, exactly two lines:
  1. `Compared <O> types and functions in <P> packages; skipped <N> generated files (<paths>) and all test files.`
     With no generated files: `…; skipped all test files (no generated files found).` Paths are
     relative to the run directory, byte order; over 5, the first 5 and `and N more`.
     When a package did not load at base or at head, it is left out of both sides, and the line
     gains, before its final period, `; <N> packages did not load and were not compared (<pkgs>)`,
     with packages in byte order and over 5 shortened as the generated-file list is.
  2. `Tiger can't see: calls into outside packages that never reach the operating system (fmt, strings, error libraries), calls between methods of one type, calls inside a package to a function that calls no other package or to an interface the package declares, writes through io.Writer, work handed over a channel, or a new implementation behind an unchanged interface. A call moved into a new helper, or a renamed type whose calls changed, shows as removed plus added.`

No line wrapping.

## 7. Markdown format (`--format markdown`, for the PR comment)

```
<!-- tiger-changed -->
#### tiger changed: `<base ref>` (<sha7>) → `<sha7>`[ + uncommitted changes]

Functions in this PR that started or stopped calling another package, or a function in their own package that calls one. Check that each one is intended.

<counts line, each non-zero "Label: N" wrapped in **…**>

```diff
<the list>
```

[sections]

<sub><footer line 1><br>
<footer line 2></sub>
```

- Line 1 is the marker CI uses to find and update the one comment. It is always the first line.
- **Purpose sentence**: fixed text, directly under the title, plain (not bold).
- **Counts line**: same labels and rules as text; each non-zero `Label: N` is bold, zeros are not
  (`**Calls added: 9** · Calls removed: 0 · **New packages: 1**`).
- **The list**: one ```diff fence holding section 5's lines. Never folded.
- **Empty sentence**: the same two cases and wording as text, plain, in place of the fence.
- **Renamed**: `` **Renamed:** `<old>` looks renamed to `<new>`; it calls the same N types and functions in other packages. ``
- **New package**: `` **New package `<pkg>`** — O types and functions, E calls: ``,
  three bullets (`Uses existing packages`, `Uses new packages`, `Uses outside packages`), then
  `<details>`, `<summary>Every call from <code><pkg></code>, by function (E)</summary>`, a blank
  line, a ```diff fence with the package's list (`+` lines only), a blank line, `</details>`. The
  summary bullets stay open: they are the wiring a reviewer checks. Summary text uses `<code>`,
  never backticks (GitHub does not render markdown in `<summary>`).
- **Removed package**: `` **Removed package `<pkg>`** — O types and functions. ``
- **Footer**: the two text footer lines inside one `<sub>`, separated by `<br>` and a newline.
- **Empty report** (every category zero): title, purpose sentence, counts line, the empty
  sentence, footer.
- **Size**: GitHub rejects comments over 65,536 characters. If the rendered comment exceeds 60,000,
  drop in this order until it fits: (1) every new package's `<details>` block, replaced by one line
  ``Too long for a comment; run `tiger changed <base>` for every call.``; (2) the main list after
  the last whole function header that fits, closing the fence there and adding, below it,
  ``N more calls in M functions not shown; run `tiger changed <base>` for the rest.`` The counts
  line keeps the full totals; that line is the only case where visible lines are fewer than the
  counts. Never drop the purpose sentence, the counts line, the new-package summary bullets or the
  footer.

## 8. JSON format (`--format json`)

For tools and agent reviewers that act on individual call sites. One object, two-space indent,
keys in the order below, arrays in the section-4 order, trailing newline. JSON keeps every site of
every call, tagged with the revision it belongs to. Its `functions` array matches the main list one
to one: one entry per header, one `calls` entry per `+`/`-` line.

```json
{
  "schema": "tiger.changed/v1",
  "base": {"ref": "main", "commit": "<full sha>", "merge_base": true},
  "head": {"commit": "<full sha>", "dirty": false},
  "scope": {"owners": 184, "packages": 13, "skipped_generated_files": ["<path>"],
            "not_compared": ["<pkg>"]},
  "counts": {"calls_added": 0, "calls_removed": 0, "new_packages": 0,
             "removed_packages": 0, "renamed": 0},
  "functions": [Function],
  "renamed": [{"package": "...", "from": "...", "to": "..."}],
  "new_packages": [{
    "package": "internal/graphapi", "owners": 11, "calls": 83,
    "uses": {"existing": [Uses], "new": [Uses], "outside": [Uses]},
    "functions": [Function]}],
  "removed_packages": [{"package": "...", "owners": 3, "calls": 7, "functions": [Function]}],
  "blind_spots": ["outside-without-os", "io-writer", "channel-send",
                  "interface-implementation", "same-owner", "same-package-leaf",
                  "same-package-interface", "moved-to-helper", "renamed-type"]
}
```

- `Function` = `{"function": "internal.Daemon.OnStart", "owner": "internal.Daemon",
  "method": "OnStart" | null, "calls": [Call]}`; `method` is null for a free function.
- `Call` = `{"change": "added" | "removed", "callee": "internal/graphapi.NewMux",
  "target": "internal/graphapi.NewMux", "method": "NewMux", "target_package": "internal/graphapi",
  "target_in_module": true, "same_package": false,
  "package_dependency": "new" | "kept" | "dropped" | null, "sites": [Site]}`.
  `callee` is the printed name; `target` and `method` are its parts.
  `package_dependency` keeps the old edge distinction for tools: for an added call, `new` when the
  owner made no call into `target_package` at base, else `kept`; for a removed call, `dropped`
  when the owner makes no call into it at head, else `kept`. It is `null` when `same_package` is
  true, since a package does not depend on itself. The human formats show neither field.
- `Site` = `{"file": "<path relative to the run directory>", "line": N, "rev": "base" | "head"}`.
  The line is the call's opening parenthesis (or the `go`/`defer` keyword). `rev` is `head` for
  added calls and new packages, `base` for removed calls and removed packages; an agent resolves a
  `base` site against `base.commit`.
- `head` has no `ref`. When `dirty` is true, head sites refer to the working tree, not to
  `head.commit`.
- `scope.not_compared` lists the packages that did not load at base or at head, in byte order. It
  is always present, and empty when none.
- `Uses` = `{"package", "symbols": [...]}`.
- `blind_spots` is a fixed list of stable identifiers; their prose is the footer's second line.
- `counts` equals the human counts line. The schema string changes only on an incompatible change.

## 9. Diagram

None. On aeb014c a diagram repeats two lines; on git-server it is 42 nodes and 36 arrows filtered,
past what a skimming reader can trace.

## 10. Who posts the PR comment

CI, not tiger. Tiger stays offline, holds no token and prints bytes; posting is a few lines of
workflow. Rule for the workflow:

- The checkout step fetches full history (`fetch-depth: 0`), or the merge base is missing and
  tiger exits 2.
- The checkout step uses `ref: ${{ github.event.pull_request.head.sha }}`. Under the default
  checkout the title SHA is a merge commit that appears nowhere in the PR, so a stale comment
  cannot be recognised, and the merge base becomes the tip of the base branch, not the fork point
  that "Files changed" uses.

- Every run posts. The first run on a PR creates the comment, whatever the report holds, an empty
  report included; every later run updates that same comment in place, found by the
  `<!-- tiger-changed -->` marker. One sticky comment per PR.
- An empty report is posted because silence is the thing a reader must not take as proof: the
  comment shows that tiger ran on this head, what it compared and what it cannot see. A later push
  that removes a finding updates the comment to say so, and the head SHA in the title makes a
  stale comment visibly stale.
- The text report also goes to the job summary.

```yaml
- run: |
    tiger changed --format markdown origin/${{ github.base_ref }} > changed.md
    tiger changed origin/${{ github.base_ref }} >> "$GITHUB_STEP_SUMMARY"
    id=$(gh api repos/${{ github.repository }}/issues/${{ github.event.number }}/comments --paginate \
      --jq '[.[] | select(.body | startswith("<!-- tiger-changed -->"))][0].id // empty')
    if [ -n "$id" ]; then
      gh api -X PATCH repos/${{ github.repository }}/issues/comments/$id -F body=@changed.md
    else
      gh api repos/${{ github.repository }}/issues/${{ github.event.number }}/comments -F body=@changed.md
    fi
```

An agent reviewer runs `tiger changed --format json` itself rather than scraping the comment.

## 11. The brief's open questions, answered

| Question | Answer |
|---|---|
| Grouping | One list, grouped by the function where the call is written; one header per function, one line per added or removed call. New packages, removed packages and renames sit outside the list. |
| `+`/`-` markers | Yes, on every call line, in one ```diff fence so GitHub colors them. Headers carry no sign, so colored lines equal the counts. |
| Method detail | Both sides fully named: the header names the owner's method, each line names the callee's method. |
| Positions | Not in text or markdown. JSON keeps every site with `rev` base or head. |
| Diagram | No (section 9). |
| `~` section | Gone. A new call from a method of a type that already called the package is a `+` line like any other. |
| New package | One summary: what it uses in existing, new and outside packages; every call folded in the same per-function layout. |
| Blind spots / empty | A fixed purpose sentence under the title, and a fixed two-line footer on every report: what was compared and skipped, then "Tiger can't see:" and the blind spots. An empty report says no function started or stopped calling another package, then the footer. |
| Removed edges | Shown, as `-` lines, plus Removed package and Renamed. |
| JSON | Yes, `tiger.changed/v1`, grouped like the list, every site tagged with `rev`. |
| Output selection | `--format`, default `text`. No `--link`. |
| Who posts | CI, always, one sticky comment updated in place, an empty first report included (section 10). |
| Revisions named | Yes, first line, base ref plus short SHA, head short SHA, merge base and dirty tree spelled out. |

## 12. Open questions for the human

| Question | Status |
|---|---|
| Should `<base>` resolve to the merge base (section 1, matching GitHub's diff) or to the branch tip? | Decided 2026-10-14: merge base. |
| Should calls inside one package count? | Decided 2026-10-14: section 2, callee kind 2. |
| Is one per-function list quiet enough on a high-churn repository? Every added or removed call now appears, including a call moved between methods of one owner. Measured only on querator (at most 9 lines a commit) and git-server 321da03 (10 lines, plus 91 folded in the new package). | Open. |
| Naming relative to the run directory (dropping `services/git-server/`) changes the prototype's naming. | Open; confirm. |
| ADR-0026 defines the report as additions of owner-to-callee edges and says a new method on a type already called is not reported. This format reports every added and removed call per function. | Needs an ADR-0026 amendment if adopted. |
