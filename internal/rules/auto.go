// Auto rules: the half of the dialect enforced by off-the-shelf
// golangci-lint linters. Tiger never reimplements one — it audits the
// delegation. The tiger golangci verification and the --init generator are
// both derived from this table, so the generated baseline passes
// verification by construction.
package rules

import (
	"maps"
	"slices"
	"strings"
)

// ModuleToken marks the spot in a baseline value where the audited project's
// module path belongs. Generation and verification both substitute it, so
// module-dependent settings (the gci prefix) stay checkable.
const ModuleToken = "${module}"

// SectionLinters and SectionFormatters name the two golangci-lint v2 config
// sections a baseline entry can require its linter in.
const (
	SectionLinters    = "linters"
	SectionFormatters = "formatters"
)

// standardLinters is golangci-lint v2's "standard" default group. A linter
// in this group counts as enabled when the config keeps default: standard.
var standardLinters = []string{"errcheck", "govet", "ineffassign", "staticcheck", "unused"}

// Setting is one baseline expectation: the value at a path inside the
// project's golangci-lint configuration. Comparison semantics, per the
// blueprint: extra settings always pass; a scalar or scalar list must equal
// the baseline exactly (a stricter value also fails in v1 — direction-aware
// comparison is future work); a list of maps must contain each baseline
// element (extra elements pass).
type Setting struct {
	// Path walks the YAML document, for example
	// linters.settings.funlen.lines.
	Path []string
	// Want is the baseline expectation: a scalar, []any, or map[string]any.
	Want any
}

// AutoRule binds one specification rule to the golangci-lint linter and
// baseline settings that enforce it.
type AutoRule struct {
	RuleID   string
	Linter   string
	Section  string
	Title    string
	Settings []Setting
}

