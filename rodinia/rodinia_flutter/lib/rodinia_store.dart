import 'dart:convert';
import 'dart:ffi';
import 'dart:typed_data';

import 'src/rust/api/store.dart' as native;
import 'src/rust/cache.dart';
import 'src/rust/error.dart';
import 'src/rust/events.dart';
import 'src/rust/frb_generated.dart';
import 'src/value_cache.dart';

export 'src/rust/error.dart';
export 'src/rust/events.dart' show StorageEvent;

/// Fast, secure, reactive key-value storage backed by a Rust core.
///
/// Small local operations ([set], [get], [contains], [delete]) are
/// synchronous: they never introduce `Future`/`async` overhead for the
/// common case of reading or writing a single value. Only [open] itself is
/// asynchronous, since it has to first initialize the Rust bridge.
///
/// Every mutating call accepts `durable`. When `true`, the call returns only
/// after the write is on stable storage, so it survives a power loss. The
/// default (`false`) hands the write to the operating system and returns
/// immediately, which is much faster.
///
/// Reads are served from an in-memory cache of recently used values, capped
/// at `cacheBytes`. The cache stays correct when other [RodiniaStore]
/// instances or other isolates write to the same store: Rust bumps a shared
/// change counter on every write, and the cache is dropped whenever it moved
/// for a reason this instance didn't cause.
class RodiniaStore {
  RodiniaStore._(this._cache, this._generation) : _seen = _generation.value;

  final ValueCache _cache;
  final Pointer<Uint64> _generation;
  int _seen;

  /// Opens (or creates) the store rooted at [path].
  ///
  /// [path] should point at a directory inside the app's private sandbox,
  /// typically `(await getApplicationDocumentsDirectory()).path`. The store
  /// itself is opened once per process; every call returns a new handle to
  /// it with its own read cache of up to [cacheBytes] (0 disables caching).
  ///
  /// With [preload] (the default), the handle fills its cache in the
  /// background right after opening, reading stored values in one pass up to
  /// [cacheBytes], so the first reads after an app starts don't each go to
  /// disk. This spends up to [cacheBytes] of memory straight away. Encrypted
  /// values are not preloaded; they enter the cache as they are read.
  /// [preloaded] completes when the fill is done.
  static Future<RodiniaStore> open(
    String path, {
    int cacheBytes = 8 * 1024 * 1024,
    bool preload = true,
  }) async {
    if (!RustLib.instance.initialized) {
      await RustLib.init();
    }
    if (!native.storeIsOpen()) {
      native.storeOpen(path: path);
    }
    final store = RodiniaStore._(
      ValueCache(cacheBytes),
      Pointer<Uint64>.fromAddress(native.storeGenerationAddress()),
    );
    if (preload && cacheBytes > 0) {
      _preloads[store] = store._preload(cacheBytes);
    }
    return store;
  }

  /// Kept outside the instance, so a handle holds no `Future` and can still
  /// be sent to another isolate.
  static final _preloads = Expando<Future<void>>();

  /// Completes when the background preload started by [open] has finished,
  /// or straight away if there is none. Waiting for it is never required:
  /// reads before then are correct, just not yet served from the cache.
  Future<void> get preloaded => _preloads[this] ?? Future.value();

  /// Fills the cache from Rust. The values are only used if the change
  /// counter still matches the one Rust read them under, so anything written
  /// in the meantime wins over the preloaded data.
  Future<void> _preload(int limitBytes) async {
    final Preload preload;
    try {
      preload = await native.storePreload(limitBytes: limitBytes);
    } on StoreError {
      return;
    }
    _syncCache();
    if (_seen != preload.generation) return;
    if (preload.complete) _cache.markComplete();
    for (final item in preload.items) {
      if (_cache.lookup(item.key, _nowMs) != null) continue;
      _cache.put(
        item.key,
        CachedValue(
          item.value,
          encrypted: item.encrypted,
          expiresAtMs: item.expiresAtMs,
        ),
      );
    }
  }

  static int _nowMs() => DateTime.now().millisecondsSinceEpoch;

  void _syncCache() {
    final generation = _generation.value;
    if (generation != _seen) {
      _cache.clear();
      _seen = generation;
    }
  }

  /// Runs a native write and keeps the cache coherent with it. [changes]
  /// says how many times the call itself moved the change counter; if the
  /// counter moved by anything else, another writer was involved and the
  /// cache is dropped instead of patched.
  R _mutate<R>(
    R Function() call,
    void Function(R result) apply, {
    int Function(R result)? changes,
  }) {
    final before = _generation.value;
    final R result;
    try {
      result = call();
    } catch (_) {
      _cache.clear();
      _seen = _generation.value;
      rethrow;
    }
    final after = _generation.value;
    final expected = changes == null ? 1 : changes(result);
    if (before == _seen && after - before == expected) {
      apply(result);
    } else {
      _cache.clear();
    }
    _seen = after;
    return result;
  }

