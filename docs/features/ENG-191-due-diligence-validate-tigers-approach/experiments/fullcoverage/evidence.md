# Full coverage for TS-T02: evidence

This records what the `fullcoverage` prototype did on real code. The tests in this module
(`test.out`) cover the corpus and the case packages; this file covers the module cache, the
three trial pins, and the classification of every accepted comparator site.

## How it was run

- Prototype: `cmd/fullcoverage`, run as a vet tool with `-sites`, which also reports every
  map-derived sort site it judges (accepted or fired) under category `site`.
- Baseline: tiger's `internal/analyzers/maporder/maporder.go` at this branch, copied into a scratch
  module and built as a vet tool (tiger's `internal` path can't be imported from here).
- Scanner: `cmd/scan`. It takes the latest version of every module in `~/go/pkg/mod`, copies its
  go.mod to a temp dir (`-modfile`, so the read-only cache is never written), resolves only from
  the cache (`GOPROXY=file://…/cache/download`, nothing downloaded), lists packages with
  `go list -e`, and vets those with no load errors. `GOTOOLCHAIN=local` (go1.26.2) for the cache;
  go1.26.4 for the pins, since mono-repo requires it.
- Positions are de-duplicated: vet analyzes a package and its test variant, so a non-test line can
  print twice.

Reproduce:

```
go build -o /tmp/fc ./cmd/fullcoverage
go run ./cmd/scan -vettool /tmp/fc -modcache ~/go/pkg/mod -workers 8 -out sites.tsv -status status.tsv
go run ./cmd/scan -toolchain go1.26.4 -vettool /tmp/fc-1.26.4 <querator> <mono>:./services/git-server/... <tiger>
```

### Load coverage (module cache)

| | Modules |
|---|---:|
| Latest-version modules in the cache | 970 |
| Every package vetted | 424 |
| Some packages vetted | 254 |
| None vetted (70 have no go.mod; 222 have dependencies missing from the cache or need a newer Go) | 292 |
| Packages vetted / listed | 5,474 / 9,242 |

The skipped modules are counted, not scanned. The comparator evidence below is therefore thinner
than the earlier AST-only scans, which did not need dependencies.

## Module cache: map-derived sort sites (non-test, unique positions)

A site is a map loop that appends into a slice whose first use after the loop is a sort call
(`collect-*`), or a `slices.Sorted*` call on `maps.Keys`/`maps.Values` (`iter-*`).

| Arm | Accepted | Fired | What fires |
|---|---:|---:|---|
| collect-ordered (`sort.Strings`, `sort.Ints`, `slices.Sort`) | 91 | 4 | `sort.Float64s` (1), sort of a sub-slice `x[1:]` (3) |
| collect-comparator (`slices.SortFunc`, `SortStableFunc`) | 0 | 45 | comparator does not cover every leaf; 36 are one generated file |
| collect-other (`sort.Slice`, `SliceStable`, `Sort`, `Stable`) | 0 | 39 | these comparators are never analyzed |
| collect-body (a sort follows, but the loop body is not a pure collection) | 0 | 66 | calls, `break`, carried state in the body |
| iter-sorted (`slices.Sorted(maps.Keys/Values(m))`) | 30 | 3 | element is a type parameter that could be a float (3) |
| iter-comparator (`slices.SortedFunc` / `SortedStableFunc`) | 4 | 0 | |
| **Total** | **125** | **157** | |

Test files add 15 + 6 accepted and 29 fired.

Comparator sorts whose loop body passes (collect-comparator, collect-other, iter-comparator): 88.
Full coverage accepted 4 of them (4.5%), all in one file of tiger itself. On this corpus nearly all
of the benefit comes from the plain-sort arm (91 sites), not from full coverage.

## Every accepted comparator site

The same four lines appear twice: once in the module cache's copy of tiger
(`github.com/kapetan-io/tiger@v0.0.0-20260917155855-5126797f1778`) and once in the tiger pin (this
branch). No other site in either scan was accepted by full coverage.

