// Package inner is covered by "./domain/...", and an in-module name is one
// package exactly, so the subtree entry does not allow ./store.
package inner

import _ "ts-x03/store" // want `the "\./domain/\.\.\." entry in tiger\.layers\.yaml doesn't list \./store`
