import 'dart:collection';
import 'dart:typed_data';

/// A value as stored in Rust, plus lazily memoized decodings of it.
class CachedValue {
  CachedValue(this.bytes, {required this.encrypted, this.expiresAtMs});

  final Uint8List bytes;
  final bool encrypted;
  final int? expiresAtMs;

  /// The UTF-8 decoding of [bytes], filled on first use.
  String? text;

  /// The JSON decoding of [text] when it is a `String`, `num`, `bool` or
  /// `null` (immutable, so safe to share). Maps and lists are never memoized.
  Object? scalar;
  bool hasScalar = false;

  bool isExpiredAt(int nowMs) {
    final expiresAt = expiresAtMs;
    return expiresAt != null && expiresAt <= nowMs;
  }
}

/// A least-recently-used cache of values, bounded by total bytes.
///
/// Every operation is O(1) amortized: a [LinkedHashMap] keeps insertion
/// order, and a hit is moved to the end by removing and re-inserting it, so
/// the first key is always the least recently used.
class ValueCache {
  ValueCache(this.capacityBytes);

  final int capacityBytes;
  final LinkedHashMap<String, CachedValue> _entries = LinkedHashMap();
  int _sizeBytes = 0;
  bool _complete = false;

  static int _cost(String key, CachedValue value) =>
      value.bytes.length + key.length * 2 + 64;

  int get sizeBytes => _sizeBytes;

  /// Whether the cache currently holds every live key in the store.
  bool get isComplete => _complete;

  void markComplete() => _complete = true;

  Iterable<MapEntry<String, CachedValue>> get entries => _entries.entries;

  /// Returns the live value for [key] and marks it most recently used.
  /// [nowMs] is only called when the value has an expiry.
  CachedValue? lookup(String key, int Function() nowMs) {
    final value = _entries.remove(key);
    if (value == null) return null;
    if (value.expiresAtMs != null && value.isExpiredAt(nowMs())) {
      _sizeBytes -= _cost(key, value);
      return null;
    }
    _entries[key] = value;
    return value;
  }

  void put(String key, CachedValue value) {
    remove(key);
    final cost = _cost(key, value);
    if (cost > capacityBytes) {
      _complete = false;
      return;
    }
    _entries[key] = value;
    _sizeBytes += cost;
    while (_sizeBytes > capacityBytes) {
      final oldest = _entries.keys.first;
      _sizeBytes -= _cost(oldest, _entries.remove(oldest)!);
      _complete = false;
    }
  }

  void remove(String key) {
    final old = _entries.remove(key);
    if (old != null) _sizeBytes -= _cost(key, old);
  }

  void clear() {
    _entries.clear();
    _sizeBytes = 0;
    _complete = false;
  }

  void dropEncrypted() {
    final before = _entries.length;
    _entries.removeWhere((key, value) {
      if (!value.encrypted) return false;
      _sizeBytes -= _cost(key, value);
      return true;
    });
    if (_entries.length != before) _complete = false;
  }
}
