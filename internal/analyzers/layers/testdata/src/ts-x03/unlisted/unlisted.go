package unlisted // want `package \./unlisted has no entry in tiger\.layers\.yaml — add it as a key listing the module packages, modules and IO or random subtrees \(os, net, \.\.\.\) it imports, or \[\] for none`

// A package with no entry reports only its missing entry, so this import
// expects no finding: there is no entry to judge it against.
import _ "os"
