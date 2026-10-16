package cli_test

import (
	"bytes"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kapetan-io/tiger/internal/cli"
)

const layerModule = "layers.example/app"

// layerTree is a module with a store, a service that uses it, and a web
// package, laid out the way an adopter's repo would be.
var layerTree = map[string]string{
	"go.mod": "module " + layerModule + "\n\ngo 1.26\n",
	"store/store.go": "// Package store keeps the database handle.\npackage store\n\n" +
		"import \"database/sql\"\n\n// Handle wraps the database the store keeps.\n" +
		"type Handle struct {\n\tDB *sql.DB\n}\n",
	"service/service.go": "// Package service uses the store.\npackage service\n\n" +
		"import \"" + layerModule + "/store\"\n\n" +
		"// Ready reports whether the store is open.\n" +
		"func Ready(handle store.Handle) bool { return handle.DB != nil }\n",
	"web/web.go": "// Package web talks HTTP.\npackage web\n\nimport \"net/http\"\n\n" +
		"// Client is the web package's HTTP client.\n" +
		"type Client struct {\n\tHTTP http.Client\n}\n",
	"tiger.layers.yaml": "\"./store\":\n  - database\n" +
		"\"./service\":\n  - ./store\n" +
		"\"./web\":\n  - net\n",
}

// writeFiles writes new files under dir, creating directories as needed.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	require.NoError(t, os.CopyFS(dir, mapFiles(files)))
}

// writeModuleFiles writes files into a fresh directory.
func writeModuleFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, files)
	return dir
}

// checkModule runs tiger check over every package in dir.
func checkModule(t *testing.T, dir string) outcome {
	t.Helper()
	return run(t, "check", "-C", dir, "./...")
}

// TestLayersCleanModulePasses covers the enforcement contract.
//
// Goal: a module whose every import is listed passes with no output.
func TestLayersCleanModulePasses(t *testing.T) {
	got := checkModule(t, writeModuleFiles(t, layerTree))
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestLayersImportOutsideTheEntryFails covers the enforcement contract.
//
// Goal: an import the package's entry doesn't name fails at the import, in
// every package that makes it, and the listed imports stay silent.
func TestLayersImportOutsideTheEntryFails(t *testing.T) {
	dir := writeModuleFiles(t, layerTree)
	writeFiles(t, dir, map[string]string{
		"service/db.go": "package service\n\nimport \"database/sql\"\n\n" +
			"// Pool is a database the service shouldn't hold.\n" +
			"type Pool struct {\n\tDB *sql.DB\n}\n",
		"web/db.go": "package web\n\nimport \"database/sql\"\n\n" +
			"// Pool is a database web shouldn't hold.\ntype Pool struct {\n\tDB *sql.DB\n}\n",
	})

	got := checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Empty(t, got.stderr)
	assert.Equal(t, "service/db.go:3:8: TS-X03: the \"./service\" entry in tiger.layers.yaml "+
		"doesn't list database, so this package can't import database/sql — add \"database\" "+
		"to it, or pass it in through an interface\n"+
		"web/db.go:3:8: TS-X03: the \"./web\" entry in tiger.layers.yaml doesn't list "+
		"database, so this package can't import database/sql — add \"database\" to it, or "+
		"pass it in through an interface\n"+
		"tiger: 2 blocking\n", got.stdout)
}

// TestLayersUnlistedPackageFails covers the coverage contract.
//
// Goal: a package no entry covers fails at its package clause, whether it
// imports something or nothing.
func TestLayersUnlistedPackageFails(t *testing.T) {
	dir := writeModuleFiles(t, layerTree)
	writeFiles(t, dir, map[string]string{
		"worker/worker.go": "// Package worker reads its arguments.\npackage worker\n\n" +
			"import \"os\"\n\n// Args returns the worker's arguments.\n" +
			"func Args() []string { return os.Args }\n",
		"empty/empty.go": "// Package empty imports nothing.\npackage empty\n",
	})

	got := checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "empty/empty.go:2:1: TS-X03: package ./empty has no entry in "+
		"tiger.layers.yaml — add it as a key listing the module packages, modules and IO or "+
		"random subtrees (os, net, ...) it imports, or [] for none\n"+
		"worker/worker.go:2:1: TS-X03: package ./worker has no entry in tiger.layers.yaml — "+
		"add it as a key listing the module packages, modules and IO or random subtrees (os, "+
		"net, ...) it imports, or [] for none\n"+
		"tiger: 2 blocking\n", got.stdout)
}

// TestLayersStaleEntryFails covers the coverage contract.
//
// Goal: an entry that matches no package fails at the entry's line, so a
// renamed or deleted package can't leave its old allowance behind.
func TestLayersStaleEntryFails(t *testing.T) {
	dir := writeModuleFiles(t, layerTree)
	require.NoError(t, os.RemoveAll(filepath.Join(dir, "web")))

	got := checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "tiger.layers.yaml:5:1: TS-X03: the \"./web\" entry matches no package "+
		"directory — remove it, or fix its path\ntiger: 1 blocking\n", got.stdout)
}

// TestLayersInModuleNamesAreExact covers the name contract.
//
// Goal: naming ./store allows that one package, not a package below it and
// not a sibling whose name starts the same way.
func TestLayersInModuleNamesAreExact(t *testing.T) {
	files := maps.Clone(layerTree)
	maps.Copy(files, map[string]string{
		"store/sub/sub.go":    "// Package sub sits below store.\npackage sub\n",
		"storefront/front.go": "// Package storefront shares store's prefix.\npackage storefront\n",
		"service/more.go": "package service\n\nimport (\n" +
			"\t_ \"" + layerModule + "/store/sub\"\n\t_ \"" + layerModule + "/storefront\"\n)\n",
		"tiger.layers.yaml": "\"./store\":\n  - database\n\"./store/sub\": []\n" +
			"\"./storefront\": []\n\"./service\":\n  - ./store\n\"./web\":\n  - net\n",
	})

	got := checkModule(t, writeModuleFiles(t, files))
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "service/more.go:4:2: TS-X03: the \"./service\" entry in tiger.layers.yaml "+
		"doesn't list ./store/sub — add it, or pass what you need in through an interface\n"+
		"service/more.go:5:2: TS-X03: the \"./service\" entry in tiger.layers.yaml doesn't "+
		"list ./storefront — add it, or pass what you need in through an interface\n"+
		"tiger: 2 blocking\n", got.stdout)
}

// TestLayersImpureSubtreeMustBeNamed covers the impure subtree contract.
//
// Goal: a package whose entry names no subtree is denied every package in each of
// the eight subtrees that leave the process or are nondeterministic, and
// keeps the rest of the standard library, crypto/sha256 included.
func TestLayersImpureSubtreeMustBeNamed(t *testing.T) {
	dir := writeModuleFiles(t, layerTree)
	writeFiles(t, dir, map[string]string{
		"service/pure.go": "package service\n\nimport (\n" +
			"\t_ \"crypto/sha256\"\n\t_ \"encoding/json\"\n\t_ \"io\"\n\t_ \"math\"\n" +
			"\t_ \"path/filepath\"\n\t_ \"unsafe\"\n)\n",
	})
	got := checkModule(t, dir)
	require.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)

	writeFiles(t, dir, map[string]string{
		"service/io.go": "package service\n\nimport (\n" +
			"\t_ \"crypto/rand\"\n\t_ \"database/sql/driver\"\n\t_ \"io/ioutil\"\n" +
			"\t_ \"math/rand/v2\"\n\t_ \"net/http/httptest\"\n\t_ \"os/exec\"\n" +
			"\t_ \"plugin\"\n\t_ \"syscall\"\n)\n",
	})
	got = checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	for _, want := range []string{
		"service/io.go:4:2: TS-X03: the \"./service\" entry in tiger.layers.yaml doesn't " +
			"list crypto/rand, so this package can't import crypto/rand — add \"crypto/rand\"",
		"service/io.go:5:2: TS-X03: the \"./service\" entry in tiger.layers.yaml doesn't " +
			"list database, so this package can't import database/sql/driver — add \"database\"",
		"service/io.go:6:2: TS-X03: the \"./service\" entry in tiger.layers.yaml doesn't " +
			"list io/ioutil, so this package can't import io/ioutil — add \"io/ioutil\"",
		"service/io.go:7:2: TS-X03: the \"./service\" entry in tiger.layers.yaml doesn't " +
			"list math/rand, so this package can't import math/rand/v2 — add \"math/rand\"",
		"service/io.go:8:2: TS-X03: the \"./service\" entry in tiger.layers.yaml doesn't " +
			"list net, so this package can't import net/http/httptest — add \"net\"",
		"service/io.go:9:2: TS-X03: the \"./service\" entry in tiger.layers.yaml doesn't " +
			"list os, so this package can't import os/exec — add \"os\"",
		"service/io.go:10:2: TS-X03: the \"./service\" entry in tiger.layers.yaml doesn't " +
			"list plugin, so this package can't import plugin — add \"plugin\"",
		"service/io.go:11:2: TS-X03: the \"./service\" entry in tiger.layers.yaml doesn't " +
			"list syscall, so this package can't import syscall — add \"syscall\"",
	} {
		assert.Contains(t, got.stdout, want)
	}
	assert.Contains(t, got.stdout, "tiger: 8 blocking\n")
	assert.NotContains(t, got.stdout, "pure.go")
}

