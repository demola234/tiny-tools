import 'dart:async';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';
import 'package:path_provider/path_provider.dart';
import 'package:rodinia_benchmarks/stores.dart';

/// Which part of the benchmark this app launch runs, chosen with
/// `--dart-define=BENCH=...`:
///
/// * `suite` (default): every store on every operation, in one process.
/// * `fill:<store>`: writes [coldKeys] values into `<store>` and exits.
/// * `open:<store>`: run in a fresh process after `fill:<store>`; times
///   opening the filled store and measures the memory it takes.
const bench = String.fromEnvironment('BENCH', defaultValue: 'suite');

const rounds = 3;
const plainKeys = 500;
const encryptedKeys = 500;
const secureStorageKeys = 100;
const coldKeys = 1000;
const readAllCalls = 20;
final value16 = 'v' * 16;
final value1k = 'v' * 1024;

final coldStores = <String, Future<KvStore> Function(Directory dir)>{
  'rodinia': (dir) => RodiniaKv.open(dir),
  'hive_ce': (dir) => HiveKv.open(dir),
  'shared_preferences': (_) => SharedPreferencesKv.open(),
  'sqflite': (dir) => SqfliteKv.open(dir),
  'get_storage': (dir) => GetStorageKv.open(dir),
  'flutter_secure_storage': (_) async => SecureStorageKv(),
};

class Scenario {
  const Scenario(this.name, this.op, {this.prefill, this.calls});

  final String name;
  final FutureOr<void> Function(KvStore store, int i) op;
  final String? prefill;
  final int? calls;
}

final scenarios = [
  Scenario('write 16 B', (s, i) => s.write('k$i', value16)),
  Scenario('read 16 B', (s, i) => s.read('k$i'), prefill: value16),
  Scenario('write 1 KiB', (s, i) => s.write('k$i', value1k)),
  Scenario('read 1 KiB', (s, i) => s.read('k$i'), prefill: value1k),
  Scenario('delete', (s, i) => s.delete('k$i'), prefill: value16),
  Scenario(
    'read all',
    (s, _) => s.readAll(),
    prefill: value16,
    calls: readAllCalls,
  ),
];

Future<double> timeOps(KvStore store, int ops, Scenario scenario) async {
  final watch = Stopwatch()..start();
  for (var i = 0; i < ops; i++) {
    final pending = scenario.op(store, i);
    if (pending is Future) await pending;
  }
  await store.settle();
  return watch.elapsedMicroseconds / ops;
}

Future<void> prepare(KvStore store, int keys, String? value) async {
  await store.clear();
  for (var i = 0; value != null && i < keys; i++) {
    await store.write('k$i', value);
  }
  await store.settle();
}

Future<double> medianMicros(KvStore store, int keys, Scenario scenario) async {
  final ops = scenario.calls ?? keys;
  final samples = <double>[];
  for (var round = 0; round <= rounds; round++) {
    await prepare(store, keys, scenario.prefill);
    final micros = await timeOps(store, ops, scenario);
    if (round > 0) samples.add(micros);
  }
  samples.sort();
  return samples[samples.length ~/ 2];
}

Future<void> verify(KvStore store) async {
  await store.clear();
  await store.write('a', value16);
  await store.write('b', value1k);
  expect(await store.read('a'), value16, reason: store.name);
  expect(await store.read('b'), value1k, reason: store.name);
  expect(await store.readAll(), {
    'a': value16,
    'b': value1k,
  }, reason: store.name);
  await store.delete('a');
  expect(await store.read('a'), isNull, reason: store.name);
  await store.clear();
}

String table(
  String title,
  Map<String, Map<String, double>> results, {
  String unit = 'median µs/op',
}) {
  final stores = results.keys.toList();
  final rows = results.values.first.keys;
  final out = StringBuffer()
    ..writeln('\n### $title ($unit, lower is better)\n')
    ..writeln('| operation | ${stores.join(' | ')} |')
    ..writeln('|---|${'---:|' * stores.length}');
  for (final row in rows) {
    final cells = stores.map((s) => results[s]![row]!.toStringAsFixed(2));
    out.writeln('| $row | ${cells.join(' | ')} |');
  }
  return out.toString();
}

void result(String store, String metric, num value) =>
    debugPrint('RESULT\t$store\t$metric\t$value');

