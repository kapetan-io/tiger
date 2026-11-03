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

`Opener` starts a read of at most `limit` rows that sort after the row named `after`, the way a
query carries `WHERE key > ? LIMIT ?`. `Page.After` is the row to pass to the next call, and it is
empty when the stream ended before the page filled. `clampPage` keeps the requested size in
`pageMin`..`pageMax`, so a zero limit still moves, and the constant `_ = uint(pageMax - pageMin)`
stops the package compiling if the floor rises above the maximum. `ListPage` restates the clamped
limit in the loop header, so the loop ends at the limit whatever the stream does. `Scan` is false for
a read error as well as the end, so `ListPage` returns `lines.Err()` and a failed read does not pass
as the last page.

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
