# ledger

Declare each invariant once as a named ID, assert it at both ends of an encode and decode pair, and
give every invariant a test that violates it.

## Rules

TS-A07, TS-A09

## Gap

Tiger checks that every declared invariant is asserted somewhere in production code (TS-A07) and
that a test violates it (TS-A09). It cannot check that the assertion sits at both ends of a
boundary. `DecodeHeaderUnchecked` asserts the size and drops the checksum, so a corrupted frame
decodes and tiger stays silent, because the checksum invariant is still asserted by the encoder.
The symmetric-pair rule, TS-A08, is specified and not built.

`HeaderSizeBytes` is 12: 4 checksum, 4 length and 4 sequence. `Length` and `Sequence` are named
types so that a swapped argument is a compile error (TS-Q01). `EncodeHeader` frames length and
sequence into a `HeaderSizeBytes` buffer and computes the checksum over the payload fields, and
`DecodeHeader` reads a `Header` back out and refuses corrupt input. `DecodeHeaderUnchecked` still
asserts every invariant in production code (TS-A07) and still has a test that violates each one
(TS-A09), so `tiger check` passes while a corrupted frame decodes without complaint. The package
`inv` declares one `ID` per invariant, and every assertion site references it (TS-A07). The analyzer
counts the functions referencing each ID, and the string is what a failed assertion prints.

## Bug

No filed bug; the gap is the one-sided decoder, a shape the invariant rules cannot see.
`TestUncheckedDecodeAcceptsCorruptHeader` shows the one-sided decoder returning a
corrupted length.

## Checklist

- Does every function that reads a boundary type assert the same invariants as the function that writes it?
- Does each assertion state the property itself, so that it fails on bad input, not a tautology?
- Does each invariant have a test that violates it through the exported entry point?
- Are invariant IDs declared only in the `inv` package, so no assertion uses a string literal?
