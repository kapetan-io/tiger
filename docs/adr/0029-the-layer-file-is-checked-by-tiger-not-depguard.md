# 29. The layer file is checked by tiger, not by generated depguard rules

Date: 2026-10-14

## Status

Accepted

Amends [ADR-0024](0024-packages-declare-no-restrictions.md), whose consequences describe the
layer file as not built and layering as enforced only by hand-written depguard rules.

Amends [ADR-0010](0010-whole-program-rules-run-in-a-driver-called-finish-step.md): TS-X03's
directory checks run in the CLI, not in a finish step.

## Context

TS-X03 declares the import graph in `tiger.layers.yaml`: each package lists what it may import
beyond the pure standard library, as in-module packages (`./pkg`), module paths, and eight
standard library subtrees that leave the process or are nondeterministic (`net`, `os`,
`database`, `syscall`, `plugin`, `io/ioutil`, `crypto/rand`, `math/rand`). TS-D01, the pure
standard library and nothing else, is what the file allows a package that names nothing.

Generating depguard rules from the file was the other way to enforce it: `tiger golangci --init`
would write one depguard rule per entry, plus a catch-all for packages with no entry, and
`tiger golangci` would audit them. depguard v2.2.1 checks an import against only the single
nearest allow entry in sort order, by text prefix, and `$gostd` expands to bare directory names
such as `go`, `net` and `os`. Anything that sorts near one of those names changes the answer.
Eight cases broke the generated rules, each reproduced with a fresh golangci-lint cache:

1. A dotless module path that starts like a standard library directory (`goapp`, `gosvc`,
   `timeapp`) let every in-module import through.
2. Naming `go.opentelemetry.io/otel` and `go.opentelemetry.io/otel/metric` denied
   `go.opentelemetry.io/otel/propagation`.
3. Naming `go-micro.dev/v4` denied `go/ast` and `go/token`, and so did a module
   `go.example.com/app` naming `./b`.
4. Naming `go.mongodb.org/mongo-driver` denied `go/ast`.
5. A module named `net.example/lib` allowed `net/http`.
6. A directory with a dot in its name (`./api/v1.2`) stopped golangci-lint running every depguard
   rule, because its config loader splits the rule name on the dot.
7. A `[` or `{` in the module path, or a Windows path, broke the `${config-path}` globs, so every
   import fell to the catch-all rule.
8. `./API` and `./api` collapsed into one rule, because the config loader lowercases keys.

Each fix was one more refused input or one more generated entry, and the cause kept producing new
cases.

## Decision

TS-X03 is a custom rule. The `layers` analyzer judges every import in a package's non-test files
against that package's entry, exactly: an in-module name is one package, a module path covers
itself and paths below it on a `/` boundary, and one of the eight subtree names covers its
subtree the same way. An import path is the standard library when it is a directory under the
GOROOT that `go env GOROOT` reports in the run directory, the toolchain the driver loads packages
with, so a dotless third-party module brought in by `replace` or `go.work` is named like any other
module. The GOROOT built into the tiger binary is only a fallback, because a `-trimpath` build or a
removed toolchain leaves it wrong. A package with no entry is a finding at its package clause. A
nested module, a directory below the root with a `go.mod` of its own (as `go.work` joins), is not
part of this module: its packages need no entry, because they carry their own `go.mod` and layer
file, and a package here that imports one names its module path.

The drivers load `tiger.layers.yaml` from the run directory and install it, and the analyzer only
queries it (ADR-0002). `tiger check` also reports, from the module's directories, an entry that
matches no package directory and a package with no entry that the analyzer did not report: one
outside the patterns passed, or built only for another platform.
This is not an ADR-0010 finish step: a finish step sees only the packages the run loaded, and these
checks need the directories it skipped. `tiger check` runs them after analysis, so only the CLI
reports them.

TS-D01 folds into TS-X03: it is no longer an auto rule, and the `internal-domain` depguard block
leaves the golangci-lint template. depguard stays enabled for TS-S12's bans. TS-X03's analyzer
enforces TS-D01, and no finding prints TS-D01: every finding is a TS-X03 finding. This supersedes
ENG-328's wording about a TS-D01 category.

## Consequences

- Every case above is judged correctly. `TestLayersModulePathsMatchExactly` in
  `internal/check/layers_test.go` reproduces cases 1, 2, 3, 5 and 6, with the
  `go.opentelemetry.io` names standing in for case 4's `go.mongodb.org`, and
  `TestLayersKeysAreCaseSensitive` reproduces case 8. Case 7 is not tested: Go import paths can't
  hold `[` or `{`, and macOS can't hold a Windows path; tiger never builds a glob from a path.
- The golangci-lint template no longer limits `internal/domain` to the standard library. A module
  without a layer file has no import limits at all: TS-D01, and the half of TS-T01 that denies
  `math/rand` and `crypto/rand`, apply only when `tiger.layers.yaml` exists.
- Stock golangci-lint without the tiger plugin no longer limits imports per package; TS-X03 runs
  only in tiger and its plugin. Under the plugin the import findings and the missing-entry finding
  for loaded packages fire; stale entries and unloaded packages appear only in `tiger check`, which
  still fails the build.
- `tiger golangci` no longer generates or audits layer rules, so the layer file and the
  golangci-lint config cannot drift apart.
- golangci-lint's issue cache key is the sorted analyzer names, salted with the golangci-lint
  version string, the `linters.settings` block and `go.mod`. It includes neither the plugin binary
  nor `tiger.layers.yaml`, so an edited layer file could replay stale results. The plugin therefore
  adds an analyzer, `layerfile_<digest>`, that reports nothing and whose name carries the layer
  file's digest; an edit changes the cache key. `tiger.yaml` has the same gap, and this decision
  does not close it.
- Known limits: an import in a file built for another OS or behind a build tag is checked only on a
  platform that builds it. The eight subtrees are not all of the standard library's IO:
  `path/filepath.Glob`, `html/template.ParseFiles`, `log/syslog` and `crypto/tls` reach the file
  system or the network without importing one of them. `tiger check` never judges a package in a
  directory whose name ends in `.test` import by import, because its driver drops every package ID
  ending in `.test` as the main package `go test` synthesizes; it still reports the package when no
  entry covers it. The golangci-lint plugin does judge it, because the analyzer skips only a package
  named `main` whose path ends in `.test`. Packages with cgo files are not judged import by import;
  the missing-entry check still applies. The driver takes the compiled cgo variant of a hand-written
  file for generated code, a limit every analyzer shares, though a TS-X03 finding in a generated file
  is kept, because a generated file can import the IO the layer file keeps out. A directory whose
  only Go files carry `//go:build ignore` is not a package and needs no entry; an entry for one is
  reported as matching no package. A layer file with no keys is the same as no file. Each limit in
  how tiger decides, every one here but
  the list of subtrees and cgo, has a test that pins it, in `internal/check/layers_test.go` or, for
  the plugin's `.test` case, `plugin/plugin_test.go`.
