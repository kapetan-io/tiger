// The TS-L09 blocking failure mode for retired pins: tiger changed replaced
// the effect and frame pins (ADR-0026), so a file that still carries one has
// a directive tiger no longer recognizes.
package fixture

// RetiredEffectsPin still states an effect set.
//
// want +1 `TS-L09: //tiger:effects is not a directive tiger recognizes`
//tiger:effects none
func RetiredEffectsPin() {}

// RetiredFramePin still states a frame.
//
// want +1 `TS-L09: //tiger:frame is not a directive tiger recognizes`
//tiger:frame none
func RetiredFramePin() {}
