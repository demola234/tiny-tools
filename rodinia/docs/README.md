# Rodinia documentation

Rodinia is a key-value store for Flutter apps. The storage engine is written in
Rust (`rodinia_ffi`); a Flutter plugin (`rodinia_flutter`) exposes it to Dart as
a small synchronous API with optional encryption, expiry, counters, lists and
change events.

```dart
final store = await RodiniaStore.open(
  (await getApplicationDocumentsDirectory()).path,
);
store.set('username', 'Tom');
store.get<String>('username'); // 'Tom'
```

## Pages

| Page | Read it when you want to know |
|---|---|
| [Architecture](architecture.md) | how a call travels from Dart to disk and back, where state lives, how the Dart cache stays correct, what runs on which thread |
| [Storage format](storage-format.md) | what is in `rodinia.wal`, how the store recovers after a crash, how compaction works, what `durable` really guarantees, how values are encrypted |
| [Dart API](dart-api.md) | how to use every method, which types you can store, and the edge cases (expiry, encryption, `clear` vs `lock`, counters, lists, events) |
| [Development](development.md) | how the repository is laid out, how to build, regenerate bindings, test on a device and run the benchmarks, and the known issues |

The design decisions behind the current engine are recorded in
[superpowers/specs/2026-11-11-rodinia-performance-design.md](superpowers/specs/2026-11-11-rodinia-performance-design.md),
and the measured results are in [../benchmarks/README.md](../benchmarks/README.md).

## Rodinia in five sentences

1. Every write is appended to one log file, `rodinia.wal`, as a small
   CRC-checked record; nothing is ever rewritten in place.
2. Rust keeps an in-memory index from each key to where its latest value sits in
   that file, and reads the value from disk when it is asked for.
3. The Dart side keeps its own size-capped cache of recently used values, so most
   reads never cross into Rust at all.
4. A change counter shared between Rust and Dart lets every Dart handle notice
   writes it did not make (from another isolate or another handle) and drop
   anything stale.
5. When most of the log is old history, the store rewrites it with only the live
   values, crash-safely.
