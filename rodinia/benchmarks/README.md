# rodinia_benchmarks

Head-to-head benchmarks of `rodinia_flutter` against the most widely used Flutter
key-value storage packages, measured from Dart on a real device/desktop target.

| Group | Packages |
|---|---|
| Plain | `rodinia`, `hive_ce`, `shared_preferences` (`SharedPreferencesWithCache`), `sqflite` (one `kv` table), `get_storage` |
| Encrypted | `rodinia` (`encrypted: true`, AES-256-GCM), `hive_ce` (`HiveAesCipher`), `flutter_secure_storage` |
| Cold start | `rodinia`, `hive_ce`, `shared_preferences`, `sqflite`, `get_storage`, `flutter_secure_storage` |

## Run

Always run in **profile** mode — debug builds run on the JIT and their numbers are meaningless.

```sh
# Everything, on a connected Android device (from this directory):
tool/run_all.sh <device-id>              # suite + cold start for every store
SUITE=0 tool/run_all.sh <device-id> hive_ce   # cold start for one store only

# Only the operation suite, on any target:
flutter drive --profile -d <device> \
  --driver=test_driver/integration_test.dart \
  --target=integration_test/benchmark_test.dart
```

`tool/run_all.sh` writes one log per app launch to `build/bench/` and every
result to `build/bench/results.tsv`. It keeps the screen awake, dismisses the
lock screen before each launch, and stops if the host's free disk drops below
300 MB. A single `flutter drive` prints its results as Markdown tables and writes
them to `build/benchmark_results.json`.

## What is measured

Every operation runs against 500 keys (100 for `flutter_secure_storage`, whose
keystore round-trips take milliseconds). Each scenario runs one warm-up round and
three measured rounds; the median µs/op is reported. The store is cleared and
re-seeded outside the timed region before every round. Each result is printed as a
`RESULT` line as soon as it is measured, so a dropped device connection keeps
everything measured so far.

Keep the device screen on for the whole run: a locked screen backgrounds the app
and can drop the driver connection (`tool/run_all.sh` handles this).

| Scenario | Timed operation |
|---|---|
| write 16 B / write 1 KiB | insert `k<i>` with a 16-byte / 1 KiB string |
| read 16 B / read 1 KiB | read an existing `k<i>` |
| delete | remove an existing `k<i>` |
| read all | load every entry as a `Map<String, String>` (20 calls) |

### Cold start

Each store is measured in two app launches, chosen with `--dart-define=BENCH=`:
`fill:<store>` writes 1,000 values of 1 KiB and exits; `open:<store>` then runs in
a fresh process and times opening the store, waiting for any background loading
the store starts after opening (*ready*; rodinia's preload), the first read, and
reading every key once, and reports how much the process's resident memory (RSS) grew from just
before opening to after the last read. Because every store gets its own process,
one store's memory never shows up in another's number. RSS growth includes the
package's own startup (loading a native library, a platform channel, a database
engine), which is part of what an app pays for it.

`flutter drive` uninstalls the app when it finishes, which deletes its data, so the
fill launch passes `--keep-app-running`.

Before timing, each store passes a round-trip check (write → read → read-all →
delete) so a broken adapter cannot post a fast number.

Synchronous APIs (rodinia, Hive reads, `SharedPreferencesWithCache` reads) are
timed without an `await`; asynchronous ones are awaited per operation, as an app
would. Each timed run ends with `settle()`, which waits for writes a store does
in the background: `get_storage` returns from `write()` before its file is
written, so without this its disk work would fall outside the timer.

## Reading the results fairly

- **Durability differs.** `sqflite` commits a journaled SQLite transaction per write;
  rodinia and Hive append to a log and flush without `fsync`; `shared_preferences`
  writes go through a platform channel to `NSUserDefaults`/`SharedPreferences`;
  `get_storage` keeps everything in memory and rewrites its whole JSON file after
  a batch of writes, plus an unawaited backup copy the timer cannot see.
- **Memory is one sample per store.** RSS moves with garbage collection, so treat
  differences of a megabyte or two as noise.
- **`shared_preferences` and `flutter_secure_storage` open lazily.** Part of their
  load cost lands on the first read rather than on open, so compare open and first
  read together.

## Results

Samsung Galaxy S10e (SM-G970U1), profile build, Flutter 3.47.9, one full
`tool/run_all.sh` run on 2026-11-14. `rodinia` uses its defaults (8 MiB read
cache, preload on); `rodinia (no cache)` sets `cacheBytes: 0`. Regenerate these
tables with `tool/tables.py`.

