import 'dart:io';

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

    final baseDirectory =
        await getApplicationDocumentsDirectory();

    testDirectory = Directory(
      '${baseDirectory.path}/rodinia_integration_test',
    );

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
      store.set(
        'username',
        'Tom',
      );

      expect(
        store.get<String>('username'),
        'Tom',
      );
    });

    test('can set and get an integer value', () {
      store.set(
        'age',
        28,
      );

      expect(
        store.get<int>('age'),
        28,
      );
    });

    test('can set and get a boolean value', () {
      store.set(
        'is_authenticated',
        true,
      );

      expect(
        store.get<bool>('is_authenticated'),
        isTrue,
      );
    });

    test('returns null when key does not exist', () {
      expect(
        store.get<String>('does_not_exist'),
        isNull,
      );
    });

    test('can update an existing value', () {
      store.set(
        'theme',
        'light',
      );

      expect(
        store.get<String>('theme'),
        'light',
      );

      store.set(
        'theme',
        'dark',
      );

      expect(
        store.get<String>('theme'),
        'dark',
      );
    });

    test('can check whether a key exists', () {
      expect(
        store.contains('username'),
        isFalse,
      );

      store.set(
        'username',
        'Tom',
      );

      expect(
        store.contains('username'),
        isTrue,
      );
    });

    test('can delete a value', () {
      store.set(
        'username',
        'Tom',
      );

      expect(
        store.get<String>('username'),
        'Tom',
      );

      store.delete('username');

      expect(
        store.get<String>('username'),
        isNull,
      );

      expect(
        store.contains('username'),
        isFalse,
      );
    });

    test('can increment a value', () {
      expect(
        store.get<int>('visits'),
        isNull,
      );

      store.increment('visits');

      expect(
        store.get<int>('visits'),
        1,
      );

      store.increment('visits');

      expect(
        store.get<int>('visits'),
        2,
      );

      store.increment('visits');

      expect(
        store.get<int>('visits'),
        3,
      );
    });

    test('can increment an existing value', () {
      store.set(
        'visits',
        10,
      );

      store.increment('visits');

      expect(
        store.get<int>('visits'),
        11,
      );
    });

    test('can list stored keys', () {
      store.set('first', 'one');
      store.set('second', 'two');
      store.set('third', 'three');

      expect(
        store.keys,
        containsAll([
          'first',
          'second',
          'third',
        ]),
      );

      expect(
        store.length,
        3,
      );
    });

    test('can clear the store', () {
      store.set('first', 'one');
      store.set('second', 'two');

      expect(
        store.length,
        2,
      );

      store.clear();

      expect(
        store.length,
        0,
      );

      expect(
        store.keys,
        isEmpty,
      );
    });

    test('supports expiring values with TTL', () async {
      store.set(
        'flash_sale',
        true,
        ttl: const Duration(
          milliseconds: 300,
        ),
      );

      expect(
        store.get<bool>('flash_sale'),
        isTrue,
      );

      await Future<void>.delayed(
        const Duration(
          milliseconds: 500,
        ),
      );

      expect(
        store.get<bool>('flash_sale'),
        isNull,
      );
    });

    test('can store an encrypted value', () {
      final key = RodiniaStore.generateEncryptionKey();

      store.setEncryptionKey(key);

      store.set(
        'refresh_token',
        'super-secret-token',
        encrypted: true,
      );

      expect(
        store.get<String>('refresh_token'),
        'super-secret-token',
      );
    });

    test('encrypted value cannot be read without an encryption key', () {
      final key = RodiniaStore.generateEncryptionKey();

      store.setEncryptionKey(key);

      store.set(
        'secret',
        'very-secret-value',
        encrypted: true,
      );

      store.clear();

      expect(
        () => store.get<String>('secret'),
        throwsA(
          isA<StoreError_EncryptionKeyNotSet>(),
        ),
      );
    });

    test('encrypted value can be read after restoring the key', () {
      final key = RodiniaStore.generateEncryptionKey();

      store.setEncryptionKey(key);

      store.set(
        'secret',
        'very-secret-value',
        encrypted: true,
      );

      store.clear();

      expect(
        () => store.get<String>('secret'),
        throwsA(
          isA<StoreError_EncryptionKeyNotSet>(),
        ),
      );

      store.setEncryptionKey(key);

      expect(
        store.get<String>('secret'),
        'very-secret-value',
      );
    });

    test('watch emits storage events', () async {
      final events = <StorageEvent>[];

      final subscription = store.watch().listen(events.add);

      store.set(
        'username',
        'Tom',
      );

      await Future<void>.delayed(
        const Duration(
          milliseconds: 100,
        ),
      );

      await subscription.cancel();

      expect(
        events,
        isNotEmpty,
      );
    });

    test('watch emits events when values change', () async {
      final events = <StorageEvent>[];

      final subscription = store.watch().listen(events.add);

      store.set(
        'counter',
        1,
      );

      store.set(
        'counter',
        2,
      );

      store.delete('counter');

      await Future<void>.delayed(
        const Duration(
          milliseconds: 100,
        ),
      );

      await subscription.cancel();

      expect(
        events.length,
        greaterThanOrEqualTo(3),
      );
    });

    test('data survives reopening the store', () async {
      store.set(
        'username',
        'Tom',
      );

      store.set(
        'visits',
        42,
      );

      store.clear();

      store = await RodiniaStore.open(
        testDirectory.path,
      );

      expect(
        store.get<String>('username'),
        'Tom',
      );

      expect(
        store.get<int>('visits'),
        42,
      );
    });

    test('encrypted data survives reopening when encryption key is restored',
        () async {
      final key = RodiniaStore.generateEncryptionKey();

      store.setEncryptionKey(key);

      store.set(
        'refresh_token',
        'super-secret-token',
        encrypted: true,
      );

      store.clear();

      store = await RodiniaStore.open(
        testDirectory.path,
      );

      expect(
        () => store.get<String>('refresh_token'),
        throwsA(
          isA<StoreError_EncryptionKeyNotSet>(),
        ),
      );

      store.setEncryptionKey(key);

      expect(
        store.get<String>('refresh_token'),
        'super-secret-token',
      );
    });
  });
}