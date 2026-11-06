# logstore: a columnar store for nginx access logs

Status: prototype. Nothing in production reads or writes it yet.

## Why replace Bleve

The current indexer writes one Bleve document per log line, with an inverted
index on almost every field. That is a full-text search engine. The workload is
log analytics. A benchmark on a synthetic nginx combined corpus (217 B/line)
measured both designs:

| | Bleve (current mapping) | Columnar prototype |
|---|---|---|
| Index size | 1615 B/line, 7.45x the raw log | 47 B/line, 0.22x the raw log (raw line included) |
| Indexing CPU | ~310 µs/line | ~2 µs/line |
| Heap while indexing | ~52 KiB per buffered document | one block of rows |
| Dashboard hour/day scan, 1M lines | 61.8 s | 0.1 s |
| Top-N facets, 1M lines | 551 ms | 49 ms |
| Distinct IPs, 1M lines | 384 ms | 19 ms |
| Free-text word, 1M lines | 13 ms | 301 ms single-threaded (see "Free text") |

No mapping change closes this gap. Dropping term vectors, the `raw` index, or
stored fields each saves under 15%, because the cost is the
document-per-line, posting-list-per-field model itself.

Every read in the product is one of the following (inventory taken from
`searcher/`, `analytics/` and `api/nginx_log/`):

- a time range filter
- exact-value filters: ip, method, status, country, province, browser, os,
  device, log group, file
- phrase filters on path, user agent and referer
- one free-text query over the raw line
- top-N term counts
- distinct counts (UV, unique pages)
- hour/day/minute buckets
- min/max/sum of bytes and request time
- newest-first pages

Apart from free text, these are all column scans. A column store answers them
by touching only the columns a query names. It also skips whole blocks by
their time range and by their dictionaries.

## Layout

```
<index_path>/logstore/
  <group-id>/                 one directory per log group (main_log_path)
    <file-id>.seg             one segment per physical log file
```

The directory names are short hashes. The real paths are kept in each segment
header, so opening the store rebuilds the catalog from the files alone. No
side database is needed.

A **segment** is an append-only file:

```
segment header : magic "NXLSEG" | version | uvarint-len group path | uvarint-len file path
block*         : magic "BLK1" | uvarint headerLen | header | payload* | crc32c(header+payloads)
```

A **block** holds up to 64k rows. Rows inside a block are not required to be
sorted, because nginx writes lines at request completion and timestamps are
only roughly ordered. The block header carries:

- rows, minTs, maxTs: the zone map for time pruning
- sourceEnd: the byte offset in the source log just after the last line in
  this block
- the column directory: column id, encoding, compressed length, raw length

Every payload is compressed with zstd separately, so a query decompresses only
the columns it reads.

| Column | Encoding |
|---|---|
| timestamp | zigzag varint delta from the previous row |
| status | uint16 |
| bytes_sent | uvarint |
| request_time, upstream_time | milliseconds as uvarint (upstream: 0 = absent, else ms+1) |
| ip, method, path, protocol, referer, user_agent, browser, browser_version, os, os_version, device_type, region_code, province, city, c1..c4 | per-block dictionary (one payload) + fixed-width ids (uint8/16/32, a second payload) |
| raw | `\n`-joined lines |

`file_path` and `main_log_path` are not stored per row. The segment they live
in implies them.

### Why per-block dictionaries

Equality and substring filters are evaluated once per dictionary entry, not
once per row. A block whose dictionary lacks the filtered value is skipped
before its id payload is read. Top-N counting is an array increment per row,
keyed by id, and maps are only touched once per distinct value per block.

## Ingest

- The parser output (`indexer.LogDocument`) is appended to an in-memory block
  builder. When the builder reaches its row limit, or the caller flushes at the
  end of a read, the block is encoded and appended with one `write`.
- Memory is bounded by one block per file being ingested. No merge runs in the
  background, and no batch amplification needs estimating.
- **The store is the source of truth for ingest progress.** Every block
  records `sourceEnd`. `Segment.SourceOffset()` tells incremental indexing
  where to resume. The DB position and the index can therefore never disagree
  after a crash. Today they can, which produces duplicate or missing lines.
- **Crash safety:** open scans block headers and checks each block's CRC. A
  torn tail is truncated. Its rows are lost, but `sourceEnd` rolls back with
  them, so they are re-read.