void main() {
  final binding = IntegrationTestWidgetsFlutterBinding.ensureInitialized();
  final report = <String, Map<String, Map<String, double>>>{};

  tearDownAll(() => binding.reportData = report);

  if (bench == 'suite') {
    suite(report);
    return;
  }
  final [phase, name] = bench.split(':');
  final open = coldStores[name];
  if (open == null) throw ArgumentError('unknown store "$name" in $bench');
  switch (phase) {
    case 'fill':
      fill(name, open);
    case 'open':
      coldOpen(name, open, report);
    default:
      throw ArgumentError('unknown phase "$phase" in $bench');
  }
}

Future<Directory> coldDir() async {
  final dir = Directory('${(await getApplicationSupportDirectory()).path}/cold');
  return dir..createSync(recursive: true);
}

void fill(String name, Future<KvStore> Function(Directory) open) {
  testWidgets('fill $name with $coldKeys x 1 KiB', (_) async {
    final store = await open(await coldDir());
    await store.clear();
    for (var i = 0; i < coldKeys; i++) {
      await store.write('c$i', value1k);
    }
    await store.settle();
    await store.close();
    debugPrint('FILLED\t$name\t$coldKeys');
  }, timeout: Timeout.none);
}

void coldOpen(
  String name,
  Future<KvStore> Function(Directory) open,
  Map<String, Map<String, Map<String, double>>> report,
) {
  testWidgets('cold open of $name holding $coldKeys x 1 KiB', (_) async {
    final dir = await coldDir();
    final rssBefore = ProcessInfo.currentRss;
    final watch = Stopwatch()..start();

    final store = await open(dir);
    final openMs = watch.elapsedMicroseconds / 1000;
    await store.ready();
    final readyMs = watch.elapsedMicroseconds / 1000;

    watch.reset();
    final first = await store.read('c0');
    final firstReadUs = watch.elapsedMicroseconds.toDouble();

    watch.reset();
    for (var i = 0; i < coldKeys; i++) {
      if (await store.read('c$i') != value1k) {
        fail('$name lost c$i after reopening');
      }
    }
    final readEveryMs = watch.elapsedMicroseconds / 1000;
    final rssMiB = (ProcessInfo.currentRss - rssBefore) / (1024 * 1024);

    expect(first, value1k, reason: name);
    expect((await store.readAll()).length, coldKeys, reason: name);

    final metrics = {
      'open (ms)': openMs,
      'ready (ms)': readyMs,
      'first read (µs)': firstReadUs,
      'read every key (ms)': readEveryMs,
      'RSS growth (MiB)': rssMiB,
    };
    metrics.forEach((metric, value) => result(name, metric, value));
    report['Cold start, $coldKeys x 1 KiB'] = {name: metrics};

    await store.clear();
    await store.settle();
    await store.close();
  }, timeout: Timeout.none);
}

void suite(Map<String, Map<String, Map<String, double>>> report) {
  late Directory root;

  setUpAll(() async {
    root = Directory('${(await getApplicationSupportDirectory()).path}/bench');
    if (root.existsSync()) root.deleteSync(recursive: true);
    root.createSync(recursive: true);
  });

  tearDownAll(() {
    if (root.existsSync()) root.deleteSync(recursive: true);
  });

  Future<void> runGroup(
    String title,
    List<Future<KvStore> Function()> openers,
    int Function(KvStore store) keysFor,
  ) async {
    final results = <String, Map<String, double>>{};
    for (final open in openers) {
      final store = await open();
      await verify(store);
      final keys = keysFor(store);
      final timings = results[store.name] = {};
      for (final scenario in scenarios) {
        final micros = await medianMicros(store, keys, scenario);
        timings[scenario.name] = micros;
        result(store.name, scenario.name, micros);
      }
      await store.clear();
      await store.settle();
      await store.close();
    }
    report[title] = results;
    debugPrint(table(title, results));
  }

  testWidgets('plain key-value storage', (_) async {
    await runGroup('Plain storage, $plainKeys keys', [
      () => RodiniaKv.open(root),
      () => RodiniaKv.open(root, cacheBytes: 0),
      () => HiveKv.open(root),
      SharedPreferencesKv.open,
      () => SqfliteKv.open(root),
      () => GetStorageKv.open(root),
    ], (_) => plainKeys);
  }, timeout: Timeout.none);

  testWidgets('encrypted key-value storage', (_) async {
    await runGroup(
      'Encrypted storage ($encryptedKeys keys; flutter_secure_storage $secureStorageKeys)',
      [
        () => RodiniaKv.open(root, encrypted: true),
        () => HiveKv.open(root, encrypted: true),
        () async => SecureStorageKv(),
      ],
      (store) => store is SecureStorageKv ? secureStorageKeys : encryptedKeys,
    );
  }, timeout: Timeout.none);
}