  /// Stores [value] under [key].
  ///
  /// [value] may be a [String], [bool], [num], a `Uint8List`/`List<int>`, or
  /// any object supported by [jsonEncode] (e.g. a `Map`/`List` of the
  /// above) — it is encoded to bytes before crossing into Rust.
  void set<T>(
    String key,
    T value, {
    Duration? ttl,
    bool encrypted = false,
    bool durable = false,
  }) {
    final bytes = _encodeOwned(value);
    final expiresAtMs = ttl == null ? null : _nowMs() + ttl.inMilliseconds;
    _mutate(
      () => native.storeSet(
        key: key,
        value: bytes,
        ttlMs: ttl?.inMilliseconds,
        encrypted: encrypted,
        durable: durable,
      ),
      (_) => _cache.put(
        key,
        _written(value, bytes, encrypted: encrypted, expiresAtMs: expiresAtMs),
      ),
    );
  }

  /// The cache entry for a value this instance just wrote. It keeps the
  /// original String or scalar, so the next [get] returns it without decoding
  /// the bytes again.
  CachedValue _written<T>(
    T value,
    Uint8List bytes, {
    required bool encrypted,
    required int? expiresAtMs,
  }) {
    final cached = CachedValue(
      bytes,
      encrypted: encrypted,
      expiresAtMs: expiresAtMs,
    );
    if (value is String) {
      cached.text = value;
    } else if (value == null || value is num || value is bool) {
      cached.scalar = value;
      cached.hasScalar = true;
    }
    return cached;
  }

  /// Reads the value stored under [key], or `null` if it is absent or has
  /// expired. `T` must match the type originally passed to [set].
  T? get<T>(String key) {
    _syncCache();
    final cached = _cache.lookup(key, _nowMs) ?? _load(key);
    return cached == null ? null : _fromCached<T>(cached);
  }

  CachedValue? _load(String key) {
    final before = _generation.value;
    final entry = native.storeGetEntry(key: key);
    if (entry == null) return null;
    final value = CachedValue(
      entry.value,
      encrypted: entry.encrypted,
      expiresAtMs: entry.expiresAtMs,
    );
    if (_generation.value == before) _cache.put(key, value);
    return value;
  }

  /// Returns all live key-value pairs in the store, decrypting encrypted
  /// values. Throws `StoreError.encryptionKeyNotSet` if any value is
  /// encrypted and no key is set.
  Map<String, T> getAll<T>() {
    _syncCache();
    if (_cache.isComplete) {
      final now = _nowMs();
      final entries = _cache.entries.toList();
      if (!entries.any((e) => e.value.isExpiredAt(now))) {
        return {for (final e in entries) e.key: _fromCached<T>(e.value)};
      }
    }

    final before = _generation.value;
    final items = native.storeGetAll();
    final fill = _generation.value == before;
    if (fill) {
      _cache.clear();
      _cache.markComplete();
    }
    final result = <String, T>{};
    for (final item in items) {
      final value = CachedValue(
        item.value,
        encrypted: item.encrypted,
        expiresAtMs: item.expiresAtMs,
      );
      if (fill) _cache.put(item.key, value);
      result[item.key] = _fromCached<T>(value);
    }
    return result;
  }

  /// Whether [key] currently holds a live (non-expired) value.
  bool contains(String key) {
    _syncCache();
    return _cache.lookup(key, _nowMs) != null || native.storeContains(key: key);
  }

  /// Removes [key]. Returns whether a value was actually removed.
  bool delete(String key, {bool durable = false}) => _mutate(
    () => native.storeDelete(key: key, durable: durable),
    (_) => _cache.remove(key),
    changes: (removed) => removed ? 1 : 0,
  );

  /// Removes every key and value from the store. The encryption key, if
  /// set, stays set.
  void clear({bool durable = false}) =>
      _mutate(() => native.storeClear(durable: durable), (_) {
        _cache.clear();
        _cache.markComplete();
      });

  /// All currently-live keys.
  List<String> get keys => native.storeKeys();

  /// The number of currently-live keys.
  int get length => native.storeLen();

  /// Sets the key used to encrypt and decrypt values written with
  /// `encrypted: true`.
  void setEncryptionKey(Uint8List key) => _mutate(
    () => native.storeSetEncryptionKey(key: key),
    (_) => _cache.dropEncrypted(),
  );