// autoRules is the auto half of the dialect, in specification order.
// Baseline values mirror config/golangci.yml, the Stage 0 template.
var autoRules = []AutoRule{
	{
		RuleID: "TS-S04", Linter: "funlen", Section: SectionLinters,
		Title: "hard limit of 70 lines per function",
		Settings: []Setting{
			{Path: []string{"linters", "settings", "funlen", "lines"}, Want: 70},
			{Path: []string{"linters", "settings", "funlen", "statements"}, Want: 45},
			{Path: []string{"linters", "settings", "funlen", "ignore-comments"}, Want: true},
		},
	},
	{
		RuleID: "TS-S05", Linter: "cyclop", Section: SectionLinters,
		Title: "cyclomatic complexity at most 10",
		Settings: []Setting{
			{Path: []string{"linters", "settings", "cyclop", "max-complexity"}, Want: 10},
			{Path: []string{"linters", "settings", "cyclop", "package-average"}, Want: 5.0},
		},
	},
	{
		RuleID: "TS-S05", Linter: "gocognit", Section: SectionLinters,
		Title: "cognitive complexity at most 15",
		Settings: []Setting{
			{Path: []string{"linters", "settings", "gocognit", "min-complexity"}, Want: 15},
		},
	},
	{
		RuleID: "TS-S05", Linter: "nestif", Section: SectionLinters,
		Title: "nesting depth at most 3",
		Settings: []Setting{
			{Path: []string{"linters", "settings", "nestif", "min-complexity"}, Want: 3},
		},
	},
	{
		RuleID: "TS-S08", Linter: "exhaustive", Section: SectionLinters,
		Title: "every switch over a closed set is exhaustive",
		Settings: []Setting{
			{
				Path: []string{"linters", "settings", "exhaustive", "default-signifies-exhaustive"},
				Want: false,
			},
			{
				Path: []string{"linters", "settings", "exhaustive", "check"},
				Want: []any{"switch", "map"},
			},
		},
	},
	{
		RuleID: "TS-S10", Linter: "gochecknoinits", Section: SectionLinters,
		Title: "no init() functions",
	},
	{
		RuleID: "TS-S11", Linter: "gochecknoglobals", Section: SectionLinters,
		Title: "no package-level mutable state",
	},
	{
		RuleID: "TS-S12", Linter: "depguard", Section: SectionLinters,
		Title: "no unsafe, no cgo, no reflection",
	},
	{
		RuleID: "TS-S13", Linter: "ineffassign", Section: SectionLinters,
		Title: "no dead assignments",
	},
	{
		RuleID: "TS-S13", Linter: "wastedassign", Section: SectionLinters,
		Title: "no wasted assignments",
	},
	{
		RuleID: "TS-S14", Linter: "govet", Section: SectionLinters,
		Title: "no shadowing",
		Settings: []Setting{
			{Path: []string{"linters", "settings", "govet", "enable-all"}, Want: true},
			{
				Path: []string{"linters", "settings", "govet", "settings", "shadow", "strict"},
				Want: true,
			},
		},
	},
	{
		RuleID: "TS-S15", Linter: "exhaustruct", Section: SectionLinters,
		Title: "struct literals name every field on config and wire types",
	},
	{
		RuleID: "TS-S16", Linter: "mnd", Section: SectionLinters,
		Title: "no magic numbers",
		Settings: []Setting{
			{
				Path: []string{"linters", "settings", "mnd", "checks"},
				Want: []any{"argument", "case", "condition", "operation", "return", "assign"},
			},
		},
	},
	{
		RuleID: "TS-S19", Linter: "gosec", Section: SectionLinters,
		Title: "integer conversions are checked",
		Settings: []Setting{
			{Path: []string{"linters", "settings", "gosec", "includes"}, Want: []any{"G115"}},
		},
	},
	{
		RuleID: "TS-S19", Linter: "unconvert", Section: SectionLinters,
		Title: "no pointless conversions",
	},
	{
		RuleID: "TS-S20", Linter: "forcetypeassert", Section: SectionLinters,
		Title: "type assertions are always checked",
	},
	{
		RuleID: "TS-S20", Linter: "errcheck", Section: SectionLinters,
		Title: "type assertions are checked via errcheck",
		Settings: []Setting{
			{
				Path: []string{"linters", "settings", "errcheck", "check-type-assertions"},
				Want: true,
			},
		},
	},
	{
		RuleID: "TS-E01", Linter: "errcheck", Section: SectionLinters,
		Title: "every error is handled",
		Settings: []Setting{
			{Path: []string{"linters", "settings", "errcheck", "check-blank"}, Want: true},
		},
	},
	{
		RuleID: "TS-E03", Linter: "errorlint", Section: SectionLinters,
		Title: "wrap once with %w, compare with errors.Is and errors.As",
	},
	{
		RuleID: "TS-E03", Linter: "wrapcheck", Section: SectionLinters,
		Title: "wrap errors crossing package boundaries",
	},
	{
		RuleID: "TS-E04", Linter: "errname", Section: SectionLinters,
		Title: "sentinel errors are ErrFoo, error types are FooError",
	},
	{
		RuleID: "TS-E04", Linter: "staticcheck", Section: SectionLinters,
		Title: "error strings are lowercase and unpunctuated",
		Settings: []Setting{
			{Path: []string{"linters", "settings", "staticcheck", "checks"}, Want: []any{"all"}},
		},
	},
	{
		RuleID: "TS-E05", Linter: "nilnil", Section: SectionLinters,
		Title: "never return both a nil value and a nil error",
	},
	{
		RuleID: "TS-E06", Linter: "ireturn", Section: SectionLinters,
		Title: "accept interfaces, return concrete types",
		Settings: []Setting{
			{
				Path: []string{"linters", "settings", "ireturn", "allow"},
				Want: []any{"error", "stdlib"},
			},
		},
	},
	{
		RuleID: "TS-E08", Linter: "nakedret", Section: SectionLinters,
		Title: "no naked returns",
		Settings: []Setting{
			{Path: []string{"linters", "settings", "nakedret", "max-func-lines"}, Want: 0},
		},
	},
	{
		RuleID: "TS-C07", Linter: "containedctx", Section: SectionLinters,
		Title: "context is never stored in a struct",
	},
	{
		RuleID: "TS-C07", Linter: "contextcheck", Section: SectionLinters,
		Title: "context propagates",
	},
	{
		RuleID: "TS-C07", Linter: "noctx", Section: SectionLinters,
		Title: "no HTTP without context",
	},
	{
		RuleID: "TS-C08", Linter: "forbidigo", Section: SectionLinters,
		Title: "no time.Sleep in production code",
		Settings: []Setting{
			{Path: []string{"linters", "settings", "forbidigo", "analyze-types"}, Want: true},
			{Path: []string{"linters", "settings", "forbidigo", "forbid"}, Want: []any{
				map[string]any{"pattern": `^time\.Sleep$`},
				map[string]any{"pattern": `^time\.Now$`},
				map[string]any{"pattern": `^time\.After$`},
				map[string]any{"pattern": `^panic$`},
			}},
		},
	},
	{
		RuleID: "TS-M01", Linter: "prealloc", Section: SectionLinters,
		Title: "preallocate with a known capacity",
	},
	{
		RuleID: "TS-M01", Linter: "makezero", Section: SectionLinters,
		Title: "no make-plus-append misuse",
	},
	{
		RuleID: "TS-M03", Linter: "gocritic", Section: SectionLinters,
		Title: "pass structs larger than 64 bytes by pointer",
		Settings: []Setting{
			{
				Path: []string{
					"linters",
					"settings",
					"gocritic",
					"settings",
					"hugeParam",
					"sizeThreshold",
				},
				Want: 64,
			},
			{
				Path: []string{
					"linters",
					"settings",
					"gocritic",
					"settings",
					"rangeValCopy",
					"sizeThreshold",
				},
				Want: 64,
			},
		},
	},
	{
		RuleID: "TS-M07", Linter: "perfsprint", Section: SectionLinters,
		Title: "strconv over fmt",
	},
	{
		RuleID: "TS-N01", Linter: "revive", Section: SectionLinters,
		Title: "MixedCaps; acronyms keep their case",
		Settings: []Setting{
			{Path: []string{"linters", "settings", "revive", "rules"}, Want: []any{
				map[string]any{"name": "var-naming"},
			}},
		},
	},
	{
		RuleID: "TS-N02", Linter: "varnamelen", Section: SectionLinters,
		Title: "no invented abbreviations",
		Settings: []Setting{
			{Path: []string{"linters", "settings", "varnamelen", "min-name-length"}, Want: 3},
		},
	},
	{
		RuleID: "TS-N10", Linter: "revive", Section: SectionLinters,
		Title: "no stutter",
		Settings: []Setting{
			{Path: []string{"linters", "settings", "revive", "rules"}, Want: []any{
				map[string]any{"name": "exported"},
			}},
		},
	},
	{
		RuleID: "TS-L01", Linter: "gofumpt", Section: SectionFormatters,
		Title: "gofmt is not negotiable; gofumpt on top",
		Settings: []Setting{
			{Path: []string{"formatters", "settings", "gofumpt", "extra-rules"}, Want: true},
		},
	},
	{
		RuleID: "TS-L02", Linter: "lll", Section: SectionLinters,
		Title: "lines at most 100 columns",
		Settings: []Setting{
			{Path: []string{"linters", "settings", "lll", "line-length"}, Want: 100},
			{Path: []string{"linters", "settings", "lll", "tab-width"}, Want: 4},
		},
	},
	{
		RuleID: "TS-L03", Linter: "decorder", Section: SectionLinters,
		Title: "declaration order is const, var, type, func",
		Settings: []Setting{
			{
				Path: []string{"linters", "settings", "decorder", "dec-order"},
				Want: []any{"const", "var", "type", "func"},
			},
			{
				Path: []string{"linters", "settings", "decorder", "disable-dec-order-check"},
				Want: false,
			},
			{
				Path: []string{"linters", "settings", "decorder", "disable-init-func-first-check"},
				Want: false,
			},
		},
	},
	{
		RuleID: "TS-L06", Linter: "godot", Section: SectionLinters,
		Title: "comments are sentences",
		Settings: []Setting{
			{Path: []string{"linters", "settings", "godot", "scope"}, Want: "declarations"},
			{Path: []string{"linters", "settings", "godot", "capital"}, Want: true},
		},
	},
	{
		RuleID: "TS-L07", Linter: "revive", Section: SectionLinters,
		Title: "every exported identifier has a doc comment",
		Settings: []Setting{
			{Path: []string{"linters", "settings", "revive", "rules"}, Want: []any{
				map[string]any{"name": "exported"},
				map[string]any{"name": "package-comments"},
			}},
		},
	},
	{
		RuleID: "TS-L09", Linter: "nolintlint", Section: SectionLinters,
		Title: "every suppression carries a reason",
		Settings: []Setting{
			{
				Path: []string{"linters", "settings", "nolintlint", "require-explanation"},
				Want: true,
			},
			{Path: []string{"linters", "settings", "nolintlint", "require-specific"}, Want: true},
			{Path: []string{"linters", "settings", "nolintlint", "allow-unused"}, Want: false},
		},
	},
	{
		RuleID: "TS-L11", Linter: "gci", Section: SectionFormatters,
		Title: "imports grouped standard, external, internal",
		Settings: []Setting{
			{
				Path: []string{"formatters", "settings", "gci", "sections"},
				Want: []any{"standard", "default", "prefix(" + ModuleToken + ")"},
			},
		},
	},
	{
		RuleID: "TS-T01", Linter: "forbidigo", Section: SectionLinters,
		Title: "time and randomness are injected",
	},
	{
		RuleID: "TS-D01", Linter: "depguard", Section: SectionLinters,
		Title: "the standard library, and nothing else, where restrictions permit",
	},
}

