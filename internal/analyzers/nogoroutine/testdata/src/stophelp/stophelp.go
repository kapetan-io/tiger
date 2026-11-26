// Package stophelp waits on a group handed to it from another package, which
// TS-C02 does not follow.
package stophelp

import "sync"

func Wait(wg *sync.WaitGroup) { wg.Wait() }
