# capped-worklist

Replace recursion with a worklist, cap it, and make hitting the cap return an error, because a cap
that stands in for a proof that the loop ends must say when it fires.

## Rules

TS-S02, TS-S01

## Gap

TS-S01 sends recursion to a worklist and TS-S02 asks a counter cap to say when it ends the loop.
Tiger cannot see that a cap which passes its check is still silent. `ReadRefsTruncated` ends on a
`bufio.Scanner`, which tiger treats as an outside stream, so it reads `limit` as a page size and
stays silent while the caller gets a short list that looks complete. `Walk` shows the shape to copy:
it returns `ErrWalkTooLarge` when it reaches `nodesMax` with nodes left, and it does not fire when
the graph has exactly `nodesMax` nodes.

## Bug

git-server's tag-peel denial of service (ENG-159): a loop capped at a limit stood in for a proof
that it ends, and hitting the cap was silent, so an attack turned into truncated output.
`TestReadRefsTruncatedHidesLongInput` shows five refs read with a limit of two returning two with no
signal. `TestWalkReportsCycle` shows `Walk` failing on a cycle instead.

## Checklist

- Does hitting the cap return an error the caller must handle, not a short result?
- Does a graph of exactly the cap size still succeed, so the error means the walk was cut short?
- Is there a test that reaches the cap and one that stays below it?
- Is the cap a named constant with a stated reason, and does a compile-time check tie it to the memory a walk may use?
- Does the caller of a capped read treat a full page as possibly cut short?
