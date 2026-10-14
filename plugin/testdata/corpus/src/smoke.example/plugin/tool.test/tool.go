// Package tool sits in a directory named like go test's main package and imports
// os, which its tiger.layers.yaml entry does not list.
package tool

import _ "os"
