# sorted-map-output

Never let a map decide what is printed. Give each entry a sort key that is unique, such as a sequence
number assigned when it was added, and print by ranging over the sorted keys.

## Rules

TS-T02

## Gap

Tiger flags a map range or `maps.Keys` that reaches an output unsorted, and checks that a sort
comparator covers every field of what it sorts. It cannot check that the map's key was unique to begin
with. `IndexByPosition` keys notes by `Pos()`, and a `Statement` and the `Call` it wraps share one
position, so two entries write the same slot and the one visited last wins. `RenderByPosition` then
sorts the positions and tiger stays silent, because every sort it sees is complete. This is a documented
known miss of TS-T02: two keys with one value write one slot.

## Bug

Go issues #27013 and #30202: go/ast's `CommentMap.String` printed entries in an order that varied
between runs, because its key is an interface over pointers with no order, and position alone does not
identify a node. `TestIndexByPositionLosesANote` shows `RenderByPosition` returning two different
outputs over 200 runs, and `TestDocumentRenderIsStable` shows `Document.Render` returning one.

## Checklist

- Does the key that is sorted differ for every entry the map can hold, not just for the entries in the test?
- If entries are re-keyed into another map, can two source entries land on the same new key?
- For pointer or interface keys, is the sort key a sequence number or another field that is unique, not a position or a name?
- Is the output compared in a test that runs the function many times, with entries that share a position or name?
- Does the test also assert the exact expected output, so a stable but wrong order fails?
