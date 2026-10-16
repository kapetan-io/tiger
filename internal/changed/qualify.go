package changed

import (
	"maps"
	"slices"
	"strings"

	"github.com/kapetan-io/tiger/internal/callinventory"
)

// rootNames is the printed names of the packages under the run directory at
// either revision.
func rootNames(sides Sides) map[string]bool {
	names := map[string]bool{}
	maps.Copy(names, sides.Base.Roots)
	maps.Copy(names, sides.Head.Roots)
	return names
}

// qualify prints a module package outside the run directory by its import
// path when a package under the run directory has the same printed name at
// either revision. Deciding that per revision would rename an unchanged callee
// when the other revision adds the colliding package.
func qualify(inv callinventory.Inventory, roots map[string]bool) callinventory.Inventory {
	renamed := map[string]string{}
	for _, name := range slices.Sorted(maps.Keys(inv.Above)) {
		if roots[name] {
			renamed[name] = inv.Above[name]
		}
	}
	if len(renamed) == 0 {
		return inv
	}
	out := inv
	out.Packages = map[string]bool{}
	for _, name := range slices.Sorted(maps.Keys(inv.Packages)) {
		path, ok := renamed[name]
		if ok {
			out.Packages[path] = true
		}
		// The plain name stays when a package under the run directory owns it.
		if !ok || inv.Roots[name] {
			out.Packages[name] = true
		}
	}
	out.Functions = map[string]callinventory.Function{}
	for _, header := range slices.Sorted(maps.Keys(inv.Functions)) {
		fn := inv.Functions[header]
		fn.Calls = map[string]callinventory.Call{}
		for _, key := range slices.Sorted(maps.Keys(inv.Functions[header].Calls)) {
			call := inv.Functions[header].Calls[key]
			if path, ok := renamed[call.Package]; ok && call.Path == path && !call.SamePackage {
				call = relabel(call, path)
			}
			fn.Calls[key] = call
		}
		out.Functions[header] = fn
	}
	return out
}

// relabel prints a call's package by import path.
func relabel(call callinventory.Call, path string) callinventory.Call {
	old := call.Package
	call.Callee = path + strings.TrimPrefix(call.Callee, old)
	call.Target = path + strings.TrimPrefix(call.Target, old)
	call.Package = path
	return call
}
