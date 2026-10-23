package command_test

import (
	"bytes"
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kapetan-io/tiger/internal/command"
)

func TestPrintUsageWritesTheEntryThenTheFlagDefaults(t *testing.T) {
	out := &bytes.Buffer{}
	flags := flag.NewFlagSet("tiger demo", flag.ContinueOnError)
	flags.SetOutput(out)
	flags.String("C", ".", "run as if tiger was started in this directory")

	command.PrintUsage(flags, "  tiger demo [-C dir]\n      does a thing\n")

	assert.Equal(t, "usage:\n  tiger demo [-C dir]\n      does a thing\n"+
		"  -C string\n    \trun as if tiger was started in this directory "+
		"(default \".\")\n", out.String())
}
