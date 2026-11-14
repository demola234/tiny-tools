# Dart API

Everything an app uses is the `RodiniaStore` class in
`package:rodinia_flutter/rodinia_store.dart`. Apart from `open` and
`preloaded`, every method is synchronous: it returns its result directly,
not a `Future`.

## Opening

```dart
import 'package:path_provider/path_provider.dart';
import 'package:rodinia_flutter/rodinia_store.dart';

final dir = await getApplicationDocumentsDirectory();
final store = await RodiniaStore.open(dir.path);
```

`open(path, {cacheBytes = 8 MiB, preload = true})`:

- **`path`**: a directory in the app's private storage. The log is
  `<path>/rodinia.wal`; the directory is created if missing.
- **One store per process.** The first `open` opens the store; later calls,
  from any isolate, return new handles to the same store and ignore `path`. You
  cannot have two stores in different directories in one app.
- **`cacheBytes`**: the size cap of this handle's read cache. `0` turns the
  cache off: every read then goes to Rust and the disk.
- **`preload`**: fill the cache in the background right after opening, up to
  `cacheBytes`, so the first reads after start-up don't each go to disk. This
  spends up to `cacheBytes` of memory straight away. Encrypted values are not
  preloaded.
- **`preloaded`**: a `Future<void>` that completes when that fill is done.
  You never have to wait for it; reads before then are correct, just not yet
  served from the cache.

## What you can store

`set<T>(key, value)` turns `value` into bytes; `get<T>(key)` turns them back.
**`T` on `get` must match what you stored.**

| You pass to `set` | Stored as | Read back with |
|---|---|---|
| `String` | its UTF-8 bytes | `get<String>` |
| `int`, `double`, `bool`, `null` | JSON text, e.g. `28`, `true` | `get<int>`, `get<double>`, `get<bool>`, `get<num>` |
| `Map`, `List` (of JSON-encodable values) | JSON text | `get<Map<String, dynamic>>`, `get<List<dynamic>>` |
| `Uint8List`, `List<int>` | the bytes as they are | `get<Uint8List>` (or `get<List<int>>`, which also returns a `Uint8List`) |
| any other object | `jsonEncode(value)`, so it needs a `toJson()` | `get<Map<String, dynamic>>`, then build your object from it |

Things that catch people out:

- A `String` is stored raw, not as JSON. `get<String>` returns it; `get<dynamic>`
  tries to JSON-decode it and throws a `FormatException` for ordinary text.
- JSON gives back `Map<String, dynamic>` and `List<dynamic>`. `get<Map<String, int>>`
  fails with a cast error; read `Map<String, dynamic>` and convert.
- `2.0` is stored as `2.0` and reads back as a `double`; `get<int>` on it fails.
- `get<String>` on a number or a JSON value returns its JSON text (`'28'`).
- `get<Uint8List>` returns a copy, and maps and lists are decoded fresh on every
  read, so changing what you get back never changes the store.
- A value read straight after `set` is the very object you passed: the same
  `String`, number or boolean.

## Reading and writing

```dart
store.set('profile', {'name': 'Tom', 'age': 28});
final profile = store.get<Map<String, dynamic>>('profile');

store.contains('profile'); // true
store.delete('profile');   // true: something was removed
store.get<String>('missing'); // null
```

| Method | Returns | Notes |
|---|---|---|
| `set<T>(key, value, {ttl, encrypted, durable})` | — | replaces any existing value, its expiry and its encryption |
| `get<T>(key)` | `T?` | `null` if missing or expired |
| `contains(key)` | `bool` | whether the key holds a live value |
| `delete(key, {durable})` | `bool` | `false` if there was nothing to delete |
| `getAll<T>()` | `Map<String, T>` | every live key; every value must be readable as `T`; decrypts encrypted values and throws `StoreError_EncryptionKeyNotSet` if any are encrypted and no key is set |
| `keys` | `List<String>` | live keys, in no particular order |
| `length` | `int` | number of live keys |
| `clear({durable})` | — | deletes every key and value; the encryption key stays set |

## Expiry

```dart
store.set('otp', '123456', ttl: const Duration(minutes: 5));
```

- The expiry is fixed when you write: wall-clock time plus `ttl`. Writing the key
  again replaces it (a `set` without `ttl` makes the key permanent).
- After it passes, the key reads as missing everywhere: `get`, `contains`,
  `keys`, `length`, `getAll`.