### Plain storage, 500 keys

Median µs per operation, lower is better. Fastest in bold.

| | `rodinia` | `rodinia (no cache)` | `hive_ce` | `shared_preferences` | `sqflite` | `get_storage` |
|---|---:|---:|---:|---:|---:|---:|
| write 16 B | 8.03 | **5.12** | 116.4 | 7,702 | 13,489 | 1,587 |
| read 16 B | **0.39** | 3.19 | 0.99 | 0.43 | 2,836 | 0.67 |
| write 1 KiB | 48.9 | **47.5** | 111.7 | 20,545 | 14,426 | 16,118 |
| read 1 KiB | **0.67** | 11.7 | 0.86 | 0.75 | 2,597 | 0.69 |
| delete | 7.29 | **5.70** | 123.7 | 10,261 | 13,262 | 905.6 |
| read all | 77.9 | 670.3 | **53.2** | 192.0 | 6,376 | 75.4 |

### Encrypted storage, 500 keys (flutter_secure_storage: 100)

Median µs per operation, lower is better. Fastest in bold.

| | `rodinia (AES-GCM)` | `hive_ce (AES-CBC)` | `flutter_secure_storage` |
|---|---:|---:|---:|
| write 16 B | **6.85** | 114.9 | 2,464 |
| read 16 B | **0.27** | 0.85 | 1,885 |
| write 1 KiB | **76.0** | 237.2 | 1,397 |
| read 1 KiB | **0.87** | 0.89 | 1,928 |
| delete | **5.79** | 128.8 | 1,448 |
| read all | 78.8 | **55.5** | 17,356 |

### Cold start, 1,000 × 1 KiB, fresh process per store

One sample per store, lower is better. Lowest in bold. *ready* is when background loading after open has finished; only rodinia loads in the background (its preload), so for the others it equals *open*. Reads start after *ready*.

| | `rodinia` | `hive_ce` | `shared_preferences` | `sqflite` | `get_storage` | `flutter_secure_storage` |
|---|---:|---:|---:|---:|---:|---:|
| open (ms) | 7.78 | 48.5 | 455.2 | 13.1 | 17.2 | **0.02** |
| ready (ms) | 39.1 | 48.5 | 455.2 | 13.1 | 17.2 | **0.03** |
| first read (µs) | 33.0 | **14.0** | 20.0 | 1,536 | 19.0 | 110,738 |
| read every key (ms) | 6.24 | 2.36 | **0.64** | 1,173 | 1.31 | 1,081 |
| RSS growth (MiB) | 16.1 | 10.4 | 18.3 | 25.5 | **8.20** | 18.1 |

### What the numbers say

- **Writes:** rodinia is the fastest at every write and delete, plain and
  encrypted: about 15× ahead of Hive on 16 B writes and deletes, 2.3× (plain) to
  3.1× (encrypted) on 1 KiB writes, and 2–4 orders of magnitude ahead of the
  packages that rewrite a file or commit a transaction per write
  (`shared_preferences`, `sqflite`, `get_storage`).
- **Reads:** cached rodinia reads are the fastest measured: 0.39 µs for 16 B and
  0.67 µs for 1 KiB plain, against 0.43–0.99 µs for `shared_preferences`,
  `get_storage` and Hive, and ahead of Hive encrypted too. A value read straight
  after `set` returns the same `String` that was written, with no decoding.
- **Read all:** Hive is ahead (53 µs against 78 µs for rodinia and 75 µs for
  `get_storage`). Without the cache rodinia takes 670 µs because every value
  comes from disk.
- **Cold start:** rodinia's `open` returns in 8 ms and its preload finishes at
  39 ms, against 13–48 ms for `sqflite`, `get_storage` and Hive, which load during
  `open`. Reading every key once after that takes 6 ms; the stores that keep
  decoded strings in memory take 0.6–2.4 ms, because rodinia decodes each value's
  UTF-8 on its first read.
- **Memory:** rodinia's RSS grew 16 MiB with 1,000 × 1 KiB loaded, more than
  `get_storage` (8 MiB) and Hive (10 MiB) and less than `shared_preferences`,
  `flutter_secure_storage` (18 MiB) and `sqflite` (26 MiB). The preload is part of
  this; `preload: false` trades it back for slower first reads.
- **Run-to-run noise:** single cold-start samples move between runs (Hive's open
  took 24 ms in an earlier run and 48 ms in this one), so read differences under
  about 2× in that table as noise.
