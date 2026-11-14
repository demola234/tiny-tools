import 'dart:async';
import 'dart:io';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:get_storage/get_storage.dart';
import 'package:hive_ce/hive.dart';
import 'package:rodinia_flutter/rodinia_store.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:sqflite/sqflite.dart';

/// The operations every benchmarked package is measured on.
///
/// Methods return [FutureOr] so synchronous stores (rodinia, Hive reads) are
/// timed without the cost of an artificial `await`.
abstract interface class KvStore {
  String get name;
  FutureOr<void> write(String key, String value);
  FutureOr<String?> read(String key);
  FutureOr<void> delete(String key);
  FutureOr<Map<String, String>> readAll();
  FutureOr<void> clear();

  /// Waits until every write made so far has been handed to the operating
  /// system. Stores whose awaited writes already do that leave it empty;
  /// stores that write to disk in the background wait for it here, so their
  /// timings include the disk work.
  FutureOr<void> settle();

  /// Waits for loading a store does in the background after it opens.
  /// Stores that finish loading inside `open` leave it empty.
  FutureOr<void> ready();
  FutureOr<void> close();
}

class RodiniaKv implements KvStore {
  RodiniaKv._(this._store, this._encrypted, this._cached);

  final RodiniaStore _store;
  final bool _encrypted;
  final bool _cached;

  static Future<RodiniaKv> open(
    Directory dir, {
    bool encrypted = false,
    int? cacheBytes,
  }) async {
    final store = cacheBytes == null
        ? await RodiniaStore.open('${dir.path}/rodinia')
        : await RodiniaStore.open(
            '${dir.path}/rodinia',
            cacheBytes: cacheBytes,
          );
    if (encrypted) store.setEncryptionKey(RodiniaStore.generateEncryptionKey());
    return RodiniaKv._(store, encrypted, cacheBytes != 0);
  }

  @override
  String get name => [
    'rodinia',
    if (_encrypted) '(AES-GCM)',
    if (!_cached) '(no cache)',
  ].join(' ');

  @override
  void write(String key, String value) =>
      _store.set(key, value, encrypted: _encrypted);

  @override
  String? read(String key) => _store.get<String>(key);

  @override
  void delete(String key) => _store.delete(key);

  @override
  Map<String, String> readAll() => _store.getAll<String>();

  @override
  void clear() => _store.clear();

  @override
  void settle() {}

  @override
  Future<void> ready() => _store.preloaded;

  @override
  void close() {}
}

class HiveKv implements KvStore {
  HiveKv._(this._box, this._encrypted);

  final Box<String> _box;
  final bool _encrypted;

  static Future<HiveKv> open(Directory dir, {bool encrypted = false}) async {
    Hive.init('${dir.path}/hive');
    final box = await Hive.openBox<String>(
      encrypted ? 'bench_encrypted' : 'bench',
      encryptionCipher: encrypted
          ? HiveAesCipher(Hive.generateSecureKey())
          : null,
    );
    return HiveKv._(box, encrypted);
  }

  @override
  String get name => _encrypted ? 'hive_ce (AES-CBC)' : 'hive_ce';

  @override
  Future<void> write(String key, String value) => _box.put(key, value);

  @override
  String? read(String key) => _box.get(key);

  @override
  Future<void> delete(String key) => _box.delete(key);

  @override
  Map<String, String> readAll() => _box.toMap().cast<String, String>();

  @override
  Future<void> clear() => _box.clear();

  @override
  void settle() {}

  @override
  void ready() {}

  @override
  Future<void> close() => _box.close();
}

class SharedPreferencesKv implements KvStore {
  SharedPreferencesKv._(this._prefs);

  final SharedPreferencesWithCache _prefs;

  static Future<SharedPreferencesKv> open() async {
    final prefs = await SharedPreferencesWithCache.create(
      cacheOptions: const SharedPreferencesWithCacheOptions(),
    );
    return SharedPreferencesKv._(prefs);
  }

  @override
  String get name => 'shared_preferences';

  @override
  Future<void> write(String key, String value) => _prefs.setString(key, value);

  @override
  String? read(String key) => _prefs.getString(key);

  @override
  Future<void> delete(String key) => _prefs.remove(key);

  @override
  Map<String, String> readAll() => {
    for (final key in _prefs.keys) key: _prefs.getString(key)!,
  };

  @override
  Future<void> clear() => _prefs.clear();

  @override
  void settle() {}

  @override
  void ready() {}

  @override
  void close() {}
}

class SqfliteKv implements KvStore {
  SqfliteKv._(this._db);

  final Database _db;

  static Future<SqfliteKv> open(Directory dir) async {
    final db = await openDatabase(
      '${dir.path}/sqflite.db',
      version: 1,
      onCreate: (db, _) => db.execute(
        'CREATE TABLE kv (key TEXT PRIMARY KEY, value TEXT NOT NULL)',
      ),
    );
    return SqfliteKv._(db);
  }

  @override
  String get name => 'sqflite';

  @override
  Future<void> write(String key, String value) => _db.insert('kv', {
    'key': key,
    'value': value,
  }, conflictAlgorithm: ConflictAlgorithm.replace);

  @override
  Future<String?> read(String key) async {
    final rows = await _db.query(
      'kv',
      columns: ['value'],
      where: 'key = ?',
      whereArgs: [key],
    );
    return rows.isEmpty ? null : rows.first['value'] as String;
  }

  @override
  Future<void> delete(String key) =>
      _db.delete('kv', where: 'key = ?', whereArgs: [key]);

  @override
  Future<Map<String, String>> readAll() async {
    final rows = await _db.query('kv');
    return {for (final r in rows) r['key'] as String: r['value'] as String};
  }

  @override
  Future<void> clear() => _db.delete('kv');

  @override
  void settle() {}

  @override
  void ready() {}

  @override
  Future<void> close() => _db.close();
}

class SecureStorageKv implements KvStore {
  final _storage = const FlutterSecureStorage(
    mOptions: MacOsOptions(usesDataProtectionKeychain: false),
  );

  @override
  String get name => 'flutter_secure_storage';

  @override
  Future<void> write(String key, String value) =>
      _storage.write(key: key, value: value);

  @override
  Future<String?> read(String key) => _storage.read(key: key);

  @override
  Future<void> delete(String key) => _storage.delete(key: key);

  @override
  Future<Map<String, String>> readAll() => _storage.readAll();

  @override
  Future<void> clear() => _storage.deleteAll();

  @override
  void settle() {}

  @override
  void ready() {}

  @override
  void close() {}
}

class GetStorageKv implements KvStore {
  GetStorageKv._(this._box);

  final GetStorage _box;

  static Future<GetStorageKv> open(Directory dir) async {
    final path = Directory('${dir.path}/get_storage')..createSync();
    final box = GetStorage('bench', path.path);
    await box.initStorage;
    return GetStorageKv._(box);
  }

  @override
  String get name => 'get_storage';

  @override
  Future<void> write(String key, String value) => _box.write(key, value);

  @override
  String? read(String key) => _box.read<String>(key);

  @override
  Future<void> delete(String key) => _box.remove(key);

  @override
  Map<String, String> readAll() => {
    for (final key in _box.getKeys<Iterable<String>>())
      key: _box.read<String>(key)!,
  };

  @override
  Future<void> clear() => _box.erase();

  @override
  Future<void> settle() async {
    await Future<void>.delayed(Duration.zero);
    await _box.queue.add(() async {});
  }

  @override
  void ready() {}

  @override
  Future<void> close() => settle();
}
