package l14

//easyjson:json // want `TS-L14`
type Easy struct{}

//why:x // want `TS-L14`
type Why struct{}

//lint:ignore SA1000 x // want `TS-L14`
type Lint struct{}

// nolint // want `TS-L14`
type SpacedNolint struct{}

///nolint // want `TS-L14`
type TripleNolint struct{}

/* x */ // want `TS-L14`
type Block struct{}

//go:generate x
// prose after a directive in the same group // want `TS-L14`
type Mixed struct{}
