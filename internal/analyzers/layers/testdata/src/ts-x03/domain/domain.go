// Package domain is covered by the "./domain/..." entry, which names no IO.
package domain

import _ "os" // want `the "\./domain/\.\.\." entry in tiger\.layers\.yaml doesn't list os, so this package can't import os`
