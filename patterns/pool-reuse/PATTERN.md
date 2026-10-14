# pool-reuse

Zero the whole pooled value as the last write before `Put`, so the next `Get` cannot see the last
user's data, and test it by inspecting the value you put back.

## Rules

TS-M05

## Gap

Tiger checks that `*b = T{}` runs before `Put` on every path (TS-M05). It cannot check that nothing
writes the value after the zeroing. `ZeroThenUse` zeroes the buffer when it takes it out and then
writes the user and data, so the zeroing precedes `Put` and tiger stays silent while the buffer goes
back dirty. `UseThenZero` zeroes after the last write.

## Bug

A pooled value that held the last user's data when it went back, reproduced in tiger's own
experiments (ENG-191 call 7), leaked that data to the next user. `TestZeroThenUseLeaksLastUser` shows
the user and data still in the buffer put back, where `TestUseThenZeroLeavesNothingBehind` shows
nothing left. `sync.Pool` does not promise to return the same object, so both functions return the
buffer they put back and the tests inspect that.

## Checklist

- Is the zeroing the last write to the value before Put, on every path?
- Does the zeroing cover the whole value (`*b = Buffer{}`), so a field added later is cleared?
- Is there a test that fills the value, returns it, and checks nothing of the last user remains?
- Does that test inspect the value put back, not whatever the next `Get` happens to return?
