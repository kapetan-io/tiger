package changed

import (
	"maps"
	"slices"
	"strings"

	"github.com/kapetan-io/tiger/internal/callinventory"
)

func rootNames(sides Sides) map[string]bool {
	names := map[string]bool{}
	for _, name := range slices.Concat(
		slices.Sorted(maps.Keys(sides.Base.Roots)), slices.Sorted(maps.Keys(sides.Head.Roots))) {
		names[name] = true
	}
	return names
}

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
		if _, root := inv.Roots[name]; !ok || root {
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

func relabel(call callinventory.Call, path string) callinventory.Call {
	old := call.Package
	call.Callee = path + strings.TrimPrefix(call.Callee, old)
	call.Target = path + strings.TrimPrefix(call.Target, old)
	call.Package = path
	return call
}
