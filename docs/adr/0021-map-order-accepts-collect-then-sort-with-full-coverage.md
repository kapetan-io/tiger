# 21. The map-order rule accepts collect-then-sort with plain sorts and full coverage

Date: 2026-10-12

## Status

Accepted

This ADR records ENG-191 call 8 and call 12. Call 12 is a documentation fix that describes the
rule call 8 decides, so it has no ADR of its own.

## Context

TS-T02 bans ranging over a map unless the loop body matches an allowlist of shapes that can't
depend on order, such as summing, counting, or writing into another map. Go randomizes map order on
every run, and the rule keeps that randomness out of anything the program emits.

The allowlist missed a common correct shape. querator's in-memory user store collects ids and sorts
them before use, and TS-T02 blocks it:

```go
ids := make([]string, 0, len(m.users))
for id := range m.users {
	ids = append(ids, id)
}
sort.Strings(ids)
```

Of four querator findings sampled, one was a real order leak into logs and three were shapes the
allowlist does not model
(`docs/features/ENG-191-due-diligence-validate-tigers-approach/decision.md`, call 8).

Trusting every sort is not safe. About 15% of comparator sorts after a map loop in the module cache
tie and leak map order (`docker images`, x/tools linecount, revive, containerd's unmount order,
`go/ast`'s `CommentMap.String`). The rule also had an escape: `slices.Collect(maps.Keys(m))`
collects keys without a sort and passes today.

The documents disagreed about what the rule is. The specification calls it a "range over a map
whose body appends or writes. Heuristic". The explainer called it a heuristic too. The code does
neither: it is an allowlist, which the ENG-150 blueprint chose on purpose (call 12).

## Decision

TS-T02 stays blocking and gains four changes, each proven in `experiments/fullcoverage`:

1. **Plain sorts.** A map loop that appends into a local slice passes when the slice's first use
   after the loop is `sort.Strings`, `sort.Ints` or `slices.Sort` on string or integer elements.
   Equal values of those types can't be told apart. Floats are excluded, because -0 and +0 compare
   equal and print differently.
2. **Full coverage.** A comparator sort passes only when its comparisons, taken together, cover
   every field of the element with `cmp.Compare`, `strings.Compare`, or `bytes.Compare` on a whole
   array. That proves the comparator returns 0 only for identical elements. Pointer, interface,
   bool, float and `time.Time` fields never count as covered. A `Compare` method on a key type that
   passes the same check counts too, as in `slices.SortedFunc(maps.Keys(m), K.Compare)`. Every other
   comparator sort fires with a message naming `slices.Sorted(maps.Keys(m))` followed by
   `slices.SortStableFunc`.
3. **The escape closes.** A `maps.Keys`, `maps.Values` or `maps.All` result fires unless it goes to
   `slices.Sorted`, a full-coverage `SortedFunc`, `maps.Collect` or `maps.Insert`, or a range that is
   checked like a map range.
4. **No reads of carried state.** A map write gated by a counter, such as copying the first three
   entries visited, fires.

Full coverage was chosen over proving a comparator total by tracing the key, because it is a check
on the element's type, not on what the code means.

The documents describe this rule (call 12). A finding means the loop's shape is outside the
allowlist, not that the output really varies. The rule is not called a heuristic, and it is not
called exact either, because it has five known misses, all from `experiments/fullcoverage`:

- last writer wins (`inverse[v] = k` when two keys share a value)
- a map write whose value calls a function (`labels[k] = fmt.Sprint(v)`)
- `golang.org/x/exp/maps.Keys`
- `reflect.Value.MapKeys`
- a range over a type parameter constrained to a map

TS-T11, which runs the tests twice and compares the output, backs up those misses on every path the
tests exercise.

## Consequences

- Correct collect-then-sort code stops firing. Plain sorts accept 91 sites in the module cache,
  including querator's 4. On the trial code findings drop from 11 to 7 on querator and from 17 to 15
  on git-server, and none are added.
- Full coverage accepts few sites (4 in 970 modules, all in tiger), because most comparators are
  `sort.Slice` or cover only the key. Its value is the path for struct keys and the closed escape.
- The experiment's soundness test runs the prototype's verdict on 72 functions, 9 of them real
  sites, and calls each 200 times. No function it passes prints more than one result, and every
  boundary probe it rejects really does vary.
- Keys with no order, such as pointers and interfaces, have no check tiger can make. They get a
  pattern (call 15).
- Tightening the rest of the allowlist is a separate follow-up. The prototype also refuses
  `keys[i] = k; i++` and reads of carried state anywhere in the body, which finds 31 real leaks
  tiger misses across the module cache and adds 47 false findings to work down first.
- The specification's TS-T02 line and Part V analyzer table, and the explainer, change to describe
  the allowlist, the closed escape, and the five misses.

## Amendment (ENG-341)

The follow-up named in the last Consequences bullet shipped. The decision above stands, with these
changes to what the rule accepts and refuses:

- The first-use check follows a slice by name only, so the slice must not be readable under another
  name before its sort. A package-level slice, one whose address is taken, one a function literal
  captures, one that is sliced or copied, and one that starts from anything but a fresh slice no
  longer qualify.
- A map loop may fill a preallocated local slice by position (`keys[i] = k` directly followed by
  `i++`), and `slices.Collect` or `slices.AppendSeq` of a map iterator may fill one, when a plain
  sort comes next. `sort.Slice` and `sort.SliceStable` count as a plain sort only when the less
  function compares the slice's own string or integer elements.
- A loop that can `break` or `return` fires when it writes a map, adds to a counter or deletes, or
  when an exit skips a constant outcome the body sets on another entry.
- A loop that writes or deletes an entry of its own map other than the current one fires. So does
  a `break`, `continue` or `goto` between a collect and its sort.
- Parentheses around an iterator call and a label on the map loop change nothing.

A scan of 970 module-cache modules, comparing against the rule as it stood before ENG-340, finds 84
sites that fire only now. A read of each found 26 real leaks, 54 false findings and 4 it could not
decide. The false
findings need a proof about the data and are recorded as known false findings in the rule
reference. A comparator held in a variable and `sort.Sort` would clear none of them, so neither
was added.