- Expired data is removed from the file lazily, when it is next touched;
  `purgeExpired()` removes all of it now and returns how many keys went.

## Encryption

```dart
final key = RodiniaStore.generateEncryptionKey(); // 32 random bytes
// Keep `key` somewhere safe, e.g. flutter_secure_storage. Rodinia does not.
store.setEncryptionKey(key);

store.set('token', 'abc', encrypted: true);
store.get<String>('token'); // 'abc'
```

- Values are encrypted with AES-256-GCM; see
  [Storage format](storage-format.md#encryption) for what is and isn't protected
  (key names are not encrypted).
- `encrypted` is per value. Plain and encrypted values can share a store.
- The key is held in memory only. Set it again after every app start, before
  touching encrypted values.

The three key operations do different things:

| Call | Data | Key |
|---|---|---|
| `clear()` | **all deleted** | stays set |
| `lock()` | kept | forgotten; encrypted values throw `StoreError_EncryptionKeyNotSet` until it is set again; plain values still read |
| `setEncryptionKey(other)` | kept | replaced; values written under the old key now fail to decrypt with `StoreError_AuthenticationFailed` (there is no re-encryption) |

`lock()` and `setEncryptionKey` also drop every decrypted value from every
handle's cache.

## Durability

Every mutating method takes `durable` (default `false`):

```dart
store.set('balance', 100, durable: true); // returns after an fsync
```

- `false`: the write is in the file when the call returns and survives the app
  being killed, but the last writes before a power loss or OS crash may be
  lost. A few microseconds.
- `true`: the call returns only once the write is on stable storage. A few
  milliseconds; use it for data you cannot afford to lose.

## Counters

```dart
store.increment('launches');           // 1
store.increment('launches', delta: 5); // 6
store.get<int>('launches');            // 6
```

- A missing or expired key counts as 0. The result saturates at the `int` limits
  instead of overflowing.
- The value is stored as decimal text, so `get<int>` and `get<String>` both read
  it, and `set('n', 5)` followed by `increment('n')` works.
- `increment` on a value that is not an integer throws
  `StoreError_InvalidCounterValue`. That includes encrypted values: counters are
  always stored unencrypted.
- **Each `increment` clears any expiry on the key.**

## Conditional write

```dart
final created = store.setIfAbsent('install_id', uuid); // true the first time
```

Writes only if the key holds no live value, in one atomic step, and returns
whether it wrote. Takes the same `ttl`, `encrypted` and `durable` options as `set`.

## Lists

```dart
store.listPush('recent', 'a');
store.listPush('recent', 'b');
store.listGet<String>('recent');        // ['a', 'b']
store.deleteFromList('recent', 'a');    // true
```

- A list is stored as one value; each push reads, appends and rewrites it, so a
  push costs O(list size). Keep lists short (recent items, small queues).
- Items are encoded like `set` values, and `listGet<T>` decodes every item as `T`.
- `deleteFromList` removes every item equal to the given one (compared as
  encoded bytes) and returns whether any was removed. Removing the last item
  deletes the key. On an expired key it deletes the key and returns `false`.
- `ttl` and `encrypted` apply to the whole list. A push without `ttl` keeps the
  list's existing expiry.
- List reads are not cached.

## Watching for changes

```dart
final subscription = store.watch('auth.*').listen((event) {
  // KeyCreated, KeyUpdated, KeyDeleted, KeyExpired, StorageCleared
});
```

Patterns: `'*'` for everything, `'auth.*'` for `auth` and every key starting
with `auth.`, `'user*'` for every key starting with `user`, or an exact key.
`StorageCleared` reaches every subscriber. See
[Architecture](architecture.md#events) for which call sends which event.

## Maintenance

- `compact()`: purges expired keys and rewrites the log with only live data. The
  store also does this by itself once most of the file is old history, so you
  rarely need it.
- `purgeExpired()`: deletes every expired key now.

## Errors

Failures throw a `StoreError` subclass, for example
`StoreError_EncryptionKeyNotSet` or `StoreError_Io`. The full list is in
[Architecture](architecture.md#errors).

## The raw API

`package:rodinia_flutter/rodinia_flutter.dart` exports the generated functions
(`storeSet`, `storeGetEntry`, …) as they are. They skip the Dart cache and the
value encoding. Writes made through them are still noticed by every
`RodiniaStore` handle through the change counter, but prefer `RodiniaStore`.
