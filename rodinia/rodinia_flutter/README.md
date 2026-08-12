# rodinia_flutter

Flutter bindings for [Rodinia](../README.md), generated over
[rodinia_ffi](../rodinia_ffi) via
[flutter_rust_bridge](https://cbeuw.github.io/flutter_rust_bridge/).

## Installation

Not on pub.dev yet — pull it straight from GitHub in your app's
`pubspec.yaml`, pointing `path` at this subfolder:

```yaml
dependencies:
  rodinia_flutter:
    git:
      url: https://github.com/demola234/tiny-tools.git
      path: rodinia/rodinia_flutter
```

Then run `flutter pub get`. To pin to a specific commit/branch/tag, add a
`ref:`:

```yaml
  rodinia_flutter:
    git:
      url: https://github.com/demola234/tiny-tools.git
      path: rodinia/rodinia_flutter
      ref: main 
```

This is a Rust-backed FFI plugin (via `flutter_rust_bridge`/cargokit), so it
compiles native code per-platform — no manual setup beyond the pubspec
entry, but the first build will take longer while Cargo compiles the Rust
core.

## Usage

```dart
import 'package:path_provider/path_provider.dart';
import 'package:rodinia_flutter/rodinia_flutter.dart';

final dir = await getApplicationDocumentsDirectory();
final store = await RodiniaStore.open(dir.path);

store.set('user.name', 'Ada');
store.set('session.token', 'abc123', ttl: const Duration(minutes: 30), encrypted: true);

final name = store.get<String>('user.name');
final count = store.increment('login.count');

final sub = store.watch('user.*').listen((event) {
  print('Storage event: $event');
});

store.listPush('tasks', 'buy milk');
store.listPush('tasks', 'walk dog');
final tasks = store.listGet<String>('tasks'); 
// ['buy milk', 'walk dog']
store.deleteFromList('tasks', 'buy milk');
// ['walk dog']
```

`open` is the only async call — everything else is synchronous.

## API reference

### `RodiniaStore`

| Function | Signature | Description |
|---|---|---|
| `open` | `static Future<RodiniaStore> open(String path)` | Opens/creates the store at `path`. |
| `set` | `void set<T>(String key, T value, {Duration? ttl, bool encrypted = false})` | Writes a value (String, bool, num, bytes, or anything `jsonEncode`-able), optional TTL and encryption. |
| `get` | `T? get<T>(String key)` | Reads a value; `null` if missing/expired. |
| `getAll` | `Map<String, T> getAll<T>()` | Returns every live key/value pair. |
| `contains` | `bool contains(String key)` | Whether a live value exists for `key`. |
| `delete` | `bool delete(String key)` | Removes a key; returns whether something was removed. |
| `clear` | `void clear()` | Removes every key. |
| `keys` | `List<String> get keys` | All currently-live keys. |
| `length` | `int get length` | Count of currently-live keys. |
| `setEncryptionKey` | `void setEncryptionKey(Uint8List key)` | Sets the 256-bit key used for `encrypted: true` writes. |
| `generateEncryptionKey` | `static Uint8List generateEncryptionKey()` | Generates a random 256-bit key. |
| `increment` | `int increment(String key, {int delta = 1})` | Atomically adds `delta` to an integer counter (missing/expired treated as 0), returns the new value. |
| `setIfAbsent` | `bool setIfAbsent<T>(String key, T value, {Duration? ttl, bool encrypted = false})` | Writes only if `key` has no live value; returns whether it wrote. |
| `purgeExpired` | `int purgeExpired()` | Immediately evicts all expired keys; returns how many were removed. |
| `compact` | `void compact()` | Rewrites the on-disk log to drop stale/deleted history. |
| `watch` | `Stream<StorageEvent> watch([String pattern = '*'])` | Reactive stream of storage events (`'*'`, `'auth.*'`, or an exact key). |
| `listPush` | `int listPush<T>(String key, T item, {Duration? ttl, bool encrypted = false})` | Appends `item` to the list at `key` (missing/expired treated as empty), returns the new length. Omitting `ttl` preserves the list's existing expiry. |
| `listGet` | `List<T>? listGet<T>(String key)` | Reads the full list at `key`; `null` if missing or expired. |
| `deleteFromList` | `bool deleteFromList<T>(String key, T item, {Duration? ttl, bool encrypted = false})` | Removes every occurrence of `item` from the list; returns whether anything was removed. Deletes the key entirely if this empties the list. |

### `StorageEvent` variants (from `watch`)

`keyCreated(key)`, `keyUpdated(key)`, `keyDeleted(key)`, `keyExpired(key)`,
`storageCleared()`, `cacheInvalidated(key)`

### `StoreError` variants (thrown on failure)

`notInitialized`, `alreadyInitialized`, `io(msg)`, `serialization(msg)`,
`encryptionKeyNotSet`, `invalidKeyLength(len)`, `encryption`,
`invalidCiphertext`, `unsupportedEncryptionVersion(v)`,
`authenticationFailed`, `invalidCounterValue(msg)`

## Project structure

* `../rodinia_ffi`: the actual Rust source — the storage engine plus the
  `#[flutter_rust_bridge::frb]`-annotated API surface under `src/api/`. This
  plugin has no Rust source of its own; `flutter_rust_bridge.yaml` points
  `rust_root` at it, and `cargokit` (the `android/`, `ios/`, `macos/`,
  `linux/`, `windows/` build configs) compiles it directly from there.
* `lib/`: hand-written Dart API (`rodinia_flutter.dart`) plus
  `lib/src/rust/`, the generated bindings — regenerated by
  `flutter_rust_bridge_codegen generate`, never edited by hand.

## Regenerating bindings

After changing anything under `../rodinia_ffi/src/api/`:

```sh
flutter_rust_bridge_codegen generate
```

(or `just gen` from the repo root).

## Example

See [`../examples/rodinia_flutter_example`](../examples/rodinia_flutter_example).
