# Experiments

Two small Go modules prove what tiger's golangci-lint plugin does. Each holds
its own `go.mod`, so the repo's `go test ./...` and `tiger check ./...` skip
them. Each directory also commits the output of both commands (`test.out` and
`tiger.out`).

Both tests need the plugin and tiger built. Build the plugin with
`golangci-lint custom` from the repo root (v2.11.4) and tiger with
`go build ./cmd/tiger`. Then run either one from its directory:

```sh
TIGER_GCL=<path>/tiger-gcl TIGER=<path>/tiger go test -count=1 -v ./...
```

## newfromrev

Asks whether golangci-lint's `--new-from-rev`, which tiger's
`config/golangci.yml` tells adopters to use, hides tiger's own findings. Each
test builds a git repository with two commits, `old.go` on the main line and
`new.go` on the branch, each with the same TS-S09 violation, then runs
golangci-lint built with the tiger plugin and `tiger check`. ADR-0022 rests on
the result.

| Run | Findings | Exit |
|---|---|---|
| plugin, no flag | `old.go` and `new.go` | 1 |
| plugin, `--new-from-rev=HEAD~1` | `new.go` only; nothing says `old.go` was dropped | 1 |
| plugin, `--new-from-rev=HEAD` | none | 0 |
| plugin, `--new-from-rev=HEAD~1`, run through a symlinked path | none, not even `new.go` | 0 |
| `tiger check` | `old.go` and `new.go` | 1 |

`tiger.out` holds three findings in the test helpers, not in the fixture.

## nolint

Asks what `//nolint` does to tiger's findings under `tiger check` and under the
golangci-lint plugin, and whether an opt-in "no `//nolint`" finding reported at
the file's `package` line could survive the plugin. Each fixture under
`testdata/` is copied into a fresh module. ADR-0017 and ADR-0023 rest on the
result.

| Fixture | `tiger check` | plugin |
|---|---|---|
| `dropped`: `_ = os.Remove(path)` | TS-E02, exit 1 | not run |
| `droppednolint`: the same with a bare `//nolint` | passes, the comment counts as TS-E02's reason | not run |
| `line`: `//nolint:tiger` on a TS-S09 line | TS-S09, exit 1 | passes |
| `filewide`: `//nolint:tiger` in the package doc comment | TS-P02 at the `package` line and TS-S09 | passes, both dropped |
| `excluded`: an `exclusions` rule in `.golangci.yml`, no comment | TS-S09, exit 1 | passes |

So an opt-in "no `//nolint`" check can be enforced under `tiger check` only.
Under the plugin a file-wide `//nolint:tiger` drops a finding at the `package`
line, and an `exclusions` rule drops any finding without touching the code.

`tiger.out` holds three findings in the test helpers, not in the fixtures.
