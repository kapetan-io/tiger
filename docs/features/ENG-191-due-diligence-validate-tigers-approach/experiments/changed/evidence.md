# changed: evidence on real code

`changed -base <parent> .` run on scratch clones (`git clone --shared`) of
querator and the mono-repo, prototype at this directory's state. Each run loads
both versions with go/packages and go/ssa, test files excluded, and takes
about 3 seconds on either repository.

Three filters for outside packages were measured. None uses a table.

- **package** (default): keep an outside callee if its package is `syscall`
  or imports it, directly or transitively.
- **function** (`-by-function`): keep it only if the function body statically
  reaches `syscall`.
- **mixed** (`-mixed`): free functions by body, methods and interfaces by
  package.

Each line is classed as signal (a reviewer would want to know) or noise.

## querator, package filter

| Commit | Lines | Signal | Noise |
|---|---|---|---|
| 1fd1bb2 | 0 | | |
| 1ac9f8a | 0 | | |
| 02d7fd1 | 2 | 0 | 2 |
| aeb014c | 3 | 2 | 1 |
| 682bbb3 | 0 | | |
| cd6255d | 0 | | |
| 3a846fb | 0 | | |
| 7188b09 | 0 | | |
| 38afd35 | 0 | | |
| 894f546 | 0 | | |
| c66f054 (extra, see 38afd35) | 1 | 1 | 0 |

The whole output for the ten commits:

```
# 02d7fd1
internal.Logical
  - github.com/kapetan-io/errors.Errorf: ProduceInternal calls Errorf    noise
  - github.com/kapetan-io/errors.New: ProduceInternal calls New          noise

# aeb014c
daemon.PostgresConfig
  + github.com/kapetan-io/errors.New: validate calls New                  noise
daemon.setupPartitionStorage
  + internal/store.NewPostgresPartitionStore: setupPartitionStorage calls NewPostgresPartitionStore   signal
daemon.setupQueueStorage
  + internal/store.NewPostgresQueues: setupQueueStorage calls NewPostgresQueues                       signal

# c66f054
internal.AuthCache
  + golang.org/x/sync/singleflight.Group: Authenticate calls Do           signal (cache now coalesces misses)
```

The other eight commits print nothing. That is correct for 7188b09
(`strings.EqualFold`) and 1ac9f8a. For the rest, the known change is real, but
it adds or removes a method on an edge that already existed, and the report
works one level up, at the edge:

- **38afd35.** The brief says this commit moved `UpdateLastUsed` from
  `AuthBackend.Authenticate` to `AuthCache.Authenticate`. The git history
  disagrees: `git log -S UpdateLastUsed -- internal/` shows the call was never
  in `auth_backend.go`. The commit moved it inside `AuthCache`, from a
  goroutine in `Authenticate` to the new `lastUsedLoop`. Same owner, same
  target (`internal/store.APIKeys`), so nothing is reported. That is right by
  design. The nearest commit about this, c66f054, reports singleflight only.
- **02d7fd1.** `Logical.ProduceDeadLetter` now waits on `ctx.Done`, but
  `Logical` already had a `context.Context` edge. The "about 11 functions" in
  the brief came from the reach study, which credits callers with their
  callees' calls. The real fix in this commit, deleting `ProduceInternal`,
  which wrote straight to `store.Partition` from outside the request loop,
  is also method-level: `Logical` keeps its `store.Partition` edge through
  `applyToPartitions`.
- **3a846fb.** All three backends already called `ksuid.KSUID.Next` from
  `Produce`, so the new calls from `Retry` add no edge.
- **894f546.** `Item.Key` went away, but the badger stores still call other
  `badger.Item` methods.

Changes inside an existing edge, counted by comparing the full edge lists
(`-edges`): 02d7fd1 6, 3a846fb 3, 7188b09 1, 38afd35 5, 894f546 3, c66f054 2,
the rest 0. A `~` section for these would have surfaced every known change
above, including `- ProduceInternal calls Produce` on `Logical → store.Partition`.

### Filters compared on querator

| Filter | Lines on the 10 commits | Noise | c66f054 singleflight | Edges in the module at 894f546 |
|---|---|---|---|---|
| package | 5 | 3, all `kapetan-io/errors` | kept | 868 |
| mixed | 2, aeb014c's two signals | 0 | kept | 567 |
| function | 2, aeb014c's two signals | 0 | lost | 286 |

The function filter loses real I/O, because the static walk stops at
interface calls and at function-valued package variables. At 894f546 it drops
`os.File` (`Close` reaches the close syscall through `poll.CloseFunc`, a
variable), `github.com/jackc/pgx/v5.Batch`, `net/http.Request`,
`net/http.NewRequestWithContext`, `log/slog.Logger`, `time.Now`,
`prometheus.Registry`, `os/signal.Notify`, and every `context` function.
The fixture test `TestFunctionFilter/file-close` reproduces the `os.File.Close`
loss.

