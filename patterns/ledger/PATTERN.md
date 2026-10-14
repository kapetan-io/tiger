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

## Bug

No filed bug; the gap is the one-sided decoder, a shape the invariant rules cannot see.
`TestUncheckedDecodeAcceptsCorruptHeader` shows the one-sided decoder returning a
corrupted length.

## Checklist

- Does every function that reads a boundary type assert the same invariants as the function that writes it?
- Does each assertion state the property itself, so that it fails on bad input, not a tautology?
- Does each invariant have a test that violates it through the exported entry point?
- Are invariant IDs declared only in the `inv` package, so no assertion uses a string literal?
