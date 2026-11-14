# Storage format

Everything a store holds is in one file, `<path>/rodinia.wal`. This page
describes its layout, how the store opens it, how it survives crashes, how it is
compacted, and how values are encrypted. Code: `rodinia_ffi/src/record.rs`,
`src/wal.rs`, `src/legacy.rs`, `src/crypto.rs`.

## The file

```
┌────────────┬──────────┬──────────┬──────────┬─────
│ RDNA 0x02  │ record 1 │ record 2 │ record 3 │ …
└────────────┴──────────┴──────────┴──────────┴─────
   5 bytes
```

The file starts with the 5-byte header `RDNA\x02` (the last byte is the format
version), followed by records, one per change, in the order they happened. The
file only ever grows at the end, until compaction replaces it.

## A record

All integers are little-endian.

```
┌──────────┬──────────┬────────────────────────── body ──────────────────────────┐
│ body_len │ crc32    │ kind │ flags │ expires_at │ key_len │ key │ value        │
│ u32      │ u32      │ u8   │ u8    │ u64        │ u32     │     │              │
└──────────┴──────────┴──────┴───────┴────────────┴─────────┴─────┴──────────────┘
  frame (8 bytes)       body header (14 bytes)
```

| Field | Meaning |
|---|---|
| `body_len` | length of everything after the frame |
| `crc32` | CRC-32 of the body; a record whose checksum does not match is treated as the end of the log |
| `kind` | `1` set, `2` delete, `3` clear |
| `flags` | bit 0: the value is encrypted; bit 1: the record has an expiry |
| `expires_at` | expiry as milliseconds since the Unix epoch (0 when bit 1 is clear) |
| `key_len`, `key` | the key, UTF-8 |
| `value` | the value bytes, running to the end of the body; empty for delete and clear |

The value is always the last thing in its record. That is what lets the index
store just an offset and a length for it, and read it with one positioned read.

A 16-byte value under a 10-byte key takes 8 + 14 + 10 + 16 = 48 bytes.

## Opening a store

`Wal::open` (in `wal.rs`):

1. **Deletes a leftover `rodinia.wal.tmp`**: that is a compaction or migration
   that never finished, and the original file is still intact.
2. **Checks the header**:
   - `RDNA\x02`: the current format.
   - Missing, empty, or a partial header (a crash while creating the file): a
     fresh header is written and synced.
   - Anything else: an old-format log, which is migrated (below).
3. **Replays the log** in one forward pass through a 64 KiB buffered reader. Each
   set record puts or replaces its key in the index, a delete removes it, a
   clear empties it. Nothing but the index is kept: values stay on disk.
4. **Stops at the first record that is incomplete or fails its checksum.** If
   that is before the end of the file, the file is truncated to the last good
   record and synced. This is how a write cut short by a crash or power loss is
   discarded: a torn last record never becomes visible, and everything before it
   survives.
5. **Drops keys that have already expired**, and adds up `live_bytes`.

Replay cost is proportional to the file size, which is why compaction matters.

## Durability

Every mutating call takes `durable` (default `false`).

| | `durable: false` | `durable: true` |
|---|---|---|
| What happens before the call returns | the record is written to the file (handed to the operating system) | the record is written and `fdatasync`ed |
| Survives the app crashing or being killed | yes | yes |
| Survives power loss or an OS crash | the most recent writes may be lost (the log is cut back to the last complete record) | yes |
| Cost on a Galaxy S10e | about 5–8 µs | a few milliseconds |

`close()` syncs the file as well. Compaction and migration always sync, whatever
`durable` says, because they replace the whole file.

## Compaction

The log keeps every old version of every key until it is compacted. Compacting
writes a new file containing one set record per live key and swaps it in.

**When:**

- **Automatically**, after any write, once the file is larger than 1 MiB *and*
  dead bytes (file length − header − `live_bytes`) exceed live bytes. The 1 MiB
  floor stops small stores from compacting constantly; the dead-exceeds-live rule
  means the file is at most about twice the size of its live data, and each
  compaction is paid for by at least as many bytes of writes as it rewrites. A
  failed automatic compaction is ignored; the old file stays in use.
- **On request**, with `compact()`, which first purges expired keys.

**How** (`Wal::compact`), crash-safely:

1. Write the header and every live record to `rodinia.wal.tmp`, reading each
   value from the current file.
2. `fdatasync` the temporary file.
3. Rename it over `rodinia.wal`. A rename is atomic: at every moment the path
   holds either the complete old file or the complete new one.
4. `fsync` the directory, so the rename itself survives a power loss.
5. Reopen the file handles and give the index the new value offsets.

If anything fails before the rename, the temporary file is deleted and the old
file is untouched. If the process dies before the rename, the next `open`
deletes the temporary file.

Automatic compaction currently runs inline, under the write lock, during the
write that triggered it. Because of the syncs, that write can take 15–20 ms on a
phone. Moving it to a background thread is an open item (see
[Development](development.md#known-issues)).

## Expiry

A value written with a `ttl` stores an absolute expiry time: the wall-clock time
of the write plus the TTL. Changing the device clock therefore moves expiries
too.

Expired keys are removed lazily:

- A read (`get`, `contains`, list reads) that finds an expired key appends a
  delete record for it and publishes `KeyExpired`.
- `keys`, `length`, `getAll` and the preload skip expired keys without deleting
  them.
- `purgeExpired()` deletes all of them at once.
- Replay on `open` and compaction drop them.

## Encryption

Values written with `encrypted: true` are encrypted with AES-256-GCM before they
reach the log:

```
┌─────────┬──────────────┬───────────────────────────┐
│ version │ nonce        │ ciphertext + 16-byte tag  │
│ 0x01    │ 12 bytes     │                           │
└─────────┴──────────────┴───────────────────────────┘
```

- The nonce is 12 random bytes from the operating system for every write, so
  writing the same value twice gives different bytes.
- The key name is passed as *associated data*: it is authenticated, not
  encrypted. Copying an encrypted value to a different key name makes decryption
  fail, so values cannot be swapped between keys.
- The record's encrypted flag is set, and the value is decrypted on read.
- The cipher is built once when the key is set and reused for every value.

What is **not** protected:

- **Key names, expiry times and the encrypted flag are stored in plain text.**
  Do not put secrets in key names.
- Values written without `encrypted: true` are plain text in the file.
- The key stays in process memory while it is set. The buffer passed to
  `setEncryptionKey` is wiped after use (`zeroize`), but the stored key and the
  prepared cipher are not wiped when they are replaced or after `lock()`.
- Decrypted values in the Dart cache are ordinary Dart objects in memory until
  they are evicted, dropped by `lock()`, or garbage-collected.
- Rodinia does not store the encryption key anywhere. The app must keep it, for
  example in the platform keystore through `flutter_secure_storage`, and call
  `setEncryptionKey` after each start.

## The old format and migration

Before this format, the log was a sequence of `u32 length` + bincode-serialized
`WalRecord` values with no header (`legacy.rs`). When `open` finds a file that
does not start with `RDNA`, it reads that format once, keeps the live,
unexpired entries, writes them in the current format to `rodinia.wal.tmp`, syncs
it, and renames it into place, the same crash-safe swap as compaction. Encrypted
values migrate as they are: the ciphertext format did not change.
