# filtered-page

Count the rows a filtered list keeps against the page limit, count the rows it reads against a
separate scan limit, and return an error when the scan limit stops the loop.

## Rules

TS-S02

## Gap

Tiger checks that the cursor loop has a bound in its header (TS-S02). It cannot check what the bound
counts. `ListCountingRows` counts every row read against the page limit, so rows the filter skips use
up the page, and tiger stays silent because a page limit over a stream from outside the module is
accepted as a bound. The right shape in `List` keeps two
limits: `len(names)` against the page limit, clamped to `pageMax`, and rows read against `scanMax`,
returning `ErrScanLimit` when it is hit. Tiger sees only the second bound.

## Bug

Querator's Badger list loop filtered rows before counting them, and returned a short page while more
matches existed further on. `TestListCountingRowsReturnsEmptyPage` shows `ListCountingRows` returning
an empty page while 5 matches sit at the end of a 100 row table.

## Checklist

- Does the page limit count rows kept by the filter, not rows read?
- Does a second limit bound the rows read, and does the loop return an error when it is hit?
- Is the page limit clamped to a maximum, and does a limit of zero or less return nothing?
- Is the scan limit at least the page maximum, so a dense table can fill a page?
- Is there a test with matches only at the end of the table?