| Site (`internal/budget/budget.go`) | Comparator | Element | Can tie on distinct elements? |
|---|---|---|---|
| :178 `slices.SortedFunc(maps.Keys(counts), Key.compare)` | `cmp.Or(cmp.Compare(k.Package, o.Package), cmp.Compare(k.Code, o.Code))` | `Key{Package, Code string}` | No: both fields compared; map keys are distinct |
| :201 `slices.SortedFunc(maps.Keys(f.rows), Key.compare)` | same | same | No |
| :231 same | same | same | No |
| :247 same | same | same | No |

`Key.compare` is unexported and lower-case; it earns the fact under any method name, as designed.

**Accepted comparator sites that can tie: 0.** The sample is small (4 distinct sites), so the
stronger evidence for soundness is the case packages: `TestSoundness` joins the live verdicts with
200-run outputs across 72 functions, including 20 boundary probes, 14 of them built to tie.

## Rejected comparator sample (46 distinct sites, plus one generated group)

Every non-generated, non-test rejected comparator site in the scan, plus the 36-site generated
group counted once. Classes: **leak** (distinct elements can tie and the output follows map order),
**total** (total in practice through an invariant the analyzer can't see, so a false positive),
**other** (depends on a helper's behaviour I did not prove).

| Site | Sort | Class | Why |
|---|---|---|---|
| github.com/mgechev/revive@v1.15.0/formatter/friendly.go:125 | SortFunc | leak | ties on failure count |
| github.com/moby/buildkit@v0.25.0/util/progress/progress.go:165 | SortFunc | leak | pointers; equal timestamps tie |
| github.com/moby/buildkit@v0.25.0/util/flightcontrol/flightcontrol.go:330 | SortFunc | leak | same |
| github.com/netbirdio/netbird@v0.71.0/proxy/internal/debug/handler.go:273 | SortFunc | total | compares Domain, the map key; Error not compared |
| github.com/tinylib/msgp@v1.6.4/msgp/setof/generated.go:429 (and 35 more in the file) | SortFunc | total | `if a < b {-1} else {1}` over distinct ints; not a cmp.Compare shape |
| gonum.org/v1/gonum@v0.17.0/unit/unittype.go:203 | SortFunc | total | `a.String()` tie-break over unique dimensions |
| tailscale.com@v1.96.5/appc/conn25.go:60 | SortFunc | total | `a.ID()` over distinct node ids |
| github.com/!microsoft/hcsshim@v0.13.0/ext4/internal/compactext4/compact.go:988 | Slice | total | number, then name (the key) |
| …/compactext4/compact.go:1022 | Slice | total | same |
| github.com/anishathalye/porcupine@v1.2.0/visualization.go:124 | Slice | total | int64 keys ascending |
| github.com/beorn7/perks@v1.0.1/topk/topk.go:80 | Sort | leak | ties on count, then truncated to k |
| github.com/butuzov/mirror@v1.3.0/cmd/internal/mirror-table/main.go:48 | Slice | other | total only if `cleanSortKey` is injective on its inputs |
| github.com/containerd/containerd/v2@v2.1.4/internal/cri/setutils/set.go:219 | Sort | total | ordered keys (`cmp.Ordered` type parameter) |
| github.com/go-critic/go-critic@v0.14.3/linter/helpers.go:24 | Slice | total | checker names are unique |
| github.com/golang/protobuf@v1.5.4/protoc-gen-go/generator/generator.go:2439 | Sort | total | type names unique |
| github.com/gogo/protobuf@v1.3.2/protoc-gen-gogo/generator/generator.go:3080 | Sort | total | same |
| github.com/gogo/protobuf@v1.3.2/proto/text.go:779 | Sort | total | int32 ids |
| github.com/google/licenseclassifier/v2@v2.0.0/tools/identify_license/results/results.go:137 | Sort | total | file path is the map key |
| github.com/google/pprof@v0.0.0-20260604005048-7023385849c0/internal/graph/graph.go:1138 | Sort | other | weight, then printable node names, which may collide |
| github.com/klauspost/compress@v1.18.7/dict/builder.go:141 | Slice | leak | approximate grouping; not even a valid order |
| github.com/klauspost/compress@v1.18.7/zstd/dict.go:304 | Slice | leak | ties on count |
| github.com/lucasb-eyer/go-colorful@v1.3.0/sort.go:84 | Slice | leak | ties on the map value |
| github.com/planetscale/vtprotobuf@v0.6.1-0.20240319094008-0393e58bdf10/generator/features.go:36 | Slice | total | name is the key |
| github.com/spf13/afero@v1.15.0/mem/dirmap.go:24 | Sort | total | file names unique |
| github.com/netbirdio/netbird@v0.71.0/proxy/internal/proxy/servicemapping.go:101 | Slice | leak | by path length only |
| github.com/duh-rpc/duh-cli@v0.9.0/internal/fieldmap/lock.go:223 | Slice | total | number, then key |
| github.com/tinylib/msgp@v1.6.4/parse/inline.go:98 | Slice | total | complexity, then name |
| github.com/whyrusleeping/cbor-gen@v0.3.1/package.go:93 | Slice | total | PkgPath is the key |
| go.etcd.io/bbolt@v1.4.3/internal/freelist/hashmap.go:126 | Sort | total | page ids |
| go.etcd.io/bbolt@v1.4.3/tx.go:522 | Sort | total | pages keyed by id |
| go.opencensus.io@v0.24.0/tag/map.go:56 | Slice | total | key names unique |
| golang.org/x/telemetry@v0.0.0-20260625142307-59b4966ccb57/internal/configgen/main.go:442 | Sort | other | version comparison over a set of strings |
| golang.org/x/vuln@v1.5.0/internal/client/schema.go:65 | SliceStable | total | module path is the key |
| golang.org/x/vuln@v1.5.0/internal/vulncheck/witness.go:184 | SliceStable | total | position, then String |
| golang.org/x/text@v0.41.0/message/pipeline/extract.go:514 | Slice | total | package paths unique |
| gvisor.dev/gvisor@v0.0.0-20260224225140-573d5e7127a8/pkg/state/encode.go:797 | Slice | total | int ids |
| google.golang.org/protobuf@v1.36.11/internal/cmd/pbdump/pbdump.go:248 | Slice | total | field numbers |
| k8s.io/utils@v0.0.0-20241104100929-3ea5e8cea738/set/set.go:172 | Sort | total | ordered keys |
| k8s.io/apimachinery@v0.32.3/pkg/labels/selector.go:968 | Sort | total | label key is the map key |
| honnef.co/go/tools@v0.7.0/go/ir/html.go:107 | Slice | leak | by `Pos()`; objects at `token.NoPos` tie |
| honnef.co/go/tools@v0.7.0/go/loader/hash.go:60 | Slice | total | PkgPath unique |
| k8s.io/kube-openapi@v0.0.0-20241105132330-32ad38e42d3f/pkg/util/sets/string.go:174 | Sort | total | strings |
| tailscale.com@v1.96.5/ipn/ipnstate/ipnstate.go:194 | Slice | total | `NodePublic.Less` over distinct keys |
| tailscale.com@v1.96.5/net/netutil/routes.go:79 | Slice | total | bits, then address: the whole prefix |
| tailscale.com@v1.96.5/net/netcheck/netcheck.go:392 | Slice | leak | ties on latency, and on zero |
| gonum (3 test files: network/betweenness, hits, page) | SortFunc | not classified | test code |

Of the 46 classified: **leak 10 (22%)**, **total 33 (72%)**, **other 3 (7%)**. 32 of the 36 non-leak
sites use `sort.Slice`/`sort.Sort` with an index comparator or `Less`, which full coverage never
analyzes. Rewriting them to `slices.SortFunc` with `cmp.Compare` on every field would pass; on 8
the element type itself is ordered and `slices.Sort` alone would.

## What changes against tiger (module cache, unique positions)

| | Non-test | Test |
|---|---:|---:|
| tiger's TS-T02 | 4,858 | 1,691 |
| prototype | 4,850 | 1,687 |
| fire in tiger only | 93 | 15 |
| fire in prototype only | 85 | 11 |

**Removed (93):** 91 are the accepted collect-then-sort sites. 2
(`dev.gaijin.team/go/golib@v0.6.0/fields/dict.go:31`, `:47`) are scan artifacts: running the
prototype directly on that package reports both.

**Added (85), by cause and verdict:**

| Cause | Leak | False positive | Other | Notes |
|---|---:|---:|---:|---|
| Write to a slice element or outer field (tiger skips every non-identifier assignment target) | 18 | 32 | 2 | leaks: `keys[i] = k; i++` returned unsorted (emirpasic/gods `Keys()`, gosec `strings.Join(keys)`, pelletier/go-toml, opencensus, 5 gonum iterators…). FPs: the same idiom followed by a sort (20), writes at an index that is distinct by invariant such as `heat[i] = h[id]` (10), lazy init of an outer field (1), a per-entry write that calls `fmt.Sprintf` (1) |
| Carried state (item 4) | 5 | 8 | 1 | leaks: smithy-go CBOR `off += …encode(p[off:])`, ristretto `if len(m) <= n { break }; delete`, pprof and listpkgs groupings. FPs: map type aliasing (3), groupings sorted later (2), per-entry field read through the entry's own pointer (1), distinct-count via a `seen` map (1), chi grouping (1) |
| Iterator escape (item 3) | 8 | 7 | 3 | leaks: `slices.Collect(maps.Keys(…))` returned or iterated (revive, tailscale tka, testcontrol, policytest ×2, go-sdk), iterators returned as API (tailscale syspolicy ×2). FPs: `slices.Sorted` over a type-parameter key (3), `AppendSeq` then `slices.Sort` (3), `Collect` then `sort.Slice` (1) |
| Scan artifact | | | 1 | mongo-driver `options.go:39`: tiger fires on it when run directly |
| **Total** | **31** | **47** | **7** | |

The first row is a change beyond call 8's four items. Tiger's allowlist accepts
`x.f = e` and `s[i] = e` as "locals" because it only inspects identifier targets; that let
`keys[i] = k; i++` through, which is order-dependent whenever the slice isn't sorted afterwards.
The prototype refuses slice-element writes and accepts field writes only on the range value itself
(the entry each visit owns) or on a struct declared inside the loop.

## Trial pins

| Pin | tiger TS-T02 | prototype | Difference |
|---|---:|---:|---|
| querator @ 1fd1bb2 | 11 (3 in tests) | 7 (3 in tests) | −4: `internal/store/memory.go:1054, 1204, 1456, 1602`, all `sort.Strings` collect-then-sort |
| mono-repo git-server @ 321da03 | 17 (1 in tests) | 15 (1 in tests) | −2: `internal/graphapi/mux.go:65`, `internal/registry.go:154`, both `sort.Strings` |
| tiger (this branch) | 0 | 0 | none; 4 `SortedFunc(…, Key.compare)` and 17 `slices.Sorted` sites accepted |

No pin gained a finding. Two git-server sites keep firing because of their bodies, not their sorts:
`storage/memory/memory.go:194` and `:833` (the latter is the `oid` case: the loop calls
`ParseCommitInfo`).

The mono-repo clone needed `services/git-server/internal/generated/`, which is gitignored; it was
copied from `~/Development/mono-repo` (itself at 321da03). querator and mono-repo were cloned with
`git clone --shared` into scratch; the originals were only read.

## Soundness bugs found and fixed

The scratch prototype (`rewrites/proto-keycmp`) passes all of these; this prototype fires on all of
them. Each is a corpus case; the ones marked † also vary in the 200-run test.

| Bug in the scratch prototype | Corpus case | Case package |
|---|---|---|
| An appended element reads a counter, so every field is compared yet ranks follow visit order | `collectVisitRank` | `boundary.VisitRank` † |
| `slices.Sort` accepted on float elements; `sort.Float64s` accepted | `collectFloatsSlicesSort`, `collectFloatsSortFloat64s` | `gaps.SortedValues` † |
| `slices.Sorted(maps.Values(m))` accepted on float values | `sortedFloatValues` | |
| A tie-break that reads both arguments on one side (`cmp.Compare(a-b, b-a)`) counted as mirrored | `crossedPrefix` | |
| Carried state checked only in collect-loop `if` conditions, so `FirstThree` passed | `firstThree`, `visitIndex`, `weightedSum`, `groupByValue` | `gaps.FirstThree` †, `boundary.GroupByValue` † |
| Slice-element writes skipped (inherited from tiger) | `indexedKeys`, `lastInSlot`, `lastName` | |

Found while scanning, fixed, with corpus cases: carried-state reads through an alias of the written
map (`firstThreeThroughAlias`, now tracked by map type); the own-slot exemption applied to a
`maps.Values` range whose variable repeats (`valuesAreNotKeys`); a written field read through
another entry's pointer (`markUnderMarkedParent`); a field write on the entry that reads the key
(`ownerByKey`).

## Remaining false positives (expected, each a corpus case)

- Comparator held in a variable, or a plain function (`namedComparator`,
  `plainFunctionComparator`, `boundary.StoredComparator`).
- `sort.Slice` index comparators and `sort.Sort` with `Less` (`collectIDsSortSlice`,
  `collectUsersSortSort`, `boundary.IndexComparator`). This is 39 of 88 comparator sites in the
  cache.
- Uniqueness by invariant: the comparator covers the key but not the whole element (tsidp `S4`,
  netbird `handler.go:273`).
- A helper sort (`collectThenHelper`, gitwants `Sorted`), a sort of a sub-slice, conversions in the
  appended element (`string(h[:])`, gitwants `S6`).
- Type-parameter elements (`slices.Sorted(maps.Keys(m))` with `K cmp.Ordered`): K could be a float.
- From the item-4 and target changes: indexed collect followed by a sort, permutation writes at a
  distinct index, map-type aliasing (`unicode.Categories` has the written map's type).

## Known misses (pass and depend on order; corpus `knownmiss.go`, case `boundary.Invert`)

- Last writer wins: `inverse[v] = k` when two keys share a value.
- A map write whose value calls a function: `labels[k] = fmt.Sprint(v)` calls in map order.
- Not modelled: `golang.org/x/exp/maps.Keys` (returns an unsorted slice), `reflect.Value.MapKeys`,
  ranges over a type parameter constrained to a map.

## Size

| | Code lines | Comment lines |
|---|---:|---:|
| tiger `maporder.go` | 471 | 96 |
| prototype (`analyzer.go`, `allowlist.go`, `collect.go`, `coverage.go`, `iterators.go`) | 1,273 | 200 |
| of which full coverage (`coverage.go`) | 309 | 46 |

About 30 lines of the prototype are the `-sites` reporting the scan needs.

## Summary for the README

`fullcoverage` is a prototype of TS-T02 with call 8's four changes: collect-then-sort with plain
sorts, comparator sorts proven total by full coverage, a check on every `maps.Keys`/`Values`/`All`
result, and no reads of state an earlier visit wrote. The tests show it on 72 case functions (9
real sites, the maporder gaps, 20 boundary probes): no function it passes prints more than one
output in 200 runs, except one known miss it documents (last writer wins). Every boundary in the
design (one array byte, one field, `String()`, pointer, interface, float, bool and `time.Time`
leaves, asymmetric and calling tie-breaks) is rejected, and each rejected probe really does vary.
On the module cache (678 of 970 modules loaded), full coverage accepted 4 comparator sites, all in
tiger, none of which can tie; it rejected 84, of which a sample of 46 is 22% real leaks and 72%
total by an invariant it can't see, mostly `sort.Slice` comparators it never analyzes. The plain-sort
arm did the work: 91 sites accepted, including querator's 4 and git-server's 2, and no pin gained a
finding. The prototype also found 31 real leaks tiger misses and adds 47 false positives, mostly
from refusing `keys[i] = k; i++`, which tiger accepts unchecked. It is about 800 code lines larger
than tiger's maporder; full coverage itself is 309.
