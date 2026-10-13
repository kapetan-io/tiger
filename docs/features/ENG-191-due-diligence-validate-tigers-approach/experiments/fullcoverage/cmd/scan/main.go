// Command scan runs the prototype as a vet tool, with -sites, over the
// latest version of every module in a module cache and over extra module
// roots, and writes every site and finding as TSV. No module is downloaded:
// the go command resolves only from the cache's download directory, so a
// package whose dependencies are not in the cache is skipped and counted.
//
//	go build -o /tmp/fullcoverage ./cmd/fullcoverage
//	go run ./cmd/scan -vettool /tmp/fullcoverage -modcache ~/go/pkg/mod \
//	    -out sites.tsv -status status.tsv [dir[:pattern] ...]
package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"
)

const moduleTimeout = 10 * time.Minute

// job is one module root to scan; copyMod runs the go command on a copy of
// its go.mod so a read-only module cache is never written.
type job struct {
	module, dir, pattern string
	copyMod              bool
}

// sites says whether the vet tool takes -sites.
var sites bool

// toolchain is the GOTOOLCHAIN every go command runs with.
var toolchain string

var diagLine = regexp.MustCompile(`^(\S+\.go):(\d+):(\d+): (.*)$`)

func main() {
	vettool := flag.String("vettool", "", "path to the built fullcoverage command")
	modcache := flag.String("modcache", "", "module cache to scan (latest version of each module)")
	out := flag.String("out", "sites.tsv", "TSV of sites and findings")
	status := flag.String("status", "status.tsv", "TSV of per-module load status")
	workers := flag.Int("workers", 6, "modules scanned at once")
	flag.BoolVar(&sites, "sites", true, "pass -sites to the vet tool (false for a tool without it, such as tiger's maporder)")
	flag.StringVar(&toolchain, "toolchain", "local", "GOTOOLCHAIN for the go command; build -vettool with the same one")
	flag.Parse()

	var jobs []job
	if *modcache != "" {
		for _, dir := range latestModules(*modcache) {
			rel, _ := filepath.Rel(*modcache, dir)
			jobs = append(jobs, job{module: rel, dir: dir, pattern: "./...", copyMod: true})
		}
	}
	for _, arg := range flag.Args() {
		dir, pattern, ok := strings.Cut(arg, ":")
		if !ok {
			pattern = "./..."
		}
		jobs = append(jobs, job{module: dir, dir: dir, pattern: pattern})
	}

	sites, err := os.Create(*out)
	if err != nil {
		panic(err)
	}
	defer sites.Close()
	statuses, err := os.Create(*status)
	if err != nil {
		panic(err)
	}
	defer statuses.Close()

	var mu sync.Mutex
	work := make(chan job)
	var wg sync.WaitGroup
	for range *workers {
		wg.Go(func() {
			for j := range work {
				lines, state := scan(*vettool, j)
				mu.Lock()
				for _, line := range lines {
					fmt.Fprintf(sites, "%s\t%s\n", j.module, line)
				}
				fmt.Fprintf(statuses, "%s\t%s\n", j.module, state)
				mu.Unlock()
			}
		})
	}
	for _, j := range jobs {
		work <- j
	}
	close(work)
	wg.Wait()
}

