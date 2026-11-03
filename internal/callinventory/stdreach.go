package callinventory

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/types"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

const stdReachDir = ".tiger"

const stdReachMagic = "tiger-stdreach/1"

const stdReachEnd = "end"

type StdReach struct {
	dir      string
	once     sync.Once
	err      error
	saveErr  error
	packages map[string]bool
	reaches  map[string]bool
}

type toolchain struct {
	Version string `json:"GOVERSION"`
	OS      string `json:"GOOS"`
	Arch    string `json:"GOARCH"`
	Mod     string `json:"GOMOD"`
}

func (t toolchain) key() string {
	return stdReachMagic + " " + t.Version + " " + t.OS + " " + t.Arch
}

func (t toolchain) root(dir string) string {
	if t.Mod == "" || t.Mod == os.DevNull {
		return dir
	}
	return filepath.Dir(t.Mod)
}

func goToolchain(dir string) (toolchain, error) {
	cmd := exec.Command("go", "env", "-json", "GOVERSION", "GOOS", "GOARCH", "GOMOD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return toolchain{}, fmt.Errorf("go env: %w", err)
	}
	var found toolchain
	if err := json.Unmarshal(out, &found); err != nil {
		return toolchain{}, fmt.Errorf("go env: %w", err)
	}
	return found, nil
}

func NewStdReach(dir string) *StdReach {
	return &StdReach{dir: dir}
}

func (t *StdReach) ready() error {
	t.once.Do(func() { t.err = t.resolve() })
	return t.err
}

func (t *StdReach) SaveError() error {
	return t.saveErr
}

func (t *StdReach) resolve() error {
	dir := t.dir
	chain, err := goToolchain(dir)
	if err != nil {
		return err
	}
	path := filepath.Join(chain.root(dir), stdReachDir, "stdreach-"+chain.OS+"-"+chain.Arch+".txt")
	if data, err := os.ReadFile(path); err == nil && t.parse(string(data), chain) == nil {
		return nil
	}
	var text bytes.Buffer
	if err := writeStdReach(&text, dir, chain); err != nil {
		return err
	}
	if err := t.parse(text.String(), chain); err != nil {
		return err
	}
	t.saveErr = writeFileAtomic(path, text.Bytes())
	return nil
}

func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".stdreach-*")
	if err != nil {
		return err
	}
	_, err = temp.Write(data)
	err = errors.Join(err, temp.Close())
	if err == nil {
		err = os.Rename(temp.Name(), path)
	}
	if err != nil {
		return errors.Join(err, os.Remove(temp.Name()))
	}
	return nil
}

func (t *StdReach) parse(text string, chain toolchain) error {
	first, rest, _ := strings.Cut(text, "\n")
	if first != chain.key() {
		return fmt.Errorf("reach file is for %q, not %q", first, chain.key())
	}
	body, complete := strings.CutSuffix(strings.TrimRight(rest, "\n"), "\n"+stdReachEnd)
	if !complete {
		return fmt.Errorf("reach file does not end with %q", stdReachEnd)
	}
	t.packages, t.reaches = map[string]bool{}, map[string]bool{}
	pkg := ""
	for i, line := range strings.Split(body, "\n") {
		switch {
		case line == "":
		case strings.HasPrefix(line, "\t"):
			if pkg == "" {
				return fmt.Errorf("line %d: member before any package", i+2)
			}
			t.reaches[pkg+"."+line[1:]] = true
		default:
			pkg = line
			t.packages[pkg] = true
		}
	}
	return nil
}

func (t *StdReach) knows(path string) bool {
	return t.packages[path]
}

func (t *StdReach) member(fn *ssa.Function) bool {
	if origin := fn.Origin(); origin != nil {
		fn = origin
	}
	obj, ok := fn.Object().(*types.Func)
	if !ok || obj.Pkg() == nil || !t.packages[obj.Pkg().Path()] {
		return false
	}
	return t.reaches[memberKey(obj)]
}

func memberKey(obj *types.Func) string {
	key := obj.Pkg().Path() + "." + obj.Name()
	if recv := obj.Signature().Recv(); recv != nil {
		if named := receiver(recv.Type()); named != nil {
			key = obj.Pkg().Path() + "." + named.Obj().Name() + "." + obj.Name()
		}
	}
	return key
}

func writeStdReach(w io.Writer, dir string, chain toolchain) error {
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.LoadAllSyntax,
		Dir:  dir,
		Env:  append(os.Environ(), "GOOS="+chain.OS, "GOARCH="+chain.Arch, "CGO_ENABLED=0"),
	}, "std")
	if err != nil {
		return err
	}
	if n := packages.PrintErrors(pkgs); n > 0 {
		return fmt.Errorf("%d packages of the standard library did not load", n)
	}
	prog, _ := ssautil.AllPackages(pkgs, 0)
	prog.Build()
	body := newBodyReach(nil)
	table := map[string][]string{}
	for _, pkg := range pkgs {
		if !importable(pkg.PkgPath) {
			continue
		}
		var members []string
		for _, fn := range exportedMembers(prog, pkg.Types) {
			found, err := body.reaches(fn)
			if err != nil {
				return fmt.Errorf("%s: %w", fn, err)
			}
			if found {
				members = append(members, strings.TrimPrefix(memberKey(fn.Object().(*types.Func)), pkg.PkgPath+"."))
			}
		}
		slices.Sort(members)
		table[pkg.PkgPath] = slices.Compact(members)
	}
	out := bufio.NewWriter(w)
	fmt.Fprintln(out, chain.key())
	for _, path := range slices.Sorted(maps.Keys(table)) {
		fmt.Fprintln(out, path)
		for _, member := range table[path] {
			fmt.Fprintf(out, "\t%s\n", member)
		}
	}
	fmt.Fprintln(out, stdReachEnd)
	return out.Flush()
}

func importable(path string) bool {
	if strings.HasPrefix(path, "cmd/") || strings.HasPrefix(path, "vendor/") {
		return false
	}
	return path != "internal" && !strings.HasPrefix(path, "internal/") && !strings.Contains(path, "/internal/") &&
		!strings.HasSuffix(path, "/internal")
}

func exportedMembers(prog *ssa.Program, pkg *types.Package) []*ssa.Function {
	var list []*ssa.Function
	add := func(obj *types.Func) {
		if fn := prog.FuncValue(obj); fn != nil {
			list = append(list, fn)
		}
	}
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if !obj.Exported() {
			continue
		}
		switch obj := obj.(type) {
		case *types.Func:
			add(obj)
		case *types.TypeName:
			if _, isAlias := obj.Type().(*types.Alias); isAlias {
				continue
			}
			named, ok := obj.Type().(*types.Named)
			if !ok || types.IsInterface(named) {
				continue
			}
			for _, recv := range []types.Type{named, types.NewPointer(named)} {
				set := types.NewMethodSet(recv)
				for i := 0; i < set.Len(); i++ {
					method, ok := set.At(i).Obj().(*types.Func)
					if ok && method.Exported() {
						add(method)
					}
				}
			}
		}
	}
	return list
}
