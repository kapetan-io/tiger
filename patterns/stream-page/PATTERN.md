# stream-page

Bound a page loop over a stream you do not own with the clamped limit in its header, and return the
row the next call resumes after, so a client can walk the whole listing.

## Rules

TS-S02

## Gap

Tiger accepts a loop whose header restates a limit over a stream declared outside the module, such as
a `bufio.Scanner` or a database cursor, and treats the limit as a page size. It cannot check that the
page can be continued. `ListFromStart` has the same bounded loop and takes no resume point, so every
call starts at the top and a client paging through the listing reads the first page forever.

## Bug

Querator's list loops scan whole partitions. Restating the limit in the loop header bounds each
call, and a page without a resume point sends clients back to the top of the partition for each
page. `TestListFromStartRepeatsThePageForever` shows the second call returning the first call's rows,
where `ListPage` returns the rows after them.

## Checklist

- Does the page return the row the next call resumes after, and an empty one when the stream ended?
- Does the stream open from that row, so the next page does not rescan the rows before it?
- Is the limit clamped once, where it enters, before it reaches the stream or the loop header?
- Is there a test that walks the whole listing page by page and sees each row once?
- Is there a test with a stream that ignores the limit and never ends, counting the lines read?