// AutoRules returns every registered auto rule.
func AutoRules() []AutoRule {
	listed := make([]AutoRule, len(autoRules))
	copy(listed, autoRules)
	return listed
}

// StandardLinter reports whether golangci-lint's "standard" default group
// already enables the linter.
func StandardLinter(name string) bool {
	return slices.Contains(standardLinters, name)
}

// substituteTask pairs one baseline fragment with the slot its replacement
// lands in.
type substituteTask struct {
	want any
	slot func(any)
}

// SubstituteModule replaces ModuleToken with the project's module path
// throughout a baseline value, walking nested fragments with an
// index-advancing worklist — containers are created up front and their
// elements filled through slots on later passes.
func SubstituteModule(want any, module string) any {
	var substituted any
	work := []substituteTask{{want: want, slot: func(v any) { substituted = v }}}
	for i := 0; i < len(work); i++ {
		work = substituteOne(work, i, module)
	}
	return substituted
}

// substituteOne resolves one task, appending child tasks for nested values.
func substituteOne(work []substituteTask, i int, module string) []substituteTask {
	task := work[i]
	switch typed := task.want.(type) {
	case string:
		task.slot(strings.ReplaceAll(typed, ModuleToken, module))
	case []any:
		replaced := make([]any, len(typed))
		task.slot(replaced)
		for at, element := range typed {
			work = append(work, substituteTask{
				want: element,
				slot: func(v any) { replaced[at] = v },
			})
		}
	case map[string]any:
		replaced := make(map[string]any, len(typed))
		task.slot(replaced)
		for _, key := range slices.Sorted(maps.Keys(typed)) {
			work = append(work, substituteTask{
				want: typed[key],
				slot: func(v any) { replaced[key] = v },
			})
		}
	default:
		task.slot(task.want)
	}
	return work
}