- **Small incremental appends** create small blocks. `Compact` rewrites a
  segment, merging adjacent small blocks up to the row limit. It writes a
  temp file and renames it into place under the segment's write lock.
- **Deleting a rotated-away file** removes one segment file. **Deleting a
  group** removes one directory. These replace `deleteDocsByTerm`, which
  pages through Bleve 1000 ids at a time.

## Query execution

1. **Plan.** Select segments by group and by file path. Then select blocks
   whose `[minTs, maxTs]` overlaps the requested range.
2. **Filter.** Each block is evaluated independently on a worker pool sized to
   the CPUs the process may actually use (`cgroup.AvailableCPUs`).
   - Each filter turns the block's dictionary into a per-id match table.
     Then the id column yields a row selection.
   - Filters short-circuit: an empty selection stops reading columns.
   - Time is checked per row only when the block straddles a range bound.
3. **Operate on the selection.**
   - **Count:** popcount.
   - **Facets:** per-id counters, merged into a map once per block.
   - **Distinct:** per-block dictionary values, restricted to the selected ids,
     inserted into a 64-bit hash set.
   - **Stats:** min/max/sum over the numeric columns.
   - **Search:** a top-k heap across blocks, with k = offset+limit, ordered by
     the sort key. Blocks are visited newest-first for timestamp sorts. Once
     the heap is full, a block whose maxTs is older than the heap's worst
     entry cannot contribute and its rows are skipped. Only the winning rows
     are materialized, by decoding their blocks' display columns.
   - **Scan:** a per-block callback with decoded column views, for
     aggregations the engine does not know (the dashboard's hour/day/minute
     buckets).

`total` is always exact, because counting a selection costs one popcount per
block.

## Free text

`Query.Text` is split on whitespace. Every term must occur in the line
(AND), compared case-insensitively as a substring. This differs from today's
Bleve behavior in two ways:

- Today, `MatchQuery` on `raw` ORs the analyzed tokens, so `"GET 500"`
  matches every GET line.
- Today, a token never matches part of a word: `api` does not find `/apiv2`.

The search box's placeholders already suggest substring semantics.

Path, user agent and referer filters become case-insensitive substrings,
checked once per dictionary entry. They are cheap regardless of corpus size.

Raw text is the only column whose scan cost grows with bytes rather than
distinct values. Two things keep it fast:

1. **Gram filter.** Each block stores, as a third payload of the raw column,
   a bitmap of the lower-cased trigrams its lines hold, at about eight bits
   per distinct trigram (1 KiB to 512 KiB before compression). A term with a
   trigram missing from it cannot occur in the block, so the block is skipped
   without decompressing its text, and an excluded term it rules out removes
   nothing. Trigrams keep substring semantics exactly, which a token filter
   would not: `api` still finds `/apiv2`. False positives only cost a scan.
   Blocks written before the filter, and readers that predate it, scan as
   before.
2. **Scan.** The blocks left decompress their raw column, lower-case it
   eight bytes at a time (not at all when no term has a letter), and run
   `bytes.Index` over the whole buffer. A match is mapped to its row by
   counting newlines since the previous one. At most `TextScanParallelism`
   blocks scan at once, which bounds the memory a query holds.

On 1M lines (`BenchmarkTextSearch`, Apple M2 Pro), before and after:

| Term | Before | After |
|---|---|---|
| in every block (`chrome`) | 247 ms | 74 ms |
| scattered over every block (`/api/v1/users/4242`) | 338 ms | 133 ms |
| in one window of the log (`sqlmap`) | 254 ms | 16 ms |
| absent (`wp-login.php`) | 195 ms | 0.08 ms |

Building the filter costs about 0.2 µs per ingested line and adds 0.03 B per
line on disk.

## Distinct counts at scale

The prototype counts exactly, with a hash set of 64-bit `maphash` values. That
is about 16 B per distinct value. Ten million distinct paths need ~160 MB.
The production version should switch to HyperLogLog++ (about 1.6% error at
16 KiB) above a threshold, for example 1M distinct values.

## Integration plan

The prototype ships `searcher.ColumnarSearcher`, which implements
`SearcherInterface` on top of `logstore`. It adds `Cardinality`, which takes
a `SearchRequest` and applies all of its filters, and `Scan`. The migration
steps:

1. Write path: the indexer appends to `logstore` instead of Bleve, and
   `LogFileManager` reads its resume offsets from `Segment.SourceOffset()`.