// TestLayersNamingNetAllowsOnlyItsSubtree covers the impure subtree contract.
//
// Goal: naming net allows net and everything below it, and nothing else.
func TestLayersNamingNetAllowsOnlyItsSubtree(t *testing.T) {
	dir := writeModuleFiles(t, layerTree)
	writeFiles(t, dir, map[string]string{
		"web/lifted.go": "package web\n\nimport (\n" +
			"\t_ \"net\"\n\t_ \"net/http/httptest\"\n\t_ \"net/netip\"\n)\n",
	})
	got := checkModule(t, dir)
	require.Equal(t, cli.ExitClean, got.code)

	writeFiles(t, dir, map[string]string{
		"web/os.go": "package web\n\nimport _ \"os\"\n",
	})
	got = checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "web/os.go:3:8: TS-X03: the \"./web\" entry in tiger.layers.yaml "+
		"doesn't list os, so this package can't import os — add \"os\" to it, or pass it in "+
		"through an interface\ntiger: 1 blocking\n", got.stdout)
}

// moduleWithDependencies adds local modules to a layered module through replace
// directives: example.com/a/b, with a package below it, and
// example.com/a/b-c, whose path only starts the same way, plus modules whose
// paths sort next to or start like standard library directories.
func moduleWithDependencies(module string) map[string]string {
	return map[string]string{
		"go.mod": "module " + module + "\n\ngo 1.26\n\nrequire (\n" +
			"\texample.com/a/b v0.0.0\n\texample.com/a/b-c v0.0.0\n" +
			"\tgo-micro.dev/v4 v4.0.0\n\tgo.opentelemetry.io/otel v0.0.0\n" +
			"\tnet.example/lib v0.0.0\n)\n\n" +
			"replace example.com/a/b => ./third/b\n\n" +
			"replace example.com/a/b-c => ./third/bc\n\n" +
			"replace go-micro.dev/v4 => ./third/micro\n\n" +
			"replace go.opentelemetry.io/otel => ./third/otel\n\n" +
			"replace net.example/lib => ./third/netlib\n",
		"third/b/go.mod":     "module example.com/a/b\n\ngo 1.26\n",
		"third/b/b.go":       "// Package b is the named module.\npackage b\n",
		"third/b/sub/sub.go": "// Package sub is below the named module.\npackage sub\n",
		"third/bc/go.mod":    "module example.com/a/b-c\n\ngo 1.26\n",
		"third/bc/bc.go":     "// Package bc is a sibling module.\npackage bc\n",
		"third/micro/go.mod": "module go-micro.dev/v4\n\ngo 1.26\n",
		"third/micro/m.go":   "// Package micro sorts between go and go/ast.\npackage micro\n",
		"third/otel/go.mod":  "module go.opentelemetry.io/otel\n\ngo 1.26\n",
		"third/otel/otel.go": "// Package otel is the module root.\npackage otel\n",
		"third/otel/metric/metric.go": "// Package metric is named on its own.\n" +
			"package metric\n",
		"third/otel/propagation/propagation.go": "// Package propagation is not named on " +
			"its own.\npackage propagation\n",
		"third/netlib/go.mod": "module net.example/lib\n\ngo 1.26\n",
		"third/netlib/lib.go": "// Package lib starts like net.\npackage lib\n",
	}
}

// TestLayersThirdPartyModuleMustBeNamed covers the name contract.
//
// Goal: naming a module allows its root and its packages, but not a sibling
// module whose path merely starts the same way, and a package that names no
// module may import none.
func TestLayersThirdPartyModuleMustBeNamed(t *testing.T) {
	files := moduleWithDependencies(layerModule)
	maps.Copy(files, map[string]string{
		"tiger.layers.yaml": "\"./app\":\n  - example.com/a/b\n\"./plain\": []\n",
		"app/app.go": "// Package app uses the named module.\npackage app\n\nimport (\n" +
			"\t_ \"example.com/a/b\"\n\t_ \"example.com/a/b/sub\"\n)\n",
		"plain/plain.go": "// Package plain names no module.\npackage plain\n",
	})
	dir := writeModuleFiles(t, files)
	got := checkModule(t, dir)
	require.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)

	writeFiles(t, dir, map[string]string{
		"app/sibling.go":  "package app\n\nimport _ \"example.com/a/b-c\"\n",
		"plain/module.go": "package plain\n\nimport _ \"example.com/a/b/sub\"\n",
	})
	got = checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "app/sibling.go:3:8: TS-X03: the \"./app\" entry in tiger.layers.yaml "+
		"doesn't list example.com/a/b-c — add it, or its module path, to the entry\n"+
		"plain/module.go:3:8: TS-X03: the \"./plain\" entry in tiger.layers.yaml doesn't list "+
		"example.com/a/b/sub — add it, or its module path, to the entry\n"+
		"tiger: 2 blocking\n", got.stdout)
}

// TestLayersModulePathsMatchExactly covers the name contract on module paths
// and names that a prefix match against the nearest entry would misjudge
// (ADR-0029).
//
// Goal: whatever the module path — dotless and starting like a standard
// library directory, or dotted and sorting next to one — and whatever the
// names — a module and a package below it both named, a module sorting
// between go and go/ast, a module starting like net, a directory with a dot
// or upper case — every in-module, third-party and standard library import
// is judged exactly.
func TestLayersModulePathsMatchExactly(t *testing.T) {
	for _, test := range []struct {
		name   string
		module string
	}{
		// ADR-0029 case 1: a dotless module path that starts like a standard
		// library directory.
		{name: "DotlessLikeGo", module: "goapp"},
		{name: "DotlessLikeGoAgain", module: "gosvc"},
		{name: "DotlessLikeTime", module: "timeapp"},
		// Not an ADR case: a dotless module path named like the otel names.
		{name: "DotlessOtel", module: "otel"},
		// ADR-0029 case 3: a dotted module path that sorts next to go/ast.
		{name: "DottedLikeGo", module: "go.example.com/app"},
		{name: "SortsBetweenGoAndGoAst", module: "go-micro.dev/app"},
		// ADR-0029 case 5: a module path that starts like net.
		{name: "StartsLikeNet", module: "net.example/app"},
	} {
		// Every row's layer file also reproduces ADR-0029 case 2 (the otel
		// names), case 3 (go-micro.dev/v4 beside go/ast and go/token), case 5
		// (net.example/lib beside net/http) and case 6 (./api/v1.2).
		t.Run(test.name, func(t *testing.T) {
			files := moduleWithDependencies(test.module)
			maps.Copy(files, map[string]string{
				"tiger.layers.yaml": "\"./api\":\n  - ./Model\n  - ./api/v1.2\n" +
					"  - example.com/a/b\n  - go-micro.dev/v4\n  - go.opentelemetry.io/otel\n" +
					"  - go.opentelemetry.io/otel/metric\n  - net.example/lib\n" +
					"\"./Model\": []\n\"./models\": []\n\"./api/v1.2\": []\n" +
					"\"./api/v1\": []\n\"./domain\": []\n\"./adapters/db\":\n  - os\n",
				"api/api.go": "// Package api uses its listed packages.\npackage api\n\n" +
					"import (\n\t_ \"go/ast\"\n\t_ \"go/token\"\n\t_ \"strings\"\n\n" +
					"\t_ \"example.com/a/b\"\n\t_ \"go-micro.dev/v4\"\n" +
					"\t_ \"go.opentelemetry.io/otel/metric\"\n" +
					"\t_ \"go.opentelemetry.io/otel/propagation\"\n\t_ \"net.example/lib\"\n\n" +
					"\t_ \"" + test.module + "/Model\"\n\t_ \"" + test.module + "/api/v1.2\"\n)\n",
				"Model/model.go":    "// Package model is upper case.\npackage model\n",
				"models/model.go":   "// Package models shares Model's prefix.\npackage models\n",
				"api/v1.2/v.go":     "// Package v has a dot in its directory.\npackage v\n",
				"api/v1/v.go":       "// Package v is a prefix of v1.2.\npackage v\n",
				"domain/domain.go":  "// Package domain imports nothing.\npackage domain\n",
				"adapters/db/db.go": "// Package db may do IO.\npackage db\n\nimport _ \"os\"\n",
			})
			dir := writeModuleFiles(t, files)
			got := checkModule(t, dir)
			require.Equal(t, cli.ExitClean, got.code)

			writeFiles(t, dir, map[string]string{
				"api/more.go": "package api\n\nimport (\n\t_ \"net/http\"\n\t_ \"os\"\n\n" +
					"\t_ \"example.com/a/b-c\"\n\n" +
					"\t_ \"" + test.module + "/models\"\n\t_ \"" + test.module + "/api/v1\"\n)\n",
				"domain/db.go": "package domain\n\nimport _ \"" + test.module + "/adapters/db\"\n",
			})
			got = checkModule(t, dir)
			assert.Equal(t, cli.ExitFindings, got.code)
			assert.Contains(t, got.stdout, "api/more.go:4:2: TS-X03: the \"./api\" entry in "+
				"tiger.layers.yaml doesn't list net, so this package can't import net/http")
			assert.Contains(t, got.stdout, "api/more.go:5:2: TS-X03: the \"./api\" entry in "+
				"tiger.layers.yaml doesn't list os, so this package can't import os")
			assert.Contains(t, got.stdout, "api/more.go:7:2: TS-X03: the \"./api\" entry in "+
				"tiger.layers.yaml doesn't list example.com/a/b-c")
			assert.Contains(t, got.stdout, "api/more.go:9:2: TS-X03: the \"./api\" entry in "+
				"tiger.layers.yaml doesn't list ./models")
			// The space after ./api/v1 keeps this from matching ./api/v1.2.
			assert.Contains(t, got.stdout, "api/more.go:10:2: TS-X03: the \"./api\" entry in "+
				"tiger.layers.yaml doesn't list ./api/v1 ")
			assert.Contains(t, got.stdout, "domain/db.go:3:8: TS-X03: the \"./domain\" entry in "+
				"tiger.layers.yaml doesn't list ./adapters/db")
			assert.Contains(t, got.stdout, "tiger: 6 blocking\n")
		})
	}
}

