# querator layer file

This experiment checks how much of querator breaks when every package has an entry in
`tiger.layers.yaml` (TS-X03). The ticket's rule was to build only if real code
already fits, and to stop and ask if most packages break.

Setup, run on 2026-10-14: a `git clone --shared` of querator at `2824f64` (main), with
tiger built from this branch. `tiger.layers.yaml` here is querator's layer
file: 14 entries for querator's 14 packages. `benchmarks` holds only `_test.go`
files, so it is not a package and needs no entry.

From the tiger repository root:

```sh
go build -o tigerbin ./cmd/tiger
git clone --shared ~/Development/querator <clone>
cp docs/features/ENG-328-enforce-package-layers-with-a-layer-file/experiments/querator-layers/tiger.layers.yaml <clone>/
./tigerbin check -C <clone> ./... | grep TS-X03
```

## Result

| Run | TS-X03 lines |
|---|---|
| Unmodified querator, 14 entries | 0. The file loads, no source file changed |
| Seven planted imports, listed below | 7: six imports the entry doesn't list, and one package with no entry |
| `"./gone": []` added to the file | 1: `tiger.layers.yaml:132:1: TS-X03: the "./gone" entry matches no package directory` |

All three runs exit 1, because querator has 342 findings from tiger's other
rules. None of those are TS-X03. The runs end `tiger: 342 blocking`, `tiger: 349
blocking` and `tiger: 343 blocking`.

The seven planted lines, as the final binary of this branch printed them:

```
internal/store/zz_planted.go:3:8: TS-X03: the "./internal/store" entry in tiger.layers.yaml doesn't list net, so this package can't import net/url — add "net" to it, or pass it in through an interface
internal/types/zz_planted.go:3:8: TS-X03: the "./internal/types" entry in tiger.layers.yaml doesn't list math/rand, so this package can't import math/rand/v2 — add "math/rand" to it, or pass it in through an interface
newpkg/n.go:1:1: TS-X03: package ./newpkg has no entry in tiger.layers.yaml — add it as a key listing the module packages, modules and IO or random subtrees (os, net, ...) it imports, or [] for none
proto/zz_planted.go:3:8: TS-X03: the "./proto" entry in tiger.layers.yaml doesn't list github.com/jackc/pgx/v5 — add it, or its module path, to the entry
service/zz_planted.go:3:8: TS-X03: the "./service" entry in tiger.layers.yaml doesn't list os, so this package can't import os/exec — add "os" to it, or pass it in through an interface
transport/auth/zz_planted.go:3:8: TS-X03: the "./transport/auth" entry in tiger.layers.yaml doesn't list ./internal/store — add it, or pass what you need in through an interface
transport/reply/zz_planted.go:3:8: TS-X03: the "./transport/reply" entry in tiger.layers.yaml doesn't list net, so this package can't import net/http — add "net" to it, or pass it in through an interface
```

The planted imports, each added in a new file:

| Package | Import | Line printed |
|---|---|---|
| `internal/store` | `net/url` | doesn't list net, so this package can't import net/url |
| `internal/types` | `math/rand/v2` | doesn't list math/rand, so this package can't import math/rand/v2 |
| `service` | `os/exec` | doesn't list os, so this package can't import os/exec |
| `transport/reply` | `net/http` | doesn't list net, so this package can't import net/http |
| `proto` | `github.com/jackc/pgx/v5` | doesn't list github.com/jackc/pgx/v5 |
| `transport/auth` | `internal/store` | doesn't list ./internal/store |
| `newpkg` (new) | `os` | package ./newpkg has no entry in tiger.layers.yaml |

A package with no entry gets one finding, at its package clause. Its imports
are not judged one by one, so the `os` import in `newpkg` has no line of its own.

The unmodified run is clean because the file was written from querator's own
imports. What the run shows is that the shape fits. Only `internal/store`
names `database`. Six packages name nothing impure: `service`, `internal`,
`internal/types`, `proto`, `transport/auth` (except `crypto/rand`) and
`transport/reply`. `cmd/querator-doc` names nothing impure either. The
packages that talk to the world each name the few standard library subtrees
they use: `transport`, the root client, `daemon`, `cmd/querator`,
`cmd/quickstart` and `docs`. Each of them is a plain entry.

## What imports can't see

`cmd/querator` imports `net/http/httptest` in non-test code. Its entry names
`net` for `net/http`, and naming `net` allows the whole subtree, so the layer
file passes it. A test server in a binary is a call for a reviewer to make, and
`tiger changed` (ENG-343) is the tool that reports a package that starts using one.

## Why tiger checks the file itself

depguard compares an import only against the nearest allow entry in sort
order, by text prefix. That cannot express exact matching. A module such as
`go.mongodb.org/mongo-driver`, which sorts between `go` and `go/ast`, denies
`go/ast`. A dotless module path such as `goapp` lets every in-module import
through. A directory named `v1.2` stops depguard from running at all. ADR-0029
lists the eight cases. tiger judges every import against the file with exact
matching.
