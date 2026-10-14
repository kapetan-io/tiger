// More documented TS-S03 coverage gaps.
package fixture

import "context"

// runParamOverwritten replaces its ctx parameter with Background.
//
// known-miss: a parameter counts as supplied by the caller wherever it is
// assigned, so the overwrite is not followed.
func runParamOverwritten(ctx context.Context, input <-chan int, output chan<- int) {
	ctx = context.Background()
	for {
		select {
		case <-ctx.Done():
			return
		case value := <-input:
			output <- value
		}
	}
}

// runInPackageLiteral waits on Background inside a function literal that
// initializes a package variable.
//
// known-miss: code outside any declaration has no body to trace a context
// through, so its contexts count as supplied from outside.
var runInPackageLiteral = func(input <-chan int, output chan<- int) {
	ctx := context.Background()
	for {
		select {
		case <-ctx.Done():
			return
		case value := <-input:
			output <- value
		}
	}
}