// TestLayersTestFilesAreNotLayered covers the test file contract.
//
// Goal: _test.go files, in the package or its external test package, import
// what they like, and a directory holding only tests needs no entry.
func TestLayersTestFilesAreNotLayered(t *testing.T) {
	dir := writeModuleFiles(t, layerTree)
	writeFiles(t, dir, map[string]string{
		"service/service_test.go": "package service\n\nimport (\n" +
			"\t\"net/http/httptest\"\n\t\"os\"\n\t\"testing\"\n\n" +
			"\t\"" + layerModule + "/store\"\n)\n\n" +
			"// TestReady proves a zero handle is not ready.\n" +
			"func TestReady(t *testing.T) {\n\t_ = httptest.NewRecorder()\n" +
			"\t_ = os.Args\n\t_ = Ready(store.Handle{})\n}\n",
		"service/external_test.go": "package service_test\n\nimport (\n" +
			"\t\"database/sql\"\n\t\"testing\"\n\n\t\"" + layerModule + "/web\"\n)\n\n" +
			"// TestHandle proves the test may reach any package.\n" +
			"func TestHandle(t *testing.T) {\n\tvar db *sql.DB\n\t_ = db\n\t_ = web.Client{}\n}\n",
		"e2e/e2e_test.go": "package e2e_test\n\nimport (\n" +
			"\t\"os\"\n\t\"testing\"\n)\n\n" +
			"// TestArgs proves a test-only directory needs no entry.\n" +
			"func TestArgs(t *testing.T) {\n\t_ = os.Args\n}\n",
	})

	got := checkModule(t, dir)
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestLayersWithoutAFileChangeNothing covers the opt-in contract.
//
// Goal: a module with no layer file is not layered: unlisted packages, IO
// and in-module imports all pass.
func TestLayersWithoutAFileChangeNothing(t *testing.T) {
	files := maps.Clone(layerTree)
	delete(files, "tiger.layers.yaml")
	maps.Copy(files, map[string]string{
		"worker/worker.go": "// Package worker does IO.\npackage worker\n\nimport (\n" +
			"\t_ \"os\"\n\t_ \"syscall\"\n\n\t_ \"" + layerModule + "/web\"\n)\n",
	})

	got := checkModule(t, writeModuleFiles(t, files))
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestLayersOtherPlatformPackageNeedsAnEntry covers the coverage contract.
//
// Goal: a package whose only files are built on other operating systems needs
// an entry, though this run can't load it; the finding sits at its first
// file, and its entry is not stale.
func TestLayersOtherPlatformPackageNeedsAnEntry(t *testing.T) {
	files := map[string]string{
		"go.mod":            "module " + layerModule + "\n\ngo 1.26\n",
		"lib/lib.go":        "// Package lib is pure.\npackage lib\n",
		"win/w_windows.go":  "// Package win runs on windows.\npackage win\n\nimport _ \"os\"\n",
		"win/w_plan9.go":    "package win\n\nimport _ \"os\"\n",
		"tiger.layers.yaml": "\"./lib\": []\n",
	}
	dir := writeModuleFiles(t, files)

	got := checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "win/w_plan9.go:1:1: TS-X03: package ./win has no entry in "+
		"tiger.layers.yaml — add it as a key listing the module packages, modules and IO or "+
		"random subtrees (os, net, ...) it imports, or [] for none\ntiger: 1 blocking\n",
		got.stdout)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "tiger.layers.yaml"),
		[]byte("\"./lib\": []\n\"./win\":\n  - os\n"), 0o644))
	got = checkModule(t, dir)
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
}

// TestLayersRootPackageEntry covers the key contract.
//
// Goal: the root package needs a "." entry like any other, and that entry
// governs only the root's own files.
func TestLayersRootPackageEntry(t *testing.T) {
	dir := writeModuleFiles(t, layerTree)
	writeFiles(t, dir, map[string]string{
		"main.go": "// Package app is the root.\npackage app\n\nimport _ \"" + layerModule +
			"/store\"\n",
	})
	got := checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "main.go:2:1: TS-X03: the root package \".\" has no entry in "+
		"tiger.layers.yaml — add it as a key listing the module packages, modules and IO or "+
		"random subtrees (os, net, ...) it imports, or [] for none\ntiger: 1 blocking\n",
		got.stdout)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "tiger.layers.yaml"),
		[]byte(layerTree["tiger.layers.yaml"]+"\".\":\n  - ./store\n"), 0o644))
	got = checkModule(t, dir)
	require.Equal(t, cli.ExitClean, got.code)

	writeFiles(t, dir, map[string]string{
		"io.go":        "package app\n\nimport _ \"os\"\n",
		"service/x.go": "package service\n\nimport _ \"" + layerModule + "\"\n",
	})
	got = checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "io.go:3:8: TS-X03: the \".\" entry in tiger.layers.yaml doesn't list os, "+
		"so this package can't import os — add \"os\" to it, or pass it in through an "+
		"interface\n"+
		"service/x.go:3:8: TS-X03: the \"./service\" entry in tiger.layers.yaml doesn't list "+
		"the root package \".\" — add it, or pass what you need in through an interface\n"+
		"tiger: 2 blocking\n", got.stdout)
}

// TestLayersSubtreeEntryCoversEverythingBelow covers the key contract.
//
// Goal: a "./dir/..." entry covers dir and every package below it, and
// still names in-module packages exactly, siblings in the subtree included.
func TestLayersSubtreeEntryCoversEverythingBelow(t *testing.T) {
	dir := writeModuleFiles(t, map[string]string{
		"go.mod":            "module " + layerModule + "\n\ngo 1.26\n",
		"domain/a/a.go":     "// Package a is pure.\npackage a\n",
		"domain/b/c/c.go":   "// Package c is pure.\npackage c\n",
		"tiger.layers.yaml": "\"./domain/...\": []\n",
	})
	got := checkModule(t, dir)
	require.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)

	writeFiles(t, dir, map[string]string{
		"domain/b/c/io.go": "package c\n\nimport (\n\t_ \"os\"\n\n\t_ \"" + layerModule +
			"/domain/a\"\n)\n",
	})
	got = checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "domain/b/c/io.go:4:2: TS-X03: the \"./domain/...\" entry in "+
		"tiger.layers.yaml doesn't list os, so this package can't import os — add \"os\" to "+
		"it, or pass it in through an interface\n"+
		"domain/b/c/io.go:6:2: TS-X03: the \"./domain/...\" entry in tiger.layers.yaml "+
		"doesn't list ./domain/a — add it, or pass what you need in through an interface\n"+
		"tiger: 2 blocking\n", got.stdout)
}

