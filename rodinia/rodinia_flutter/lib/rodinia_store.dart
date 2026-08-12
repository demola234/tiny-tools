import 'dart:convert';
import 'dart:typed_data';

import 'src/rust/api/store.dart' as native;
import 'src/rust/events.dart';
import 'src/rust/frb_generated.dart';

export 'src/rust/error.dart';
export 'src/rust/events.dart' show StorageEvent;

/// Fast, secure, reactive key-value storage backed by a Rust core.
///
/// Small local operations ([set], [get], [contains], [delete]) are
/// synchronous: they never introduce `Future`/`async` overhead for the
/// common case of reading or writing a single value. Only [open] itself is
/// asynchronous, since it has to first initialize the Rust bridge.
class RodiniaStore {
  RodiniaStore._();

  /// Opens (or creates) the store rooted at [path].
  ///
  /// [path] should point at a directory inside the app's private sandbox,
  /// typically `(await getApplicationDocumentsDirectory()).path`.
  static Future<RodiniaStore> open(String path) async {
    if (!RustLib.instance.initialized) {
      await RustLib.init();
    }
    if (!native.storeIsOpen()) {
      native.storeOpen(path: path);
    }
    return RodiniaStore._();
  }

  /// Stores [value] under [key].
  ///
  /// [value] may be a [String], [bool], [num], a `Uint8List`/`List<int>`, or
  /// any object supported by [jsonEncode] (e.g. a `Map`/`List` of the
  /// above) — it is encoded to bytes before crossing into Rust.
  void set<T>(String key, T value, {Duration? ttl, bool encrypted = false}) {
    native.storeSet(
      key: key,
      value: _encode(value),
      ttlMs: ttl?.inMilliseconds,
      encrypted: encrypted,
    );
  }

  /// Reads the value stored under [key], or `null` if it is absent or has
  /// expired. `T` must match the type originally passed to [set].
  T? get<T>(String key) {
    final bytes = native.storeGet(key: key);
    if (bytes == null) return null;
    return _decode<T>(Uint8List.fromList(bytes));
  }

  /// Returns all key-value pairs in the store. Returns an empty map if the
  /// store is empty.
  Map<String, T> getAll<T>() {
    final bytes = native.storeGetAll();
    if (bytes.isEmpty) return {};
    return bytes.map((key, value) {
      return MapEntry(key, _decode<T>(value.value));
    });
  }

  /// Whether [key] currently holds a live (non-expired) value.
  bool contains(String key) => native.storeContains(key: key);

  /// Removes [key]. Returns whether a value was actually removed.
  bool delete(String key) => native.storeDelete(key: key);

  /// Removes every key from the store.
  void clear() => native.storeClear();

  /// All currently-live keys.
  List<String> get keys => native.storeKeys();

  /// The number of currently-live keys.
  int get length => native.storeLen();

  void setEncryptionKey(Uint8List key) =>
      native.storeSetEncryptionKey(key: key);

  /// Generates a random 256-bit key suitable for [setEncryptionKey].
  static Uint8List generateEncryptionKey() =>
      Uint8List.fromList(native.storeGenerateEncryptionKey());

  /// Atomically adds [delta] to the integer counter at [key] (treating a
  /// missing or expired key as 0) and returns the new value.
  int increment(String key, {int delta = 1}) =>
      native.storeIncrement(key: key, delta: delta);

  /// Like [set], but only writes if [key] doesn't already hold a live
  /// value. Returns whether the write happened.
  bool setIfAbsent<T>(
    String key,
    T value, {
    Duration? ttl,
    bool encrypted = false,
  }) {
    return native.storeSetIfAbsent(
      key: key,
      value: _encode(value),
      ttlMs: ttl?.inMilliseconds,
      encrypted: encrypted,
    );
  }

  /// Evicts all expired keys immediately (rather than lazily, on next
  /// access) and returns how many were removed.
  int purgeExpired() => native.storePurgeExpired();

  /// Rewrites the on-disk log to contain only current, live entries,
  /// reclaiming space used by deleted/overwritten/expired history.
  void compact() => native.storeCompact();

  /// A stream of storage events matching [pattern]: `'*'` (the default) for
  /// everything, `'auth.*'` for a key namespace, or an exact key.
  Stream<StorageEvent> watch([String pattern = '*']) =>
      native.storeWatch(pattern: pattern);

  Uint8List _encode<T>(T value) {
    if (value is Uint8List) return value;
    if (value is List<int>) return Uint8List.fromList(value);
    if (value is String) return Uint8List.fromList(utf8.encode(value));
    return Uint8List.fromList(utf8.encode(jsonEncode(value)));
  }

  T _decode<T>(Uint8List bytes) {
    if (T == Uint8List || T == List<int>) return bytes as T;
    final text = utf8.decode(bytes);
    if (T == String) return text as T;
    return jsonDecode(text) as T;
  }

  void listPush<T>(String key, T item, {Duration? ttl, bool encrypted = false}) {
    native.storeListPush(
      key: key,
      item: _encode(item),
      ttlMs: ttl?.inMilliseconds,
      encrypted: encrypted,
    );
  }

  List<T>? listGet<T>(String key) {
    final bytes = native.storeListGet(key: key);
    if (bytes == null) return null;
    return bytes.map((item) => _decode<T>(item)).toList();
  }

  bool deleteFromList<T>(String key, T item, {Duration? ttl, bool encrypted = false}) {
    return native.storeDeleteFromList(
      key: key,
      item: _encode(item),
      ttlMs: ttl?.inMilliseconds,
      encrypted: encrypted,
    );
  }
}
