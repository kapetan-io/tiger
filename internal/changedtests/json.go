package changedtests

import (
	"bytes"
	"encoding/json"

	"github.com/kapetan-io/tiger/internal/revision"
)

const schema = "tiger.changed-tests/v1"

func (r Report) JSON(rev *revision.Comparison) ([]byte, error) {
	doc := document{
		Schema:               schema,
		Base:                 baseDoc{Ref: rev.BaseRef, Commit: rev.MergeBase, MergeBase: true},
		Head:                 headDoc{Commit: rev.Head, Dirty: rev.Dirty},
		ChangesExistingTests: len(r.Changed) > 0,
		Tests:                make([]testDoc, 0, len(r.Changed)),
	}
	for _, test := range r.Changed {
		doc.Tests = append(doc.Tests, testDoc{
			Package: test.Package, File: test.File, Function: test.Function,
			Change: string(test.Change),
		})
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

type document struct {
	Schema               string    `json:"schema"`
	Base                 baseDoc   `json:"base"`
	Head                 headDoc   `json:"head"`
	ChangesExistingTests bool      `json:"changes_existing_tests"`
	Tests                []testDoc `json:"tests"`
}

type baseDoc struct {
	Ref       string `json:"ref"`
	Commit    string `json:"commit"`
	MergeBase bool   `json:"merge_base"`
}

type headDoc struct {
	Commit string `json:"commit"`
	Dirty  bool   `json:"dirty"`
}

type testDoc struct {
	Package  string `json:"package"`
	File     string `json:"file"`
	Function string `json:"function"`
	Change   string `json:"change"`
}