// TestLayersHiddenAndNestedDirectoriesAreNotPackages covers the coverage
// contract.
//
// Goal: directories the go command skips when listing every package need no
// entry and never make an entry stale: testdata, vendor, underscore and dot
// directories, a nested module, and a directory with only tests.
func TestLayersHiddenAndNestedDirectoriesAreNotPackages(t *testing.T) {
	dir := writeModuleFiles(t, map[string]string{
		"go.mod":              "module " + layerModule + "\n\ngo 1.26\n",
		"lib/lib.go":          "// Package lib is pure.\npackage lib\n",
		"lib/testdata/x.go":   "package x\n",
		"_scratch/s.go":       "package s\n",
		".hidden/h.go":        "package h\n",
		"other/go.mod":        "module other.example/m\n\ngo 1.26\n",
		"other/o.go":          "package o\n",
		"tiger.layers.yaml":   "\"./lib\": []\n",
		"onlytests/o_test.go": "package onlytests\n",
	})
	got := checkModule(t, dir)
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestLayersInvalidFileIsOperational covers the validation contract.
//
// Goal: a layer file tiger can't read, including keys that are not clean
// module-relative directories and empty names, exits 2 with one line naming
// the entry at fault, and no finding prints.
func TestLayersInvalidFileIsOperational(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		wantErr string
	}{
		{"KeyIsNotAPackage", "\"store\":\n  - database\n", "\"store\" isn't a package"},
		{"KeyEscapesTheModule", "\"./../x\":\n  - database\n", "isn't a package"},
		{"KeyTrailingSlash", "\"./store/\":\n  - database\n", "\"./store/\" isn't a package"},
		{"WholeModuleSubtree", "\"./...\":\n  - database\n", "\"./...\" isn't a package"},
		{"KeyNotClean", "\"./a/./b\":\n  - os\n", "\"./a/./b\" isn't a package"},
		{"KeyAbsolute", "\"/abs\":\n  - os\n", "\"/abs\" isn't a package"},
		{"KeyDoubleSlash", "\"./a//b\":\n  - os\n", "\"./a//b\" isn't a package"},
		{"KeyBareSubtree", "\"...\":\n  - os\n", "\"...\" isn't a package"},
		{"NameEmpty", "\"./web\":\n  - \"\"\n", "\"./web\": "},
		{
			"NameNotClean",
			"\"./web\":\n  - ./a//b\n",
			"\"./a//b\" isn't a package of this module; write it like \"./a/b\"",
		},
		{"StandardLibraryName", "\"./store\":\n  - strings\n", "\"strings\" is always allowed"},
		{
			"ImpureSubpackage",
			"\"./store\":\n  - net/http\n",
			"\"net/http\" is under net; name \"net\" to allow it",
		},
		{
			"PureCryptoPackage",
			"\"./web\":\n  - crypto/sha256\n",
			"\"crypto/sha256\" is always allowed; remove it",
		},
		{"NameListedTwice", "\"./store\":\n  - database\n  - database\n", "listed twice"},
		{"InModuleNameEscapes", "\"./web\":\n  - ./../x\n", "isn't a package of this module"},
		{"InModuleNameEmpty", "\"./web\":\n  - ./\n", "\"./\" isn't a package of this module"},
		{"InModuleNameNotClean", "\"./web\":\n  - ./store/../web\n", "isn't a package of this"},
		{
			"InModuleImportPath",
			"\"./web\":\n  - " + layerModule + "/store\n",
			"is this module; write it as \"./store\"",
		},
		{"OverlappingKeys", "\"./a/...\":\n  - os\n\"./a/b\":\n  - os\n", "both cover ./a/b"},
		{
			"NestedSubtrees",
			"\"./a/...\":\n  - os\n\"./a/b/...\":\n  - os\n",
			"\"./a/...\" and \"./a/b/...\" both cover ./a/b — remove \"./a/b/...\"",
		},
		{
			"DuplicateKey",
			"\"./a\":\n  - os\n\"./a\":\n  - net\n",
			"line 3: \"./a\" is already defined at line 1",
		},
		{
			"NotAMap",
			"- ./store\n",
			"line 1: expected entries like \"./store\": [database, ./internal/types]",
		},
		{
			"NameIsNotAList",
			"\"./store\": database\n",
			"line 1: expected entries like \"./store\": [database, ./internal/types]",
		},
		{
			"NameIsAMap",
			"\"./store\":\n  - ./web\n\"./web\":\n  os: true\n",
			"line 3: expected entries like \"./store\": [database, ./internal/types]",
		},
		{
			"SecondDocument",
			"\"./store\": []\n---\n\"./web\":\n  - os\n",
			"line 2: a second YAML document starts here; keep every entry in one map",
		},
		{
			"SecondDocumentDoesNotParse",
			"\".\": []\n---\n\"./a\": [os\n",
			"line 2: a second YAML document starts here; keep every entry in one map",
		},
		{
			"SecondDocumentIsNotYAML",
			"\".\": []\n---\nfoo: bar: baz\n",
			"line 2: a second YAML document starts here; keep every entry in one map",
		},
		{
			"SecondDocumentAfterExplicitStart",
			"# Layers.\n---\n\".\": []\n---\n\"./a\": [os\n",
			"line 4: a second YAML document starts here; keep every entry in one map",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := writeModuleFiles(t, map[string]string{
				"go.mod":            "module " + layerModule + "\n\ngo 1.26\n",
				"tiger.layers.yaml": test.content,
			})
			got := checkModule(t, dir)
			assert.Equal(t, cli.ExitOperational, got.code)
			assert.Empty(t, got.stdout)
			assert.Contains(t, got.stderr, "tiger check: tiger.layers.yaml: ")
			assert.Contains(t, got.stderr, test.wantErr)
			assert.Len(t, splitLines(got.stderr), 1)
		})
	}
}

// TestLayersCheckIsDeterministic covers the determinism contract.
//
// Goal: two runs over a module with import, unlisted and stale findings
// print byte-identical output.
func TestLayersCheckIsDeterministic(t *testing.T) {
	files := maps.Clone(layerTree)
	maps.Copy(files, map[string]string{
		"service/io.go": "package service\n\nimport (\n\t_ \"os\"\n\t_ \"net\"\n" +
			"\t_ \"syscall\"\n)\n",
		"web/db.go":        "package web\n\nimport _ \"database/sql\"\n",
		"worker/w.go":      "// Package worker has no entry.\npackage worker\n",
		"win/w_windows.go": "// Package win runs on windows.\npackage win\n",
		"tiger.layers.yaml": layerTree["tiger.layers.yaml"] + "\"./gone\": []\n" +
			"\"./old/...\": []\n",
	})
	dir := writeModuleFiles(t, files)

	first := checkModule(t, dir)
	second := checkModule(t, dir)
	require.Equal(t, cli.ExitFindings, first.code)
	assert.Equal(t, first, second)
	assert.Contains(t, first.stdout, "tiger: 8 blocking\n")
}

// TestLayersInvalidFileNamesTheEntry covers the validation contract.
//
// Goal: the one line a bad layer file exits with names the line and entry at
// fault and the fix.
func TestLayersInvalidFileNamesTheEntry(t *testing.T) {
	dir := writeModuleFiles(t, map[string]string{
		"go.mod":            "module " + layerModule + "\n\ngo 1.26\n",
		"tiger.layers.yaml": "\"./store\":\n  - strings\n",
	})
	got := checkModule(t, dir)
	assert.Equal(t, cli.ExitOperational, got.code)
	assert.Empty(t, got.stdout)
	assert.Equal(t, "tiger check: tiger.layers.yaml: line 1: \"./store\": \"strings\" is always "+
		"allowed; remove it\n", got.stderr)

	dir = writeModuleFiles(t, map[string]string{
		"go.mod":            "module " + layerModule + "\n\ngo 1.26\n",
		"tiger.layers.yaml": "\"./a/...\":\n  - os\n\"./a/b\":\n  - os\n",
	})
	got = checkModule(t, dir)
	assert.Equal(t, cli.ExitOperational, got.code)
	assert.Empty(t, got.stdout)
	assert.Equal(t, "tiger check: tiger.layers.yaml: line 3: \"./a/...\" and \"./a/b\" both cover "+
		"./a/b — remove \"./a/b\", or replace \"./a/...\" with one key per package\n",
		got.stderr)
}

