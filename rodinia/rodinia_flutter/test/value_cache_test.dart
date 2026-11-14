import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:rodinia_flutter/src/value_cache.dart';

CachedValue value(int length, {bool encrypted = false, int? expiresAtMs}) =>
    CachedValue(
      Uint8List(length),
      encrypted: encrypted,
      expiresAtMs: expiresAtMs,
    );

int cost(String key, int length) => length + key.length * 2 + 64;

void main() {
  test('evicts least recently used when over capacity', () {
    final cache = ValueCache(cost('a', 100) * 2);
    cache.put('a', value(100));
    cache.put('b', value(100));
    cache.put('c', value(100));
    expect(cache.lookup('a', () => 0), isNull);
    expect(cache.lookup('b', () => 0), isNotNull);
    expect(cache.lookup('c', () => 0), isNotNull);
  });

  test('lookup refreshes recency', () {
    final cache = ValueCache(cost('a', 100) * 2);
    cache.put('a', value(100));
    cache.put('b', value(100));
    cache.lookup('a', () => 0);
    cache.put('c', value(100));
    expect(cache.lookup('a', () => 0), isNotNull);
    expect(cache.lookup('b', () => 0), isNull);
  });

  test(
    'value larger than capacity is not cached and replaces nothing stale',
    () {
      final cache = ValueCache(1024);
      cache.put('k', value(10));
      cache.put('k', value(10 * 1024));
      expect(cache.lookup('k', () => 0), isNull);
      expect(cache.sizeBytes, 0);
    },
  );

  test('expired entry is removed and returns null', () {
    final cache = ValueCache(1 << 20);
    cache.put('k', value(10, expiresAtMs: 1000));
    expect(cache.lookup('k', () => 999), isNotNull);
    expect(cache.lookup('k', () => 1000), isNull);
    expect(cache.sizeBytes, 0);
  });

  test('dropEncrypted keeps plain entries only', () {
    final cache = ValueCache(1 << 20);
    cache.put('secret', value(10, encrypted: true));
    cache.put('plain', value(10));
    cache.markComplete();
    cache.dropEncrypted();
    expect(cache.lookup('secret', () => 0), isNull);
    expect(cache.lookup('plain', () => 0), isNotNull);
    expect(cache.sizeBytes, cost('plain', 10));
    expect(cache.isComplete, isFalse);
  });

  test('eviction clears isComplete; markComplete sets it', () {
    final cache = ValueCache(cost('a', 100) * 2);
    expect(cache.isComplete, isFalse);
    cache.markComplete();
    cache.put('a', value(100));
    cache.put('b', value(100));
    expect(cache.isComplete, isTrue);
    cache.put('c', value(100));
    expect(cache.isComplete, isFalse);
  });

  test('size-skipped put and clear reset isComplete; remove keeps it', () {
    final cache = ValueCache(1024);
    cache.markComplete();
    cache.put('a', value(10));
    cache.remove('a');
    cache.remove('missing');
    expect(cache.isComplete, isTrue);
    cache.put('huge', value(4096));
    expect(cache.isComplete, isFalse);
    cache.markComplete();
    cache.clear();
    expect(cache.isComplete, isFalse);
  });

  test('capacity 0 never caches', () {
    final cache = ValueCache(0);
    cache.put('k', value(0));
    expect(cache.lookup('k', () => 0), isNull);
    expect(cache.sizeBytes, 0);
  });

  test('sizeBytes tracks puts, overwrites and removes', () {
    final cache = ValueCache(1 << 20);
    cache.put('a', value(10));
    cache.put('b', value(20));
    expect(cache.sizeBytes, cost('a', 10) + cost('b', 20));
    cache.put('a', value(30));
    expect(cache.sizeBytes, cost('a', 30) + cost('b', 20));
    cache.remove('b');
    expect(cache.sizeBytes, cost('a', 30));
    cache.clear();
    expect(cache.sizeBytes, 0);
  });

  test('entries lists cached values', () {
    final cache = ValueCache(1 << 20);
    cache.put('a', value(1));
    cache.put('b', value(2));
    expect(
      {for (final e in cache.entries) e.key: e.value.bytes.length},
      {'a': 1, 'b': 2},
    );
  });
}
