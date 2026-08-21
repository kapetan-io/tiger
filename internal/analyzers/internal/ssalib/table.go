package ssalib

import (
	"maps"
	"slices"

	"golang.org/x/tools/go/ssa"

	"github.com/kapetan-io/tiger/internal/directive"
)

// CallName addresses one callee in the stdlib effects table: its full
// function string as go/ssa renders it and its package path.
type CallName struct {
	// Full is (*ssa.Function).String(): "os.Getenv", "(*os.File).Write".
	Full string
	// Package is the callee's package path: "os", "os/exec".
	Package string
}

// disk, netIO, exec, env, clock, random, block, and alloc are the table's
// building blocks, spelled once.
var (
	disk   = directive.EffectSet{IO: []string{"disk"}}
	netIO  = directive.EffectSet{IO: []string{"net"}}
	exec   = directive.EffectSet{IO: []string{"exec"}}
	env    = directive.EffectSet{IO: []string{"env"}}
	clock  = directive.EffectSet{Time: true}
	random = directive.EffectSet{Rand: true}
	waits  = directive.EffectSet{Block: true}
	heap   = directive.EffectSet{Alloc: true}
)

// exactEffects maps one standard-library function to its effects. The table
// is a committed, versioned artifact: an unlisted call contributes nothing,
// and the coverage gaps are the analyzer's known misses (never a guess).
var exactEffects = map[string]directive.EffectSet{
	// Filesystem and file handles.
	"os.Open":                disk,
	"os.OpenFile":            disk,
	"os.Create":              disk,
	"os.ReadFile":            disk,
	"os.WriteFile":           disk,
	"os.ReadDir":             disk,
	"os.Remove":              disk,
	"os.RemoveAll":           disk,
	"os.Rename":              disk,
	"os.Mkdir":               disk,
	"os.MkdirAll":            disk,
	"os.MkdirTemp":           disk,
	"os.CreateTemp":          disk,
	"os.Stat":                disk,
	"os.Lstat":               disk,
	"os.Chmod":               disk,
	"os.Symlink":             disk,
	"os.Readlink":            disk,
	"os.Getwd":               disk,
	"os.Chdir":               disk,
	"(*os.File).Write":       disk,
	"(*os.File).WriteString": disk,
	"(*os.File).Read":        disk,
	"(*os.File).Close":       disk,
	"(*os.File).Sync":        disk,
	"(*os.File).Seek":        disk,
	"path/filepath.Abs":      disk,
	"path/filepath.Walk":     disk,
	"path/filepath.WalkDir":  disk,
	"path/filepath.Glob":     disk,

	// Standard streams: fmt's default printers write file descriptors.
	"fmt.Print":    disk,
	"fmt.Printf":   disk,
	"fmt.Println":  disk,
	"fmt.Fprint":   disk,
	"fmt.Fprintf":  disk,
	"fmt.Fprintln": disk,
	"fmt.Scan":     disk,
	"fmt.Scanln":   disk,

	// Network.
	"net.Dial":                          netIO,
	"net.DialTimeout":                   netIO,
	"net.Listen":                        netIO,
	"net.LookupHost":                    netIO,
	"net.LookupIP":                      netIO,
	"net/http.Get":                      netIO,
	"net/http.Post":                     netIO,
	"net/http.Head":                     netIO,
	"(*net/http.Client).Do":             netIO,
	"(*net/http.Client).Get":            netIO,
	"(*net/http.Client).Post":           netIO,
	"(*net/http.Server).ListenAndServe": netIO,
	"net/http.ListenAndServe":           netIO,

	// Environment.
	"os.Getenv":    env,
	"os.LookupEnv": env,
	"os.Setenv":    env,
	"os.Unsetenv":  env,
	"os.Environ":   env,
	"os.ExpandEnv": env,

	// The clock. Sleep also parks the goroutine.
	"time.Now":       clock,
	"time.Since":     clock,
	"time.Until":     clock,
	"time.After":     clock,
	"time.Tick":      clock,
	"time.NewTimer":  clock,
	"time.NewTicker": clock,
	"time.Sleep":     {Block: true, Time: true},

	// Randomness outside the math/rand package trees.
	"crypto/rand.Read": random,
	"crypto/rand.Int":  random,

	// Blocking synchronization; channel operations are instruction-level.
	"(*sync.Mutex).Lock":     waits,
	"(*sync.RWMutex).Lock":   waits,
	"(*sync.RWMutex).RLock":  waits,
	"(*sync.WaitGroup).Wait": waits,
	"(*sync.Once).Do":        waits,
	"(*sync.Cond).Wait":      waits,

	// Common allocators; own-instruction analysis catches user-code
	// allocation, so these cover the formatting helpers that hide it.
	"fmt.Sprint":     heap,
	"fmt.Sprintf":    heap,
	"fmt.Sprintln":   heap,
	"fmt.Errorf":     heap,
	"strings.Join":   heap,
	"strings.Repeat": heap,
}

// packageEffects maps a whole standard-library package to one effect, for
// packages whose every entry point carries it.
var packageEffects = map[string]directive.EffectSet{
	"os/exec":      exec,
	"math/rand":    random,
	"math/rand/v2": random,
	"log":          disk,
}

// Lookup resolves one callee against the table: the exact function entry
// first, then the whole-package entry.
func Lookup(call CallName) (directive.EffectSet, bool) {
	if found, ok := exactEffects[call.Full]; ok {
		return found, true
	}
	if found, ok := packageEffects[call.Package]; ok {
		return found, true
	}
	return directive.EffectSet{}, false
}

// Stdlib resolves a callee function against the table.
func Stdlib(callee *ssa.Function) (directive.EffectSet, bool) {
	if callee.Pkg == nil {
		return directive.EffectSet{}, false
	}
	return Lookup(CallName{Full: callee.String(), Package: callee.Pkg.Pkg.Path()})
}

// TableNames returns every committed entry's name in a stable order —
// exact function entries first, then whole-package entries — for the
// table's own tests.
func TableNames() []CallName {
	names := []CallName{}
	for _, full := range slices.Sorted(maps.Keys(exactEffects)) {
		names = append(names, CallName{Full: full})
	}
	for _, pkg := range slices.Sorted(maps.Keys(packageEffects)) {
		names = append(names, CallName{Package: pkg})
	}
	return names
}