// TestLayersSyntaxErrorsNameALineInTheFile covers the validation contract
// where the file is not YAML.
//
// Goal: a layer file the YAML parser refuses exits 2 with one line that reads
// like every other load error, "line N: ...", and N is never past the file's
// last line.
func TestLayersSyntaxErrorsNameALineInTheFile(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		wantErr string
	}{
		{"UnclosedList", "\"./a\": [\n", "line 1: did not find expected node content"},
		{"UnclosedListAtEnd", "\"./a\": [os", "line 1: did not find expected ',' or ']'"},
		{"UnterminatedString", "\"./a\": \"os\n", "line 1: found unexpected end of stream"},
		{
			"UnterminatedQuote",
			"\"./a\": []\n\"./b\": 'os\n",
			"line 2: found unexpected end of stream",
		},
		{"NoLineGiven", "foo: bar: baz\n", "mapping values are not allowed in this context"},
		{
			"NullDocument",
			"---\nnull\n",
			"line 2: expected entries like \"./store\": [database, ./internal/types]",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := checkModule(t, writeModuleFiles(t, map[string]string{
				"go.mod":            "module " + layerModule + "\n\ngo 1.26\n",
				"tiger.layers.yaml": test.content,
			}))
			assert.Equal(t, cli.ExitOperational, got.code)
			assert.Empty(t, got.stdout)
			assert.Equal(t, "tiger check: tiger.layers.yaml: "+test.wantErr+"\n", got.stderr)
		})
	}
}

// TestLayersFileWithoutGoModIsOperational covers the validation contract.
//
// Goal: a layer file with no go.mod beside it exits 2 with one line, because
// its keys have no module to resolve against.
func TestLayersFileWithoutGoModIsOperational(t *testing.T) {
	got := checkModule(t, writeModuleFiles(t, map[string]string{
		"tiger.layers.yaml": "\"./store\":\n  - database\n",
	}))
	assert.Equal(t, cli.ExitOperational, got.code)
	assert.Empty(t, got.stdout)
	assert.Contains(t, got.stderr, "tiger check: tiger.layers.yaml: ")
	assert.Len(t, splitLines(got.stderr), 1)
}

// TestLayersPackageWithTestsReportsOnce covers the enforcement and coverage
// contracts.
//
// Goal: a package the go command also loads as its test variant reports each
// import finding and its missing entry once, not once per variant.
func TestLayersPackageWithTestsReportsOnce(t *testing.T) {
	dir := writeModuleFiles(t, layerTree)
	writeFiles(t, dir, map[string]string{
		"service/db.go": "package service\n\nimport _ \"database/sql\"\n",
		"service/service_test.go": "package service\n\nimport \"testing\"\n\n" +
			"// TestNothing gives service a test variant.\nfunc TestNothing(t *testing.T) {}\n",
		"worker/worker.go": "// Package worker has no entry.\npackage worker\n",
		"worker/worker_test.go": "package worker\n\nimport \"testing\"\n\n" +
			"// TestNothing gives worker a test variant.\nfunc TestNothing(t *testing.T) {}\n",
		"worker/external_test.go": "package worker_test\n\nimport \"testing\"\n\n" +
			"// TestExternal gives worker an external test package.\n" +
			"func TestExternal(t *testing.T) {}\n",
	})

	got := checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "service/db.go:3:8: TS-X03: the \"./service\" entry in tiger.layers.yaml "+
		"doesn't list database, so this package can't import database/sql — add \"database\" "+
		"to it, or pass it in through an interface\n"+
		"worker/worker.go:2:1: TS-X03: package ./worker has no entry in tiger.layers.yaml — "+
		"add it as a key listing the module packages, modules and IO or random subtrees (os, "+
		"net, ...) it imports, or [] for none\n"+
		"tiger: 2 blocking\n", got.stdout)
}

// TestLayersDirectoryNamedLikeATestReportsOnce covers the coverage contract.
//
// Goal: a package whose directory name ends in _test is an ordinary package,
// even when it has tests of its own and so a test variant: with no entry it is
// reported once, at its package clause.
func TestLayersDirectoryNamedLikeATestReportsOnce(t *testing.T) {
	dir := writeModuleFiles(t, map[string]string{
		"go.mod":              "module " + layerModule + "\n\ngo 1.26\n",
		"lib/lib.go":          "// Package lib is pure.\npackage lib\n",
		"e2e_test/helpers.go": "// Package e2e holds shared test helpers.\npackage e2e\n",
		"e2e_test/helpers_test.go": "package e2e\n\nimport \"testing\"\n\n" +
			"// TestNothing gives e2e_test a test variant.\nfunc TestNothing(t *testing.T) {}\n",
		"tiger.layers.yaml": "\"./lib\": []\n",
	})

	got := checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "e2e_test/helpers.go:2:1: TS-X03: package ./e2e_test has no entry in "+
		"tiger.layers.yaml — add it as a key listing the module packages, modules and IO or "+
		"random subtrees (os, net, ...) it imports, or [] for none\ntiger: 1 blocking\n",
		got.stdout)
}

// TestLayersGeneratedPackageNeedsAnEntry covers the coverage contract.
//
// Goal: a package whose every file is generated still needs an entry, reported
// at its package clause.
func TestLayersGeneratedPackageNeedsAnEntry(t *testing.T) {
	dir := writeModuleFiles(t, map[string]string{
		"go.mod":     "module " + layerModule + "\n\ngo 1.26\n",
		"lib/lib.go": "// Package lib is pure.\npackage lib\n",
		"gen/gen.go": "// Code generated by protoc-gen-go. DO NOT EDIT.\n\n" +
			"// Package gen is generated.\npackage gen\n",
		"tiger.layers.yaml": "\"./lib\": []\n",
	})

	got := checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "gen/gen.go:4:1: TS-X03: package ./gen has no entry in tiger.layers.yaml — "+
		"add it as a key listing the module packages, modules and IO or random subtrees (os, "+
		"net, ...) it imports, or [] for none\ntiger: 1 blocking\n", got.stdout)
}

// TestLayersSkippedDirectoriesLeaveTheirEntriesStale covers the coverage
// contract.
//
// Goal: an entry naming a directory the go command skips, or one holding only
// tests, matches no package and is reported stale.
func TestLayersSkippedDirectoriesLeaveTheirEntriesStale(t *testing.T) {
	dir := writeModuleFiles(t, map[string]string{
		"go.mod":              "module " + layerModule + "\n\ngo 1.26\n",
		"lib/lib.go":          "// Package lib is pure.\npackage lib\n",
		"lib/testdata/x.go":   "package x\n",
		"_scratch/s.go":       "package s\n",
		".hidden/h.go":        "package h\n",
		"other/go.mod":        "module other.example/m\n\ngo 1.26\n",
		"other/o.go":          "package o\n",
		"onlytests/o_test.go": "package onlytests\n",
		"tiger.layers.yaml": "\"./lib\": []\n\"./lib/testdata\": []\n\"./_scratch\": []\n" +
			"\"./.hidden\": []\n\"./other\": []\n\"./onlytests\": []\n",
	})

	got := checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "tiger.layers.yaml:2:1: TS-X03: the \"./lib/testdata\" entry matches no "+
		"package directory — remove it, or fix its path\n"+
		"tiger.layers.yaml:3:1: TS-X03: the \"./_scratch\" entry matches no package directory "+
		"— remove it, or fix its path\n"+
		"tiger.layers.yaml:4:1: TS-X03: the \"./.hidden\" entry matches no package directory "+
		"— remove it, or fix its path\n"+
		"tiger.layers.yaml:5:1: TS-X03: the \"./other\" entry matches no package directory — "+
		"remove it, or fix its path\n"+
		"tiger.layers.yaml:6:1: TS-X03: the \"./onlytests\" entry matches no package directory "+
		"— remove it, or fix its path\n"+
		"tiger: 5 blocking\n", got.stdout)
}

// TestLayersSubtreeKeyStopsAtAPathBoundary covers the key contract.
//
// Goal: "./a/..." does not cover ./ab, so the two keys don't overlap and ./ab
// is judged by its own entry.
func TestLayersSubtreeKeyStopsAtAPathBoundary(t *testing.T) {
	dir := writeModuleFiles(t, map[string]string{
		"go.mod":            "module " + layerModule + "\n\ngo 1.26\n",
		"a/a.go":            "// Package a is pure.\npackage a\n",
		"a/x/x.go":          "// Package x reads arguments.\npackage x\n\nimport _ \"os\"\n",
		"ab/ab.go":          "// Package ab reads arguments.\npackage ab\n\nimport _ \"os\"\n",
		"tiger.layers.yaml": "\"./a/...\": []\n\"./ab\":\n  - os\n",
	})

	got := checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "a/x/x.go:4:8: TS-X03: the \"./a/...\" entry in tiger.layers.yaml doesn't "+
		"list os, so this package can't import os — add \"os\" to it, or pass it in through an "+
		"interface\ntiger: 1 blocking\n", got.stdout)
}