2. Read path: `modern_services.go` builds a `ColumnarSearcher`. The dashboard
   time-bucket scan moves to `Store.Scan`, and the `search_after` paging loop
   disappears.
3. Bump `indexStorageVersion`. The existing `PrepareIndexStorage` already
   wipes stale index data and triggers a rebuild.
4. Delete the Bleve indexer, `GroupedShardManager`, the memory limiter, and
   the Bleve-specific searcher, counter and cache code.

## Prototype status

What exists and is tested:

- `logstore`: block format, segment files, writer, crash-tail recovery,
  compaction, delete by file and by group, and a parallel engine. The engine
  supports Count, Facets, Distinct, Stats, Search (offset and cursor paging,
  any numeric or dictionary sort) and Scan.
- `searcher.ColumnarSearcher`: maps `SearchRequest` to the store.
  - Bleve parity: `TestColumnarSearcherMatchesBleve` runs the API's
    structured requests against both engines on the same corpus. Hits,
    ids, stored fields, facets, stats and the `search_after` walk are equal.
  - To stay drop-in, it reports empty values the way Bleve does: as `""`
    fields and as a `""` facet term.
- `indexer.LogDocumentToRow`: converts the parser's document to a row.
- The engine is checked against a brute-force reference on 150 random filter
  combinations (`TestQueriesMatchReference`). Deliberately breaking a time
  bound or a dictionary filter makes that test fail.

In-repo benchmark: 100k combined-format lines from the real `LogDocument`
type, Apple M2 Pro. Run it with:

```
GOWORK=off go test -tags=unembed -run '^$' -bench 'BenchmarkIngest|BenchmarkSearchRequests' -benchtime=3x ./internal/nginx_log/searcher/
```

| | Bleve | logstore | Ratio |
|---|---|---|---|
| Ingest CPU | 134 µs/doc | 0.98 µs/doc | 136x |
| Ingest allocations | 9.2 GB | 216 MB | 42x |
| On disk | 1508 B/doc (8.6x raw) | 28.5 B/doc (0.16x raw) | 53x |
| Log viewer page (hits + ip/path facets + stats) | 1479 ms | 19.9 ms | 74x |
| Viewer, status + path filter | 15.7 ms | 6.3 ms | 2.5x |
| Dashboard browser/os/device facets | 75.8 ms | 0.56 ms | 135x |
| Dashboard top 100 URLs | 64.7 ms | 5.7 ms | 11x |
| Geo world facet | 84.1 ms | 0.31 ms | 275x |
| Page at offset 20000 | 116 ms | 22 ms | 5x |

## Wiring status

In production now:

- **Write path.** `indexer.ColumnarIndexer` replaces `ParallelIndexer` in
  `modern_services.go`. The task scheduler, the cron incremental job, the
  rebuild API and `LogFileManager` reach it through the `indexer.LogIndexer`
  interface.
  - A read that fails rolls back the blocks it already sealed
    (`Writer.Abort`), so a retry from the same `LastPosition` cannot
    duplicate rows.
  - Each read records its end offset on its last block (`SourceOffset`).
- **Read path.** `searcher.ColumnarSearcher` reads the indexer's store. Rows
  are visible as soon as they are written, so `UpdateSearcherShards` is a
  no-op and the 5-minute stale cache is gone.
  - The API's unique-visitor and unique-page counts use `Cardinality`, so
    they now honor every filter of the search.
- **Dashboard.** The hour/day/minute buckets come from `Store.Scan`, and
  total UV from `Cardinality`. `TestDashboardFromColumnStoreMatchesBleve`
  checks the result against the Bleve path.
- **Migration.** `indexStorageVersion` is 4. On first start,
  `PrepareIndexStorage` removes the Bleve shard directories, resets the
  metadata, and every log group is re-indexed into `logstore/`.

Still to do:

- **Delete the Bleve implementation.** That covers `ParallelIndexer`,
  `GroupedShardManager`, the memory limiter, the Bleve searcher, `Counter`
  and the cache. Their tests go with them, which needs the maintainer's sign-off.
- **Use `SourceOffset` as the resume position.** The incremental job still
  resumes from `LastPosition` in the database.

## Out of scope for the prototype

- HyperLogLog
- raw-text bloom filters
- a decoded-column cache
- mmap
- multi-process locking (one process owns the directory, as today)
