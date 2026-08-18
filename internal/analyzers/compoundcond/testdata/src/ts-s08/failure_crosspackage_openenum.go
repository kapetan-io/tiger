// The cross-package //tiger:openenum limitation: a type marked openenum in
// another package still demands assert.Unreachable here, because
// compoundcond only reads directives from the package under analysis.
package fixture

import "reason"

// crossPackageOpenEnum switches over reason.Code, which is marked
// //tiger:openenum in reason's own package (see testdata/src/reason).
// compoundcond cannot read that package's comments, so the marking is
// invisible from here and the switch is judged an ordinary closed set
// requiring assert.Unreachable. This fires — a false positive against the
// documented intent — so it lives in a failure file, not a knownmiss file.
func crossPackageOpenEnum(code reason.Code) string {
	switch code { // want `TS-S08: switch over closed set Code has no default arm`
	case reason.CodeOne:
		return "one"
	case reason.CodeTwo:
		return "two"
	}
	return ""
}