// TestLayersKeysAreCaseSensitive covers the key and name contracts.
//
// Goal: a key or name that differs from a directory only in case matches
// nothing, on a case-insensitive file system too: ./Model stays unlisted, the
// "./model" entry is stale, and naming ./model doesn't allow importing ./Model.
func TestLayersKeysAreCaseSensitive(t *testing.T) {
	dir := writeModuleFiles(t, map[string]string{
		"go.mod":            "module " + layerModule + "\n\ngo 1.26\n",
		"Model/model.go":    "// Package model is upper case.\npackage model\n",
		"tiger.layers.yaml": "\"./model\": []\n",
	})
	got := checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "Model/model.go:2:1: TS-X03: package ./Model has no entry in "+
		"tiger.layers.yaml — add it as a key listing the module packages, modules and IO or "+
		"random subtrees (os, net, ...) it imports, or [] for none\n"+
		"tiger.layers.yaml:1:1: TS-X03: the \"./model\" entry matches no package directory — "+
		"remove it, or fix its path\n"+
		"tiger: 2 blocking\n", got.stdout)

	writeFiles(t, dir, map[string]string{
		"api/api.go": "// Package api uses the model.\npackage api\n\nimport _ \"" + layerModule +
			"/Model\"\n",
	})
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tiger.layers.yaml"),
		[]byte("\"./Model\": []\n\"./api\":\n  - ./model\n"), 0o644))
	got = checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "api/api.go:4:8: TS-X03: the \"./api\" entry in tiger.layers.yaml doesn't "+
		"list ./Model — add it, or pass what you need in through an interface\n"+
		"tiger: 1 blocking\n", got.stdout)
}

// TestLayersModuleSharingThisModulesPrefixIsThirdParty covers the name
// contract.
//
// Goal: a dependency whose path only starts with this module's path is a
// third-party module, named by its own path, not a package of this module.
func TestLayersModuleSharingThisModulesPrefixIsThirdParty(t *testing.T) {
	dir := writeModuleFiles(t, map[string]string{
		"go.mod": "module " + layerModule + "\n\ngo 1.26\n\nrequire " + layerModule +
			"-c v0.0.0\n\nreplace " + layerModule + "-c => ./third/c\n",
		"third/c/go.mod": "module " + layerModule + "-c\n\ngo 1.26\n",
		"third/c/c.go":   "// Package c is a sibling module.\npackage c\n",
		"svc/svc.go": "// Package svc uses the sibling.\npackage svc\n\nimport _ \"" +
			layerModule + "-c\"\n",
		"plain/plain.go":    "// Package plain names nothing.\npackage plain\n",
		"tiger.layers.yaml": "\"./svc\":\n  - " + layerModule + "-c\n\"./plain\": []\n",
	})
	got := checkModule(t, dir)
	require.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)

	writeFiles(t, dir, map[string]string{
		"plain/c.go": "package plain\n\nimport _ \"" + layerModule + "-c\"\n",
	})
	got = checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "plain/c.go:3:8: TS-X03: the \"./plain\" entry in tiger.layers.yaml doesn't "+
		"list "+layerModule+"-c — add it, or its module path, to the entry\n"+
		"tiger: 1 blocking\n", got.stdout)
}

// TestLayersNarrowPatternStillReportsUnlistedPackages covers the coverage
// contract.
//
// Goal: checking one package still reports, from the layer file, every
// package elsewhere in the module that has no entry, since the run didn't
// load it.
func TestLayersNarrowPatternStillReportsUnlistedPackages(t *testing.T) {
	dir := writeModuleFiles(t, layerTree)
	writeFiles(t, dir, map[string]string{
		"worker/worker.go": "// Package worker has no entry.\npackage worker\n",
	})

	got := run(t, "check", "-C", dir, "./web")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "worker/worker.go:1:1: TS-X03: package ./worker has no entry in "+
		"tiger.layers.yaml — add it as a key listing the module packages, modules and IO or "+
		"random subtrees (os, net, ...) it imports, or [] for none\ntiger: 1 blocking\n",
		got.stdout)
}

// TestLayersModuleLineIsParsedLikeTheGoCommand covers the module path
// contract.
//
// Goal: a go.mod module line with a trailing comment or a quoted path still
// yields the module path, so in-module imports are judged and an unlisted one
// fails rather than every import passing as third party.
func TestLayersModuleLineIsParsedLikeTheGoCommand(t *testing.T) {
	for _, test := range []struct {
		name   string
		module string
	}{
		{name: "TrailingComment", module: "module " + layerModule + " // the app\n"},
		{name: "QuotedPath", module: "module \"" + layerModule + "\"\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := maps.Clone(layerTree)
			maps.Copy(files, map[string]string{
				"go.mod":       test.module + "\ngo 1.26\n",
				"web/store.go": "package web\n\nimport _ \"" + layerModule + "/store\"\n",
			})

			got := checkModule(t, writeModuleFiles(t, files))
			assert.Equal(t, cli.ExitFindings, got.code)
			assert.Equal(t, "web/store.go:3:8: TS-X03: the \"./web\" entry in tiger.layers.yaml "+
				"doesn't list ./store — add it, or pass what you need in through an interface\n"+
				"tiger: 1 blocking\n", got.stdout)
		})
	}
}

// dotlessModule is a layered module that brings in a third-party module whose
// path has no dot, the way a replace directive or go.work can.
func dotlessModule(layers string) map[string]string {
	return map[string]string{
		"go.mod": "module " + layerModule + "\n\ngo 1.26\n\nrequire goshared v0.0.0\n\n" +
			"replace goshared => ./third/goshared\n",
		"third/goshared/go.mod":    "module goshared\n\ngo 1.26\n",
		"third/goshared/shared.go": "// Package goshared is a dotless module.\npackage goshared\n",
		"app/app.go": "// Package app uses the dotless module.\npackage app\n\n" +
			"import _ \"goshared\"\n",
		"tiger.layers.yaml": layers,
	}
}

// TestLayersDotlessThirdPartyModuleMustBeNamed covers the name contract.
//
// Goal: a third-party module whose path has no dot is not the standard
// library: importing it needs its name in the entry, and naming it is
// accepted.
func TestLayersDotlessThirdPartyModuleMustBeNamed(t *testing.T) {
	got := checkModule(t, writeModuleFiles(t, dotlessModule("\"./app\": []\n")))
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "app/app.go:4:8: TS-X03: the \"./app\" entry in tiger.layers.yaml doesn't "+
		"list goshared — add it, or its module path, to the entry\ntiger: 1 blocking\n",
		got.stdout)

	got = checkModule(t, writeModuleFiles(t, dotlessModule("\"./app\":\n  - goshared\n")))
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestLayersSymlinkedDirectoryFindsItsPackages covers the coverage contract.
//
// Goal: tiger check -C through a symlink to the module finds the module's
// packages, so no entry reads as stale and an unloaded package with no entry
// is still reported.
func TestLayersSymlinkedDirectoryFindsItsPackages(t *testing.T) {
	dir := writeModuleFiles(t, layerTree)
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(dir, link))

	got := run(t, "check", "-C", link, "./...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)

	writeFiles(t, dir, map[string]string{
		"win/w_windows.go": "// Package win runs on windows.\npackage win\n",
	})
	got = run(t, "check", "-C", link, "./...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "win/w_windows.go:1:1: TS-X03: package ./win has no entry in "+
		"tiger.layers.yaml — add it as a key listing the module packages, modules and IO or "+
		"random subtrees (os, net, ...) it imports, or [] for none\ntiger: 1 blocking\n",
		got.stdout)
}

// TestLayersNamesThatNeverMatchAreRejected covers the validation contract.
//
// Goal: a name no import can ever match exits 2 with one line saying what to
// write instead.
func TestLayersNamesThatNeverMatchAreRejected(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		wantErr string
	}{
		{
			"InModuleSubtree",
			"\"./web\":\n  - ./b/...\n",
			"\"./web\": \"./b/...\" names a subtree, but a name is one package; list each " +
				"package, like \"./b\"",
		},
		{
			"ModuleTrailingSlash",
			"\"./web\":\n  - github.com/foo/\n",
			"\"./web\": \"github.com/foo/\" ends in a slash; write \"github.com/foo\"",
		},
		{
			"ModuleSubtree",
			"\"./web\":\n  - github.com/foo/...\n",
			"\"./web\": \"github.com/foo/...\": a module path already covers every path below " +
				"it; write \"github.com/foo\"",
		},
		{
			"InModuleDoubleSlash",
			"\"./web\":\n  - .//etc\n",
			"\"./web\": \".//etc\" isn't a package of this module; write it like \"./etc\"",
		},
		{
			"InModuleTrailingSlash",
			"\"./web\":\n  - ./store/\n",
			"\"./web\": \"./store/\" isn't a package of this module; write it like \"./store\"",
		},
		{
			"Cgo",
			"\"./web\":\n  - C\n",
			"\"./web\": \"C\" is always allowed; remove it",
		},
		{
			"NotAnImportPath",
			"\"./web\":\n  - github.com//foo\n",
			"\"./web\": \"github.com//foo\" isn't an import path; write a module path like " +
				"\"example.com/lib\"",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := writeModuleFiles(t, map[string]string{
				"go.mod":            "module " + layerModule + "\n\ngo 1.26\n",
				"tiger.layers.yaml": test.content,
			})
			got := checkModule(t, dir)
			assert.Equal(t, cli.ExitOperational, got.code)
			assert.Empty(t, got.stdout)
			assert.Equal(t, "tiger check: tiger.layers.yaml: line 1: "+test.wantErr+"\n",
				got.stderr)
		})
	}
}

