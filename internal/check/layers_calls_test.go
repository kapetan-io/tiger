package check_test

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kapetan-io/tiger/internal/command"
)

const scanSource = "package service\n\nimport \"path/filepath\"\n\n" +
	"func Scan() error {\n\t_, err := filepath.Glob(\"*\")\n\treturn err\n}\n"

// TestLayersFileReadingFunctionNeedsOs covers the function contract.
//
// Goal: a use of a pure standard library function that reads the file system
// fires when the entry doesn't list os, names the function, and passes once
// the entry lists os.
func TestLayersFileReadingFunctionNeedsOs(t *testing.T) {
	dir := writeModuleFiles(t, layerTree)
	writeFiles(t, dir, map[string]string{"service/scan.go": scanSource})

	got := checkModule(t, dir)
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Contains(t, got.stdout, "TS-X03: the \"./service\" entry in tiger.layers.yaml "+
		"doesn't list os, so this package can't use path/filepath.Glob — add \"os\" to it, "+
		"or take an fs.FS or the data as an input")

	files := maps.Clone(layerTree)
	files["service/scan.go"] = scanSource
	files["tiger.layers.yaml"] = "\"./store\":\n  - database\n" +
		"\"./service\":\n  - ./store\n  - os\n\"./web\":\n  - net\n"
	got = checkModule(t, writeModuleFiles(t, files))
	assert.Equal(t, command.ExitClean, got.code)
	assert.Empty(t, got.stdout)
}

// TestLayersSyslogMustBeNamed covers the impure subtree contract.
//
// Goal: importing log/syslog fires unless the entry names it.
func TestLayersSyslogMustBeNamed(t *testing.T) {
	dir := writeModuleFiles(t, layerTree)
	writeFiles(t, dir, map[string]string{
		"service/log.go": "package service\n\nimport _ \"log/syslog\"\n",
	})

	got := checkModule(t, dir)
	assert.Equal(t, command.ExitFindings, got.code)
	assert.Contains(t, got.stdout, "service/log.go:3:8: TS-X03: the \"./service\" entry in "+
		"tiger.layers.yaml doesn't list log/syslog, so this package can't import log/syslog "+
		"— add \"log/syslog\"")
	assert.Contains(t, got.stdout, "tiger: 1 blocking\n")
}

// TestLayersWithoutAFileAllowFileReadingFunctions covers the opt-in contract.
//
// Goal: a module with no layer file may use the functions that read the file
// system.
func TestLayersWithoutAFileAllowFileReadingFunctions(t *testing.T) {
	files := maps.Clone(layerTree)
	delete(files, "tiger.layers.yaml")
	files["service/scan.go"] = scanSource

	got := checkModule(t, writeModuleFiles(t, files))
	assert.Equal(t, command.ExitClean, got.code)
	assert.Empty(t, got.stdout)
}
