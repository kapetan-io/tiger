# clamped-limit

Clamp a limit that arrives from a request, config or wire value once, where it enters the program,
to a named maximum small enough to finish in useful time and a floor of one. Pass only the clamped
value on to the loop.

## Rules

TS-S02

## Gap

Tiger checks that a loop limit is a constant, a length, or clamped against a declared maximum
before it reaches the loop (TS-S02, limit flow). It cannot check that the maximum is small, or
that every path from the outside value to the loop goes through the clamp. `ListPageHuge` clamps
against `pageHuge`, about a billion rows, so tiger accepts it, and `TestListPageHugeNeverFinishes`
shows it still never finishes on a stream with enough rows. `pageHuge` is a clamp in form only: the
loop ends at it, after about a billion rows. The constant `_ = uint(pageHuge - pageMax)` keeps
`pageHuge` above `pageMax`, so the broken shape stays a real clamp.

`ListPage` clamps the client's `Limit` where the request enters and passes only the clamped value to
`countRows`. `pageMax` is small enough that a full page finishes in well under a second. `clampPage`
keeps the page size in 1..`pageMax`, and its floor turns zero and negative requests into one row,
not an empty or runaway page.

## Bug

ENG-193 (git-server pack ingest sized by a count the client wrote), ENG-194 (six unbounded list
endpoints in querator) and ENG-195 (querator create-queue looping once per requested partition).
Each let a huge outside value drive a loop for practically ever. `TestListPageClampsTheLimit`
shows `ListPage` returning at most `pageMax` rows for huge, zero and negative limits.

## Checklist

- Does every entry point that takes a limit clamp it before any loop or allocation sees it?
- Is the maximum a named constant small enough that a full page finishes in useful time?
- Does the clamp also set a floor, so zero and negative limits give a defined result?
- Is there a test with a huge limit and a non-positive limit that runs against more data than a page?
- Does the loop's own test finish without a timeout, rather than waiting on a stuck loop?