// TestLayersPackageLoadedOnlyThroughItsTestsNeedsAnEntry covers the coverage
// contract.
//
// Goal: a package whose sources build only on another operating system, so
// this run loads nothing but its tests, still needs an entry and is reported
// once at its first source file, whether its test is internal or external.
func TestLayersPackageLoadedOnlyThroughItsTestsNeedsAnEntry(t *testing.T) {
	for _, test := range []struct {
		name string
		test string
	}{
		{
			name: "InternalTest",
			test: "package win\n\nimport \"testing\"\n\n" +
				"// TestNothing runs here though the package does not.\n" +
				"func TestNothing(t *testing.T) {}\n",
		},
		{
			name: "ExternalTest",
			test: "package win_test\n\nimport \"testing\"\n\n" +
				"// TestNothing runs here though the package does not.\n" +
				"func TestNothing(t *testing.T) {}\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := writeModuleFiles(t, map[string]string{
				"go.mod":            "module " + layerModule + "\n\ngo 1.26\n",
				"lib/lib.go":        "// Package lib is pure.\npackage lib\n",
				"win/w_windows.go":  "// Package win runs on windows.\npackage win\n",
				"win/w_test.go":     test.test,
				"tiger.layers.yaml": "\"./lib\": []\n",
			})

			got := checkModule(t, dir)
			assert.Equal(t, cli.ExitFindings, got.code)
			assert.Equal(t, "win/w_windows.go:1:1: TS-X03: package ./win has no entry in "+
				"tiger.layers.yaml — add it as a key listing the module packages, modules and IO "+
				"or random subtrees (os, net, ...) it imports, or [] for none\n"+
				"tiger: 1 blocking\n", got.stdout)
		})
	}
}

// TestLayersFindingsSortWithTheRest covers the output contract.
//
// Goal: the lines tiger check adds from the module's directories, a stale
// entry and an unloaded package, print in position order among the
// analyzers' findings, not after them.
func TestLayersFindingsSortWithTheRest(t *testing.T) {
	files := maps.Clone(layerTree)
	maps.Copy(files, map[string]string{
		"web/db.go":         "package web\n\nimport _ \"database/sql\"\n",
		"aaa/a_windows.go":  "// Package aaa runs on windows.\npackage aaa\n",
		"tiger.layers.yaml": "\"./gone\": []\n" + layerTree["tiger.layers.yaml"],
	})

	got := checkModule(t, writeModuleFiles(t, files))
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "aaa/a_windows.go:1:1: TS-X03: package ./aaa has no entry in "+
		"tiger.layers.yaml — add it as a key listing the module packages, modules and IO or "+
		"random subtrees (os, net, ...) it imports, or [] for none\n"+
		"tiger.layers.yaml:1:1: TS-X03: the \"./gone\" entry matches no package directory — "+
		"remove it, or fix its path\n"+
		"web/db.go:3:8: TS-X03: the \"./web\" entry in tiger.layers.yaml doesn't list database, "+
		"so this package can't import database/sql — add \"database\" to it, or pass it in "+
		"through an interface\n"+
		"tiger: 3 blocking\n", got.stdout)
}

// TestLayersStandardLibraryFoundWithoutTheBuildGoroot covers the standard
// library contract for a tiger binary built with -trimpath whose own GOROOT is
// gone, as when the toolchain that built it was removed.
//
// Goal: fmt, strings and os are judged standard library through the go
// command's GOROOT, so a module whose every import is listed stays clean.
func TestLayersStandardLibraryFoundWithoutTheBuildGoroot(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "tiger")
	out, err := exec.Command("go", "build", "-trimpath",
		"-ldflags=-X=runtime.defaultGOROOT="+filepath.Join(t.TempDir(), "gone"),
		"-o", bin, "github.com/kapetan-io/tiger/cmd/tiger").CombinedOutput()
	assert.Empty(t, string(out))
	require.NoError(t, err)
	dir := writeModuleFiles(t, map[string]string{
		"go.mod": "module " + layerModule + "\n\ngo 1.26\n",
		"m.go":   "// Package app prints.\npackage app\n\nimport _ \"fmt\"\n",
		"store/store.go": "// Package store reads the environment.\npackage store\n\nimport (\n" +
			"\t_ \"os\"\n\t_ \"strings\"\n)\n",
		"tiger.layers.yaml": "\".\": []\n\"./store\":\n  - os\n",
	})

	check := exec.Command(bin, "check", "-C", dir, "./...")
	// A GOROOT in the environment would mask the binary's missing one.
	check.Env = slices.DeleteFunc(os.Environ(), func(variable string) bool {
		return strings.HasPrefix(variable, "GOROOT=")
	})
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	check.Stdout, check.Stderr = stdout, stderr
	require.NoError(t, check.Run())
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
}

// TestLayersDirectoryNamedGoModIsNotAModule covers the coverage contract.
//
// Goal: a directory that is named go.mod is not a nested module's go.mod, so
// the directory holding it and the directory itself stay packages of this
// module, and their entries are not stale.
func TestLayersDirectoryNamedGoModIsNotAModule(t *testing.T) {
	dir := writeModuleFiles(t, map[string]string{
		"go.mod":              "module " + layerModule + "\n\ngo 1.26\n",
		"weird/weird.go":      "// Package weird holds a directory named go.mod.\npackage weird\n",
		"weird/go.mod/mod.go": "// Package mod sits in a directory named go.mod.\npackage mod\n",
		"tiger.layers.yaml":   "\"./weird\": []\n\"./weird/go.mod\": []\n",
	})

	got := checkModule(t, dir)
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// nestedModule is a layered module whose ./nested directory is a module of its
// own in the same workspace, the way go.work joins them.
func nestedModule(layers string) map[string]string {
	return map[string]string{
		"go.mod":        "module " + layerModule + "\n\ngo 1.26\n",
		"go.work":       "go 1.26\n\nuse (\n\t.\n\t./nested\n)\n",
		"nested/go.mod": "module " + layerModule + "/nested\n\ngo 1.26\n",
		"nested/nested.go": "// Package nested is a module of its own.\npackage nested\n\n" +
			"import _ \"os\"\n",
		"app/app.go": "// Package app uses the nested module.\npackage app\n\nimport _ \"" +
			layerModule + "/nested\"\n",
		"tiger.layers.yaml": layers,
	}
}

// TestLayersNestedModuleIsNotPartOfThisModule covers the module contract.
//
// Goal: a nested module's packages carry their own go.mod and layer file, so
// even when a pattern loads them they need no entry here, and a package of this
// module that imports one names it by its module path like any other module.
func TestLayersNestedModuleIsNotPartOfThisModule(t *testing.T) {
	dir := writeModuleFiles(t, nestedModule("\"./app\":\n  - "+layerModule+"/nested\n"))
	got := run(t, "check", "-C", dir, layerModule+"/...")
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)

	dir = writeModuleFiles(t, nestedModule("\"./app\": []\n"))
	got = run(t, "check", "-C", dir, layerModule+"/...")
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "app/app.go:4:8: TS-X03: the \"./app\" entry in tiger.layers.yaml doesn't "+
		"list "+layerModule+"/nested — add it, or its module path, to the entry\n"+
		"tiger: 1 blocking\n", got.stdout)

	dir = writeModuleFiles(t, nestedModule("\"./app\":\n  - ./nested\n"))
	got = run(t, "check", "-C", dir, layerModule+"/...")
	assert.Equal(t, cli.ExitOperational, got.code)
	assert.Equal(t, "tiger check: tiger.layers.yaml: line 1: \"./app\": \"./nested\" is in the "+
		"nested module "+layerModule+"/nested; name that module path\n", got.stderr)
}