// scan vets the packages of one module that load without errors and
// returns its diagnostics as "file:line:col\tmessage" plus a status line
// "packages\tclean\tvet-exit\tnote".
func scan(vettool string, j job) ([]string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), moduleTimeout)
	defer cancel()
	modflags, cleanup, err := modfileCopy(j)
	if err != nil {
		return nil, "0\t0\t-\t" + err.Error()
	}
	defer cleanup()

	list := exec.CommandContext(ctx, "go", append(append([]string{"list", "-e"}, modflags...),
		"-f", "{{.ImportPath}}\t{{if or .Error .DepsErrors}}bad{{end}}", j.pattern)...)
	list.Dir, list.Env = j.dir, env(j)
	listed, _ := list.Output()
	var all, clean []string
	for line := range strings.Lines(string(listed)) {
		path, state, _ := strings.Cut(strings.TrimSpace(line), "\t")
		if path == "" {
			continue
		}
		all = append(all, path)
		if state == "" {
			clean = append(clean, path)
		}
	}
	if len(clean) == 0 {
		return nil, fmt.Sprintf("%d\t0\t-\tno package loads", len(all))
	}
	args := append(append([]string{"vet"}, modflags...), "-vettool="+vettool)
	if sites {
		args = append(args, "-sites")
	}
	vet := exec.CommandContext(ctx, "go", append(args, clean...)...)
	vet.Dir, vet.Env = j.dir, env(j)
	var output bytes.Buffer
	vet.Stdout, vet.Stderr = &output, &output
	runErr := vet.Run()
	exit := "0"
	if runErr != nil {
		exit = "1"
	}
	var lines []string
	scanner := bufio.NewScanner(&output)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	note := ""
	for scanner.Scan() {
		match := diagLine.FindStringSubmatch(scanner.Text())
		if match != nil && !strings.HasPrefix(match[4], "TS-T02") && !strings.HasPrefix(match[4], "site ") {
			match = nil
		}
		if match == nil {
			if text := scanner.Text(); !strings.HasPrefix(text, "#") && note == "" {
				note = text
			}
			continue
		}
		file := match[1]
		if !filepath.IsAbs(file) {
			file = filepath.Join(j.dir, file)
		}
		lines = append(lines, fmt.Sprintf("%s:%s:%s\t%s", file, match[2], match[3], match[4]))
	}
	return lines, fmt.Sprintf("%d\t%d\t%s\t%s", len(all), len(clean), exit, note)
}

func env(j job) []string {
	cache := filepath.Join(os.Getenv("HOME"), "go", "pkg", "mod", "cache", "download")
	vars := append(os.Environ(), "GOTOOLCHAIN="+toolchain, "GOWORK=off", "GOSUMDB=off",
		"GOPROXY=file://"+cache)
	if j.copyMod {
		vars = append(vars, "GOFLAGS=-mod=mod")
	}
	return vars
}

// modfileCopy copies j's go.mod and go.sum to a temporary directory and
// returns the -modfile flag pointing at the copy.
func modfileCopy(j job) ([]string, func(), error) {
	if !j.copyMod {
		return nil, func() {}, nil
	}
	data, err := os.ReadFile(filepath.Join(j.dir, "go.mod"))
	if err != nil {
		return nil, nil, fmt.Errorf("no go.mod")
	}
	tmp, err := os.MkdirTemp("", "fcscan")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), data, 0o644); err != nil {
		cleanup()
		return nil, nil, err
	}
	if sum, err := os.ReadFile(filepath.Join(j.dir, "go.sum")); err == nil {
		_ = os.WriteFile(filepath.Join(tmp, "go.sum"), sum, 0o644)
	}
	return []string{"-modfile=" + filepath.Join(tmp, "go.mod")}, cleanup, nil
}

// latestModules returns the directory of the highest version of every
// module in cache.
func latestModules(cache string) []string {
	best := map[string]string{}
	_ = filepath.WalkDir(cache, func(path string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(cache, path)
		if rel == "cache" || strings.HasPrefix(rel, "golang.org/toolchain") {
			return filepath.SkipDir
		}
		at := strings.LastIndex(d.Name(), "@")
		if at < 0 {
			return nil
		}
		module := strings.TrimSuffix(rel, d.Name()[at:])
		version := d.Name()[at+1:]
		if current, ok := best[module]; !ok || semver.Compare(version, current) > 0 {
			best[module] = version
		}
		return filepath.SkipDir
	})
	var dirs []string
	for module, version := range best {
		dirs = append(dirs, filepath.Join(cache, module+"@"+version))
	}
	slices.Sort(dirs)
	return dirs
}