  /// Forgets the encryption key and keeps all data. Reading an encrypted
  /// value then throws `StoreError.encryptionKeyNotSet` until
  /// [setEncryptionKey] is called again. Plain values stay readable.
  void lock() =>
      _mutate(() => native.storeLock(), (_) => _cache.dropEncrypted());

  /// Generates a random 256-bit key suitable for [setEncryptionKey].
  static Uint8List generateEncryptionKey() =>
      native.storeGenerateEncryptionKey();

  /// Atomically adds [delta] to the integer counter at [key] (treating a
  /// missing or expired key as 0) and returns the new value.
  int increment(String key, {int delta = 1, bool durable = false}) => _mutate(
    () => native.storeIncrement(key: key, delta: delta, durable: durable),
    (_) => _cache.remove(key),
  );

  /// Like [set], but only writes if [key] doesn't already hold a live
  /// value. Returns whether the write happened.
  bool setIfAbsent<T>(
    String key,
    T value, {
    Duration? ttl,
    bool encrypted = false,
    bool durable = false,
  }) {
    final bytes = _encodeOwned(value);
    final expiresAtMs = ttl == null ? null : _nowMs() + ttl.inMilliseconds;
    return _mutate(
      () => native.storeSetIfAbsent(
        key: key,
        value: bytes,
        ttlMs: ttl?.inMilliseconds,
        encrypted: encrypted,
        durable: durable,
      ),
      (written) {
        if (written) {
          _cache.put(
            key,
            _written(
              value,
              bytes,
              encrypted: encrypted,
              expiresAtMs: expiresAtMs,
            ),
          );
        }
      },
      changes: (written) => written ? 1 : 0,
    );
  }

  /// Evicts all expired keys immediately (rather than lazily, on next
  /// access) and returns how many were removed.
  int purgeExpired() => _mutate(
    () => native.storePurgeExpired(),
    (_) {},
    changes: (purged) => purged,
  );

  /// Rewrites the on-disk log to contain only current, live entries,
  /// reclaiming space used by deleted/overwritten/expired history. The store
  /// also does this on its own once most of the log is dead history.
  void compact() => native.storeCompact();

  /// A stream of storage events matching [pattern]: `'*'` (the default) for
  /// everything, `'auth.*'` for a key namespace, or an exact key.
  Stream<StorageEvent> watch([String pattern = '*']) =>
      native.storeWatch(pattern: pattern);

  void listPush<T>(
    String key,
    T item, {
    Duration? ttl,
    bool encrypted = false,
    bool durable = false,
  }) {
    _mutate(
      () => native.storeListPush(
        key: key,
        item: _encode(item),
        ttlMs: ttl?.inMilliseconds,
        encrypted: encrypted,
        durable: durable,
      ),
      (_) => _cache.remove(key),
    );
  }

  List<T>? listGet<T>(String key) {
    final items = native.storeListGet(key: key);
    if (items == null) return null;
    return [for (final item in items) _decode<T>(item)];
  }

  bool deleteFromList<T>(
    String key,
    T item, {
    Duration? ttl,
    bool encrypted = false,
    bool durable = false,
  }) {
    return _mutate(
      () => native.storeDeleteFromList(
        key: key,
        item: _encode(item),
        ttlMs: ttl?.inMilliseconds,
        encrypted: encrypted,
        durable: durable,
      ),
      (_) => _cache.remove(key),
      changes: (removed) => removed ? 1 : 0,
    );
  }

  Uint8List _encode<T>(T value) {
    if (value is Uint8List) return value;
    if (value is List<int>) return Uint8List.fromList(value);
    if (value is String) return utf8.encode(value);
    return utf8.encode(jsonEncode(value));
  }

  Uint8List _encodeOwned<T>(T value) =>
      value is Uint8List ? Uint8List.fromList(value) : _encode(value);

  T _fromCached<T>(CachedValue value) {
    if (T == Uint8List || T == List<int>) {
      return Uint8List.fromList(value.bytes) as T;
    }
    if (T != String && value.hasScalar) return value.scalar as T;
    final text = value.text ??= utf8.decode(value.bytes);
    if (T == String) return text as T;
    final decoded = jsonDecode(text);
    if (decoded == null ||
        decoded is String ||
        decoded is num ||
        decoded is bool) {
      value.scalar = decoded;
      value.hasScalar = true;
    }
    return decoded as T;
  }

  T _decode<T>(Uint8List bytes) {
    if (T == Uint8List || T == List<int>) return bytes as T;
    final text = utf8.decode(bytes);
    if (T == String) return text as T;
    return jsonDecode(text) as T;
  }
}