// TestLayersNestedModuleAboveThisModulesPathIsIgnored covers the module
// contract where module paths overlap.
//
// Goal: a nested module whose path is a prefix of this module's path owns
// none of this module's packages, because the longest matching module path
// wins, so every package here is still judged and its names still validate.
func TestLayersNestedModuleAboveThisModulesPathIsIgnored(t *testing.T) {
	files := map[string]string{
		"go.mod":        "module example.com/m/v2\n\ngo 1.26\n",
		"legacy/go.mod": "module example.com/m\n\ngo 1.26\n",
		"legacy/legacy.go": "// Package legacy is the v1 module.\npackage legacy\n\n" +
			"import _ \"os\"\n",
		"store/store.go":    "// Package store reads files.\npackage store\n\nimport _ \"os\"\n",
		"tiger.layers.yaml": "\"./store\": []\n",
	}
	got := checkModule(t, writeModuleFiles(t, files))
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "store/store.go:4:8: TS-X03: the \"./store\" entry in tiger.layers.yaml "+
		"doesn't list os, so this package can't import os — add \"os\" to it, or pass it in "+
		"through an interface\ntiger: 1 blocking\n", got.stdout)

	files["service/service.go"] = "// Package service uses the store.\npackage service\n\n" +
		"import _ \"example.com/m/v2/store\"\n"
	files["tiger.layers.yaml"] = "\"./store\":\n  - os\n\"./service\":\n  - ./store\n"
	got = checkModule(t, writeModuleFiles(t, files))
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
	assert.Empty(t, got.stderr)
}

// TestLayersEmptyFileIsNoFile covers the opt-in contract.
//
// Goal: a layer file with no keys, written as {} or holding only comments,
// layers nothing, exactly as if there were no file.
func TestLayersEmptyFileIsNoFile(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
	}{
		{name: "EmptyMap", content: "{}\n"},
		{name: "CommentsOnly", content: "# Layers come later.\n"},
		{name: "DocumentStartOnly", content: "---\n"},
		{name: "DocumentStartAndComments", content: "---\n# \"./worker\":\n#   - os\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := checkModule(t, writeModuleFiles(t, map[string]string{
				"go.mod": "module " + layerModule + "\n\ngo 1.26\n",
				"worker/worker.go": "// Package worker does IO.\npackage worker\n\nimport (\n" +
					"\t_ \"os\"\n\n\t_ \"" + layerModule + "/lib\"\n)\n",
				"lib/lib.go":        "// Package lib is pure.\npackage lib\n",
				"tiger.layers.yaml": test.content,
			}))
			assert.Equal(t, cli.ExitClean, got.code)
			assert.Empty(t, got.stdout)
			assert.Empty(t, got.stderr)
		})
	}
}

// TestLayersIgnoredOnlyDirectoryNeedsNoEntry covers the coverage contract
// for generator scripts that no build includes.
//
// Goal: a directory whose only Go file sits behind //go:build ignore is never
// built, so it is not a package and needs no entry, while one that also holds
// an unconstrained file is a package again.
func TestLayersIgnoredOnlyDirectoryNeedsNoEntry(t *testing.T) {
	dir := writeModuleFiles(t, map[string]string{
		"go.mod":     "module " + layerModule + "\n\ngo 1.26\n",
		"lib/lib.go": "// Package lib is pure.\npackage lib\n",
		"tools/gen.go": "//go:build ignore\n\n// Package main generates code.\npackage main\n\n" +
			"func main() {}\n",
		"tiger.layers.yaml": "\"./lib\": []\n",
	})
	got := checkModule(t, dir)
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)

	// Control case, so the exemption is the ignore tag and not the directory.
	writeFiles(t, dir, map[string]string{
		"tools/real.go": "package main\n",
	})
	got = checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Contains(t, got.stdout, "TS-X03: package ./tools has no entry")
}

// TestLayersLegacyPlusBuildIgnoreNeedsNoEntry covers the coverage contract
// for generator scripts written before //go:build existed.
//
// Goal: a directory whose only Go file carries // +build ignore, which the go
// command still honors, needs no entry.
func TestLayersLegacyPlusBuildIgnoreNeedsNoEntry(t *testing.T) {
	got := checkModule(t, writeModuleFiles(t, map[string]string{
		"go.mod":            "module " + layerModule + "\n\ngo 1.26\n",
		"lib/lib.go":        "// Package lib is pure.\npackage lib\n",
		"tools/gen.go":      "// +build ignore\n\npackage main\n\nfunc main() {}\n",
		"tiger.layers.yaml": "\"./lib\": []\n",
	}))
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
}

// TestLayersEntryForIgnoredOnlyDirectoryIsStale covers the coverage contract
// for adopters who added an entry to satisfy the old rule.
//
// Goal: an entry for a directory holding only //go:build ignore files matches
// no package, so it is reported like any other stale entry.
func TestLayersEntryForIgnoredOnlyDirectoryIsStale(t *testing.T) {
	got := checkModule(t, writeModuleFiles(t, map[string]string{
		"go.mod":            "module " + layerModule + "\n\ngo 1.26\n",
		"lib/lib.go":        "// Package lib is pure.\npackage lib\n",
		"tools/gen.go":      "//go:build ignore\n\npackage main\n\nfunc main() {}\n",
		"tiger.layers.yaml": "\"./lib\": []\n\"./tools\": []\n",
	}))
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Contains(t, got.stdout, "tiger.layers.yaml:2:1: TS-X03")
}

// TestLayersImportInGeneratedFileIsReported covers the enforcement
// contract where it meets generated code.
//
// Goal: an import the entry doesn't list is reported from a generated file,
// at the import, as it is from a hand-written one.
func TestLayersImportInGeneratedFileIsReported(t *testing.T) {
	got := checkModule(t, writeModuleFiles(t, map[string]string{
		"go.mod":     "module " + layerModule + "\n\ngo 1.26\n",
		"lib/lib.go": "// Package lib is pure.\npackage lib\n",
		"lib/lib.pb.go": "// Code generated by protoc-gen-go. DO NOT EDIT.\n\npackage lib\n\n" +
			"import _ \"os\"\n",
		"tiger.layers.yaml": "\"./lib\": []\n",
	}))
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "lib/lib.pb.go:5:8: TS-X03: the \"./lib\" entry in tiger.layers.yaml doesn't "+
		"list os, so this package can't import os — add \"os\" to it, or pass it in through an "+
		"interface\ntiger: 1 blocking\n", got.stdout)
}

// TestLayersOtherPlatformImportIsCheckedWhereItBuilds covers the enforcement
// contract across operating systems.
//
// Goal: an import in a file built only for another operating system passes a
// run on this one, and fails a run that targets that system.
func TestLayersOtherPlatformImportIsCheckedWhereItBuilds(t *testing.T) {
	other := "windows"
	if runtime.GOOS == other {
		other = "linux"
	}
	file := "win/w_" + other + ".go"
	dir := writeModuleFiles(t, map[string]string{
		"go.mod":            "module " + layerModule + "\n\ngo 1.26\n",
		"win/w.go":          "// Package win is pure here.\npackage win\n",
		file:                "package win\n\nimport _ \"os\"\n",
		"tiger.layers.yaml": "\"./win\": []\n",
	})
	got := checkModule(t, dir)
	require.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)

	t.Setenv("GOOS", other)
	got = checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, file+":3:8: TS-X03: the \"./win\" entry in tiger.layers.yaml doesn't list "+
		"os, so this package can't import os — add \"os\" to it, or pass it in through an "+
		"interface\ntiger: 1 blocking\n", got.stdout)
}

// TestLayersDirectoryEndingInDotTestIsNotJudged covers the coverage contract
// for a directory named like go test's synthesized main package.
//
// Goal: a package in a directory whose name ends in .test is never judged
// import by import, because the driver takes it for go test's main, but tiger
// check still reports it when no entry covers it.
func TestLayersDirectoryEndingInDotTestIsNotJudged(t *testing.T) {
	dir := writeModuleFiles(t, map[string]string{
		"go.mod":            "module " + layerModule + "\n\ngo 1.26\n",
		"lib/lib.go":        "// Package lib is pure.\npackage lib\n",
		"tool.test/tool.go": "// Package tool reads arguments.\npackage tool\n\nimport _ \"os\"\n",
		"tiger.layers.yaml": "\"./lib\": []\n",
	})
	got := checkModule(t, dir)
	assert.Equal(t, cli.ExitFindings, got.code)
	assert.Equal(t, "tool.test/tool.go:1:1: TS-X03: package ./tool.test has no entry in "+
		"tiger.layers.yaml — add it as a key listing the module packages, modules and IO or "+
		"random subtrees (os, net, ...) it imports, or [] for none\ntiger: 1 blocking\n",
		got.stdout)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "tiger.layers.yaml"),
		[]byte("\"./lib\": []\n\"./tool.test\": []\n"), 0o644))
	got = checkModule(t, dir)
	assert.Equal(t, cli.ExitClean, got.code)
	assert.Empty(t, got.stdout)
}
