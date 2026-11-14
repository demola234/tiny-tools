import 'dart:io';
import 'dart:isolate';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:path_provider/path_provider.dart';
import 'package:rodinia_flutter/rodinia_flutter.dart';
import 'package:rodinia_flutter/rodinia_store.dart';

void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  late Directory testDirectory;
  late RodiniaStore store;

  setUpAll(() async {
    await RustLib.init();

    final baseDirectory = await getApplicationDocumentsDirectory();

    testDirectory = Directory('${baseDirectory.path}/rodinia_integration_test');

    if (testDirectory.existsSync()) {
      await testDirectory.delete(recursive: true);
    }

    await testDirectory.create(recursive: true);

    store = await RodiniaStore.open(testDirectory.path);
  });

  tearDownAll(() async {
    store.clear();

    if (testDirectory.existsSync()) {
      await testDirectory.delete(recursive: true);
    }
  });

  setUp(() {
    store.clear();
  });

  group('RodiniaStore integration', () {
    test('can set and get a string value', () {
      store.set('username', 'Tom');

      expect(store.get<String>('username'), 'Tom');
    });

    test('can set and get an integer value', () {
      store.set('age', 28);

      expect(store.get<int>('age'), 28);
    });

    test('can set and get a boolean value', () {
      store.set('is_authenticated', true);


    });

    test('returns null when key does not exist', () {
      expect(store.get<String>('does_not_exist'), isNull);
    });

    test('can update an existing value', () {
      store.set('theme', 'light');

      expect(store.get<String>('theme'), 'light');

      store.set('theme', 'dark');

      expect(store.get<String>('theme'), 'dark');
    });

    test('can check whether a key exists', () {
      expect(store.contains('username'), isFalse);

      store.set('username', 'Tom');

      expect(store.contains('username'), isTrue);
    });

    test('can delete a value', () {
      store.set('username', 'Tom');

      expect(store.get<String>('username'), 'Tom');

      store.delete('username');

      expect(store.get<String>('username'), isNull);

      expect(store.contains('username'), isFalse);
    });

    test('can increment a value', () {
      expect(store.get<int>('visits'), isNull);

      store.increment('visits');

      expect(store.get<int>('visits'), 1);

      store.increment('visits');

      expect(store.get<int>('visits'), 2);

      store.increment('visits');

      expect(store.get<int>('visits'), 3);
    });

    test('can increment an existing value', () {
      store.set('visits', 10);

      store.increment('visits');

      expect(store.get<int>('visits'), 11);
    });

    test('can list stored keys', () {
      store.set('first', 'one');
      store.set('second', 'two');
      store.set('third', 'three');

      expect(store.keys, containsAll(['first', 'second', 'third']));

      expect(store.length, 3);
    });

    test('can clear the store', () {
      store.set('first', 'one');
      store.set('second', 'two');

      expect(store.length, 2);

      store.clear();

      expect(store.length, 0);

      expect(store.keys, isEmpty);
    });

    test('supports expiring values with TTL', () async {
      store.set('flash_sale', true, ttl: const Duration(milliseconds: 300));

      expect(store.get<bool>('flash_sale'), isTrue);

      await Future<void>.delayed(const Duration(milliseconds: 500));

      expect(store.get<bool>('flash_sale'), isNull);
    });

    test('can store an encrypted value', () {
      final key = RodiniaStore.generateEncryptionKey();

      store.setEncryptionKey(key);

      store.set('refresh_token', 'super-secret-token', encrypted: true);

      expect(store.get<String>('refresh_token'), 'super-secret-token');
    });

    test('encrypted value cannot be read without an encryption key', () {
      final key = RodiniaStore.generateEncryptionKey();

      store.setEncryptionKey(key);

      store.set('secret', 'very-secret-value', encrypted: true);

      store.lock();

      expect(
        () => store.get<String>('secret'),
        throwsA(isA<StoreError_EncryptionKeyNotSet>()),
      );
    });

    test('encrypted value can be read after restoring the key', () {
      final key = RodiniaStore.generateEncryptionKey();

      store.setEncryptionKey(key);

      store.set('secret', 'very-secret-value', encrypted: true);

      store.lock();

      expect(
        () => store.get<String>('secret'),
        throwsA(isA<StoreError_EncryptionKeyNotSet>()),
      );

      store.setEncryptionKey(key);

      expect(store.get<String>('secret'), 'very-secret-value');
    });

    test('watch emits storage events', () async {
      final events = <StorageEvent>[];

      final subscription = store.watch().listen(events.add);

      store.set('username', 'Tom');

      await Future<void>.delayed(const Duration(milliseconds: 100));

      await subscription.cancel();

      expect(events, isNotEmpty);
    });

    test('watch emits events when values change', () async {
      final events = <StorageEvent>[];

      final subscription = store.watch().listen(events.add);

      store.set('counter', 1);

      store.set('counter', 2);

      store.delete('counter');

      await Future<void>.delayed(const Duration(milliseconds: 100));

      await subscription.cancel();

      expect(events.length, greaterThanOrEqualTo(3));
    });

    test('data survives reopening the store', () async {
      store.set('username', 'Tom');

      store.set('visits', 42);

      store = await RodiniaStore.open(testDirectory.path);

      expect(store.get<String>('username'), 'Tom');

      expect(store.get<int>('visits'), 42);
    });

    test(
      'encrypted data survives reopening when encryption key is restored',
      () async {
        final key = RodiniaStore.generateEncryptionKey();

        store.setEncryptionKey(key);

        store.set('refresh_token', 'super-secret-token', encrypted: true);

        store.lock();

        store = await RodiniaStore.open(testDirectory.path);

        expect(
          () => store.get<String>('refresh_token'),
          throwsA(isA<StoreError_EncryptionKeyNotSet>()),
        );

        store.setEncryptionKey(key);

        expect(store.get<String>('refresh_token'), 'super-secret-token');
      },
    );

    test('getAll returns decrypted values', () {
      store.setEncryptionKey(RodiniaStore.generateEncryptionKey());
      store.set('secret', 'value', encrypted: true);
      store.set('plain', 'p');
      expect(store.getAll<String>(), {'secret': 'value', 'plain': 'p'});
    });

    test('getAll after lock throws EncryptionKeyNotSet', () {
      store.setEncryptionKey(RodiniaStore.generateEncryptionKey());
      store.set('secret', 'value', encrypted: true);
      store.lock();
      expect(
        () => store.getAll<String>(),
        throwsA(isA<StoreError_EncryptionKeyNotSet>()),
      );
    });

    test('lock keeps plain values readable', () {
      store.setEncryptionKey(RodiniaStore.generateEncryptionKey());
      store.set('plain', 'p');
      store.lock();
      expect(store.get<String>('plain'), 'p');
    });

    test('durable write is readable', () {
      store.set('k', 'v', durable: true);
      expect(store.get<String>('k'), 'v');
      expect(store.delete('k', durable: true), isTrue);
      expect(store.increment('n', durable: true), 1);
    });

    test('clear keeps the encryption key', () {
      store.setEncryptionKey(RodiniaStore.generateEncryptionKey());
      store.set('secret', 'value', encrypted: true);
      store.clear();
      expect(store.get<String>('secret'), isNull);
      store.set('secret', 'again', encrypted: true);
      expect(store.get<String>('secret'), 'again');
    });

    test('write from another isolate is visible on next get', () async {
      store.set('k', 'old');
      expect(store.get<String>('k'), 'old');
      final path = testDirectory.path;
      await Isolate.run(() async {
        final other = await RodiniaStore.open(path);
        other.set('k', 'new');
      });
      expect(store.get<String>('k'), 'new');
    });

    test('write that bypasses the wrapper is visible on next get', () {
      store.set('k', 'old');
      expect(store.get<String>('k'), 'old');
      storeSet(
        key: 'k',
        value: Uint8List.fromList('new'.codeUnits),
        encrypted: false,
        durable: false,
      );
      expect(store.get<String>('k'), 'new');
    });

    test('write through a second wrapper is visible on next get', () async {
      final other = await RodiniaStore.open(testDirectory.path);
      store.set('k', 'old');
      expect(store.get<String>('k'), 'old');
      other.set('k', 'new');
      expect(store.get<String>('k'), 'new');
      other.delete('k');
      expect(store.get<String>('k'), isNull);
    });

    test('mutating a returned map does not change the next get', () {
      store.set('profile', {'name': 'Tom'});
      final first = store.get<Map<String, dynamic>>('profile')!;
      first['name'] = 'changed';
      expect(store.get<Map<String, dynamic>>('profile'), {'name': 'Tom'});
    });

    test('returned bytes are a copy', () {
      final original = Uint8List.fromList([1, 2, 3]);
      store.set('bytes', original);
      original[0] = 9;
      final first = store.get<Uint8List>('bytes')!;
      expect(first, [1, 2, 3]);
      first[1] = 9;
      expect(store.get<Uint8List>('bytes'), [1, 2, 3]);
    });

    test('ttl expires while cached', () async {
      store.set('flash', 'sale', ttl: const Duration(milliseconds: 200));
      expect(store.get<String>('flash'), 'sale');
      await Future<void>.delayed(const Duration(milliseconds: 300));
      expect(store.get<String>('flash'), isNull);
      expect(store.contains('flash'), isFalse);
    });

    test('wrong key after lock does not serve cached plaintext', () {
      store.setEncryptionKey(RodiniaStore.generateEncryptionKey());
      store.set('secret', 'value', encrypted: true);
      expect(store.get<String>('secret'), 'value');
      store.lock();
      expect(
        () => store.get<String>('secret'),
        throwsA(isA<StoreError_EncryptionKeyNotSet>()),
      );
      store.setEncryptionKey(RodiniaStore.generateEncryptionKey());
      expect(() => store.get<String>('secret'), throwsA(isA<StoreError>()));
    });

    test('value larger than cacheBytes reads correctly', () async {
      final small = await RodiniaStore.open(
        testDirectory.path,
        cacheBytes: 1024,
      );
      final big = 'x' * (10 * 1024);
      small.set('big', big);
      expect(small.get<String>('big'), big);
      expect(small.get<String>('big'), big);
    });

    test('cacheBytes 0 behaves like no cache', () async {
      final uncached = await RodiniaStore.open(
        testDirectory.path,
        cacheBytes: 0,
      );
      uncached.set('k', 'v');
      expect(uncached.get<String>('k'), 'v');
      store.set('k', 'w');
      expect(uncached.get<String>('k'), 'w');
    });

    test('warm getAll matches cold getAll', () {
      store.set('a', 'one');
      store.set('b', 'two');
      final cold = store.getAll<String>();
      final warm = store.getAll<String>();
      expect(cold, {'a': 'one', 'b': 'two'});
      expect(warm, cold);
      store.set('c', 'three');
      expect(store.getAll<String>()['c'], 'three');
      store.delete('a');
      expect(store.getAll<String>().containsKey('a'), isFalse);
    });

    test('warm getAll returns fresh JSON objects', () {
      store.set('p', {'name': 'Tom'});
      store.getAll<Map<String, dynamic>>();
      final warm = store.getAll<Map<String, dynamic>>();
      warm['p']!['name'] = 'changed';
      expect(store.getAll<Map<String, dynamic>>()['p'], {'name': 'Tom'});
    });

    test('get after set returns the String that was set', () {
      final value = 'v' * 1024;
      store.set('k', value);
      expect(identical(store.get<String>('k'), value), isTrue);
      final other = 'w' * 1024;
      store.setIfAbsent('fresh', other);
      expect(identical(store.get<String>('fresh'), other), isTrue);
    });

    test('preloaded handle serves every value', () async {
      store.set('a', 'one');
      store.set('m', {'name': 'Tom'});
      store.set('n', 28);
      final handle = await RodiniaStore.open(testDirectory.path);
      await handle.preloaded;
      expect(handle.get<String>('a'), 'one');
      expect(handle.get<Map<String, dynamic>>('m'), {'name': 'Tom'});
      expect(handle.get<int>('n'), 28);
      expect(handle.get<String>('missing'), isNull);
    });

    test('preloaded handle getAll matches the store', () async {
      store.set('a', 'one');
      store.set('b', 'two');
      final handle = await RodiniaStore.open(testDirectory.path);
      await handle.preloaded;
      expect(handle.getAll<String>(), {'a': 'one', 'b': 'two'});
      store.delete('a');
      expect(handle.getAll<String>(), {'b': 'two'});
    });

    test('writes made while preloading win', () async {
      store.set('a', 'old');
      store.set('b', 'old');
      final handle = await RodiniaStore.open(testDirectory.path);
      handle.set('a', 'new');
      store.set('b', 'new');
      await handle.preloaded;
      expect(handle.get<String>('a'), 'new');
      expect(handle.get<String>('b'), 'new');
      expect(handle.getAll<String>(), {'a': 'new', 'b': 'new'});
    });

    test('preloaded value still expires', () async {
      store.set('t', 'soon', ttl: const Duration(milliseconds: 200));
      final handle = await RodiniaStore.open(testDirectory.path);
      await handle.preloaded;
      expect(handle.get<String>('t'), 'soon');
      await Future<void>.delayed(const Duration(milliseconds: 300));
      expect(handle.get<String>('t'), isNull);
    });

    test('encrypted value reads correctly after preload', () async {
      store.setEncryptionKey(RodiniaStore.generateEncryptionKey());
      store.set('s', 'secret', encrypted: true);
      final handle = await RodiniaStore.open(testDirectory.path);
      await handle.preloaded;
      expect(handle.get<String>('s'), 'secret');
      store.lock();
      expect(
        () => handle.get<String>('s'),
        throwsA(isA<StoreError_EncryptionKeyNotSet>()),
      );
    });

    test('preload false and cacheBytes 0 still read', () async {
      store.set('a', 'one');
      final noPreload = await RodiniaStore.open(
        testDirectory.path,
        preload: false,
      );
      final noCache = await RodiniaStore.open(
        testDirectory.path,
        cacheBytes: 0,
      );
      await noPreload.preloaded;
      await noCache.preloaded;
      expect(noPreload.get<String>('a'), 'one');
      expect(noCache.get<String>('a'), 'one');
    });

    test('number read back as String is its JSON text', () {
      store.set('n', 28);
      expect(store.get<int>('n'), 28);
      expect(store.get<String>('n'), '28');
    });
  });
}