The mixed filter drops only free functions: `fmt.*`, `kapetan-io/errors.*`,
`encoding/json.Marshal`, `context.Background/WithTimeout/WithValue`,
`time.Now`, `slog` constructors, `ksuid.New`, `badger.DefaultOptions`, and
`net.Pipe`. It keeps every type the module calls methods on (`os.File`,
`pgx.Batch`, `pgxpool.Pool`, `badger.DB`, `net/http.Server`), free functions
that do reach syscall (`badger.Open`, `net/http.Get`), and `context.Context`.

## git-server 321da03 vs its parent

The scratch clone needs `internal/generated`, which is gitignored. It was
built with `task proto-gen` inside the scratch clone (no tracked file
changed). The parent does not import it, so `git archive` of the parent
loads without it.

| Filter | Lines | Owned by generated code | Hand-written |
|---|---|---|---|
| package | 173 | 110 | 63 |
| mixed | 137 | 101 | 36 |
| function | 37 | 6 | 31 |

The commit adds a whole package (`internal/graphapi`, 54 of the 63
hand-written lines) and a generated DUH-RPC/protobuf package. Classification of
the 63 hand-written lines under the package filter:

| Kind | Lines | Class |
|---|---|---|
| wiring between module parts: `Daemon` → 3 graphapi constructors; `graphapi.Mux` and `transport.HTTPTransport` → `auth.ParseCredential`; `guard` → `auth.Enforcer`; `backendStore` → `storage.Backend`; graphapi → `generated.Handler`, `generated.NewHandler`, `generated.ServiceInterface` | 10 | signal |
| HTTP surface: `net/http.Header/Request/ResponseWriter/Flusher`, duh `Reply*`, `BytesWriter` | 10 | borderline, expected in a new HTTP package |
| `context.Context` (`Value`), `context.WithValue` | 2 | borderline |
| `fmt.Sprintf/Errorf/Fprintf` | 12 | noise |
| duh `NewServiceError*` in error constructors | 10 | noise |
| `encoding/json.Marshal/Unmarshal` | 4 | noise |
| module value-type helpers: `storage.OID.String`, `storage.ParseOID`, `storage.IsNotFound`, `object.TreeEntry.IsTree`, `object.Parse*` | 15 | noise |

The 110 generated lines are 30 protobuf message types × 3 lines to
`google.golang.org/protobuf/internal/impl`, 2 descriptor lines, and 18 lines
on the generated `Client` and `Handler`. All are noise to a reviewer.

The signal is there, `Daemon` starting the graph API and graphapi reaching
storage only through `storage.Backend` and auth through `auth.Enforcer`, but
it is 10 lines out of 173.

## Summary

`changed` lists, for each owner (a type, or a free function), the packages it
started or stopped calling between two versions. On ten querator commits it
printed 5 lines; 2 were what a reviewer needs (PostgreSQL wired into the
daemon), 3 were an error library that leaks through the import filter. Four
known changes printed nothing because each stays inside an owner-to-package
edge that already existed: a call moved between methods of one type, a new
`ctx.Done` wait, a new `ksuid` method, a removed `Item.Key` read.
That silence is by design and matches the brief ("I might not even want to
know if a method changed"), but it means a shortcut that adds a second
database call to a struct that already talks to the database stays invisible.
On git-server's graph API commit it printed 173 lines, 110 from generated code
and most of the rest from a brand-new package, with 10 lines of real wiring.
Filtering outside packages by "imports syscall" lets `fmt`, `context`, `time`
and error libraries through; judging free functions by their body and types
by their package removes them on both repositories without losing any I/O
type, still with no table.

## Recommendations

1. **context.Context: show it.** It appeared in 0 of 5 querator lines and 2
   of 173 git-server lines, because almost every owner already has a context
   edge. Hiding it needs a named exception, which is a table of one. Under
   the mixed filter the free functions (`context.WithValue`,
   `context.Background`) drop out anyway and only the interface stays.
2. **fmt and errors: use the mixed filter.** Package `errors` does not reach
   syscall and never leaked. `fmt` does (via `os`), and so does any
   third-party error library that imports `fmt`, which made up all of
   querator's noise. The function-only filter removes them but loses real
   I/O (`os.File.Close`, `pgx.Batch`, `http.Request`), so do not use it alone.
3. **Skip generated files.** 110 of git-server's 173 lines came from files
   carrying `// Code generated ... DO NOT EDIT.` (`ast.IsGenerated`). The
   prototype does not skip them yet.
4. **Collapse new owners and new packages.** A package that did not exist in
   the base should print as one line ("new package internal/graphapi: 34
   owners, 54 edges") with its edges to existing module packages listed, since
   those are the wiring a reviewer checks.
5. **Consider a `~` detail section** for methods added to or removed from an
   existing edge. It would have surfaced every known querator change, at 18
   changed edges over the 10 commits.
6. **Known blind spots, each pinned by a fixture test:** a write through
   `io.Writer` is silent because `io` does not import syscall; a channel send
   to a worker that owns the database is silent because a send is not a call;
   moving code from a method into an unexported free helper reports `-` on
   the method and `+` on the helper, since a free function is its own owner.
   A renamed type reads as all of its edges removed and re-added.
7. **Test files are excluded.** Including them would report every test
   helper's edges; the prototype loads with `Tests: false`.
