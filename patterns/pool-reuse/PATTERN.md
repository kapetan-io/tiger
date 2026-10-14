# pool-reuse

Reset the whole pooled value before `Put`, so the next `Get` cannot see the last user's data, and
test it by inspecting the value you put back.

## Rules

TS-M05

## Gap

Tiger checks that a `Reset` call runs before `Put` (TS-M05). It cannot check that `Reset` clears
every field. `UseWithEmptyReset` calls `LeakyBuffer.Reset`, whose body is empty, and tiger stays silent
because of the method's name. `UseWithReset` calls `Buffer.Reset`, which assigns `Buffer{}`, so it
clears every field, including ones added later.

## Bug

A pooled value whose `Reset` has an empty body, reproduced in tiger's own experiments, leaked the last
user's data to the next user.
`TestUseWithEmptyResetLeaksLastUser` shows the user and data still in the buffer put back.
`sync.Pool` does not promise to return the same object, so both functions return the buffer they
put back and the tests inspect that.

## Checklist

- Does `Reset` assign every field, including slices and maps, or does it zero the whole value (`*b = Buffer{}`)?
- If a field is added to the pooled type, is it cleared without editing `Reset`?
- Is there a test that fills the value, returns it, and checks nothing of the last user remains?
- Does that test inspect the value put back, not whatever the next `Get` happens to return?
