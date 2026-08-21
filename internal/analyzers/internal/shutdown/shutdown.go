// Package shutdown recognizes shutdown channels, the second recognized
// shutdown shape shared by boundedloop (TS-S03) and selectctx (TS-C05).
//
// A channel is a shutdown channel when its element type is struct{} — a
// pure signal, the closed-channel broadcast shape — or, as a name-based
// fallback, when the channel expression's identifier or selected field
// name contains a shutdown token. Both recognitions only relax a check:
// a false match silences a finding (a marked known-miss class), never
// creates one.
package shutdown

import (
	"go/ast"
	"go/types"
	"strings"
)

// nameTokens are the lowercased substrings that mark a channel name as a
// shutdown signal. A constant until a trial says it needs a flag.
var nameTokens = []string{"shutdown", "stop", "quit", "done"}

// Channel reports whether expr is a recognized shutdown channel: a channel
// whose element type is struct{}, or one whose identifier or selected
// field name contains a shutdown token.
func Channel(info *types.Info, expr ast.Expr) bool {
	channel, ok := channelType(info, expr)
	if !ok {
		return false
	}
	if signal, isStruct := channel.Elem().Underlying().(*types.Struct); isStruct {
		if signal.NumFields() == 0 {
			return true
		}
	}
	return namedShutdown(expr)
}

// Named reports whether expr is a channel recognized as a shutdown channel
// by its name alone. A bare receive or send is exempt from TS-C05 only
// under this narrower recognition: a struct{} element type marks a pure
// signal, but outside a select that signal is just as often a completion
// wait (querator's req.ReadyCh) — exactly the missing-cancellation bug the
// rule exists to catch — so type alone never exempts a bare operation.
func Named(info *types.Info, expr ast.Expr) bool {
	_, ok := channelType(info, expr)
	return ok && namedShutdown(expr)
}

// channelType resolves expr's type to a channel, seeing through named
// types.
func channelType(info *types.Info, expr ast.Expr) (*types.Chan, bool) {
	resolved := info.TypeOf(expr)
	if resolved == nil {
		return nil, false
	}
	channel, ok := resolved.Underlying().(*types.Chan)
	return channel, ok
}

// namedShutdown reports whether expr is an identifier or a selector whose
// name contains a shutdown token, lowercased.
func namedShutdown(expr ast.Expr) bool {
	name := ""
	switch shape := expr.(type) {
	case *ast.Ident:
		name = shape.Name
	case *ast.SelectorExpr:
		name = shape.Sel.Name
	default:
		return false
	}
	lower := strings.ToLower(name)
	for _, token := range nameTokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}
