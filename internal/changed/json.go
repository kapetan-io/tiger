package changed

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/kapetan-io/tiger/internal/callinventory"
	"github.com/kapetan-io/tiger/internal/revision"
)

const schema = "tiger.changed/v1"

// blindSpotIDs are stable names for what the footer's second line says tiger
// cannot see, in the order the footer lists them. Tools match on these, so
// changing one is a breaking change; add one when the footer gains a blind spot.
var blindSpotIDs = []string{
	"outside-without-os", "io-writer", "channel-send", "interface-implementation",
	"same-owner", "same-package-leaf", "same-package-interface", "moved-to-helper",
	"renamed-type",
}

// JSON renders the report for tools and agent reviewers. Every site of every
// call is kept. The bytes depend only on the report and the revisions.
func (r Report) JSON(rev revision.Comparison) ([]byte, error) {
	doc := document{
		Schema: schema,
		Base:   baseDoc{Ref: rev.BaseRef, Commit: rev.MergeBase, MergeBase: true},
		Head:   headDoc{Commit: rev.Head, Dirty: rev.Dirty},
		Scope: scopeDoc{
			Owners:                r.Scope.Owners,
			Packages:              r.Scope.Packages,
			SkippedGeneratedFiles: nonNil(r.Scope.Generated),
			NotCompared:           nonNil(r.Scope.NotCompared),
		},
		Counts: countsDoc{
			CallsAdded:      r.added(),
			CallsRemoved:    r.removed(),
			NewPackages:     len(r.NewPackages),
			RemovedPackages: len(r.RemovedPackages),
			Renamed:         len(r.Renamed),
		},
		Functions:       functionDocs(r.Functions),
		Renamed:         []renameDoc{},
		NewPackages:     []newPackageDoc{},
		RemovedPackages: []removedPackageDoc{},
		BlindSpots:      blindSpotIDs,
	}
	for _, rename := range r.Renamed {
		doc.Renamed = append(doc.Renamed, renameDoc{
			Package: rename.Package, From: rename.From, To: rename.To,
		})
	}
	for _, pkg := range r.NewPackages {
		doc.NewPackages = append(doc.NewPackages, newPackageDoc{
			Package: pkg.Name,
			Owners:  pkg.Owners,
			Calls:   pkg.calls(),
			Uses: usesDoc{
				Prior:   useDocs(pkg.Uses.Prior),
				New:     useDocs(pkg.Uses.New),
				Outside: useDocs(pkg.Uses.Outside),
			},
			Functions: functionDocs(pkg.Functions),
		})
	}
	for _, pkg := range r.RemovedPackages {
		doc.RemovedPackages = append(doc.RemovedPackages, removedPackageDoc{
			Package: pkg.Name, Owners: pkg.Owners, Calls: pkg.calls(),
			Functions: functionDocs(pkg.Functions),
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
	Schema          string              `json:"schema"`
	Base            baseDoc             `json:"base"`
	Head            headDoc             `json:"head"`
	Scope           scopeDoc            `json:"scope"`
	Counts          countsDoc           `json:"counts"`
	Functions       []functionDoc       `json:"functions"`
	Renamed         []renameDoc         `json:"renamed"`
	NewPackages     []newPackageDoc     `json:"new_packages"`
	RemovedPackages []removedPackageDoc `json:"removed_packages"`
	BlindSpots      []string            `json:"blind_spots"`
}

// baseDoc names the merge base: Commit is that commit, not the tip of Ref.
type baseDoc struct {
	Ref       string `json:"ref"`
	Commit    string `json:"commit"`
	MergeBase bool   `json:"merge_base"`
}

// headDoc has no ref because CI checks out a detached merge commit that has
// no branch to name. When Dirty is true, head sites refer to the working tree.
type headDoc struct {
	Commit string `json:"commit"`
	Dirty  bool   `json:"dirty"`
}

type scopeDoc struct {
	Owners                int      `json:"owners"`
	Packages              int      `json:"packages"`
	SkippedGeneratedFiles []string `json:"skipped_generated_files"`
	NotCompared           []string `json:"not_compared"`
}

type countsDoc struct {
	CallsAdded      int `json:"calls_added"`
	CallsRemoved    int `json:"calls_removed"`
	NewPackages     int `json:"new_packages"`
	RemovedPackages int `json:"removed_packages"`
	Renamed         int `json:"renamed"`
}

type functionDoc struct {
	Function string    `json:"function"`
	Owner    string    `json:"owner"`
	Method   *string   `json:"method"`
	Calls    []callDoc `json:"calls"`
}

type callDoc struct {
	Change            string    `json:"change"`
	Callee            string    `json:"callee"`
	Target            string    `json:"target"`
	Method            string    `json:"method"`
	TargetPackage     string    `json:"target_package"`
	TargetInModule    bool      `json:"target_in_module"`
	SamePackage       bool      `json:"same_package"`
	PackageDependency *string   `json:"package_dependency"`
	Sites             []siteDoc `json:"sites"`
}

type siteDoc struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Rev  string `json:"rev"`
}

type renameDoc struct {
	Package string `json:"package"`
	From    string `json:"from"`
	To      string `json:"to"`
}

type newPackageDoc struct {
	Package   string        `json:"package"`
	Owners    int           `json:"owners"`
	Calls     int           `json:"calls"`
	Uses      usesDoc       `json:"uses"`
	Functions []functionDoc `json:"functions"`
}

type usesDoc struct {
	Prior   []useDoc `json:"existing"`
	New     []useDoc `json:"new"`
	Outside []useDoc `json:"outside"`
}

type useDoc struct {
	Package string   `json:"package"`
	Symbols []string `json:"symbols"`
}

type removedPackageDoc struct {
	Package   string        `json:"package"`
	Owners    int           `json:"owners"`
	Calls     int           `json:"calls"`
	Functions []functionDoc `json:"functions"`
}

func functionDocs(functions []Function) []functionDoc {
	docs := make([]functionDoc, 0, len(functions))
	for _, fn := range functions {
		doc := functionDoc{Function: fn.Name, Owner: fn.Owner, Calls: make([]callDoc, 0, len(fn.Calls))}
		if fn.Name != fn.Owner {
			doc.Method = new(strings.TrimPrefix(fn.Name, fn.Owner+"."))
		}
		for _, call := range fn.Calls {
			doc.Calls = append(doc.Calls, callDocOf(call))
		}
		docs = append(docs, doc)
	}
	return docs
}

func callDocOf(call Call) callDoc {
	change, rev := "removed", "base"
	if call.Added {
		change, rev = "added", "head"
	}
	doc := callDoc{
		Change:         change,
		Callee:         call.Callee,
		Target:         call.Target,
		Method:         call.Method,
		TargetPackage:  call.TargetPackage,
		TargetInModule: call.InModule,
		SamePackage:    call.SamePackage,
		Sites:          siteDocs(call.Sites, rev),
	}
	if call.Dependency != "" {
		doc.PackageDependency = new(string(call.Dependency))
	}
	return doc
}

func siteDocs(sites []callinventory.Site, rev string) []siteDoc {
	docs := make([]siteDoc, 0, len(sites))
	for _, site := range sites {
		docs = append(docs, siteDoc{File: site.File, Line: site.Line, Rev: rev})
	}
	return docs
}

func useDocs(uses []Use) []useDoc {
	docs := make([]useDoc, 0, len(uses))
	for _, use := range uses {
		docs = append(docs, useDoc{Package: use.Package, Symbols: use.Symbols})
	}
	return docs
}

func nonNil(names []string) []string {
	if names == nil {
		return []string{}
	}
	return names
}
