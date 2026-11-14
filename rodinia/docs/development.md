# Development

How the repository is laid out, how to build and test it, how to run the
benchmarks, and what is known to be unfinished.

## Repository layout

```
rodinia/
├── rodinia_ffi/                  Rust crate: the storage engine
│   ├── src/
│   │   ├── api/store.rs          everything exposed to Dart (one function per operation)
│   │   ├── store.rs              Store: index, locks, encryption, expiry, events, change counter
│   │   ├── wal.rs                the log file: open, replay, append, read, compact
│   │   ├── record.rs             byte layout of one log record
│   │   ├── legacy.rs             reader for the old log format, used once to migrate
│   │   ├── cache.rs              plain data types (IndexEntry, ValueLoc, StoredItem, Preload, …)
│   │   ├── crypto.rs             AES-256-GCM Cipher and EncryptionKey
│   │   ├── events.rs             EventBus and pattern matching for watch()
│   │   ├── error.rs              StoreError
│   │   └── frb_generated.rs      generated, do not edit
│   └── benches/store.rs          Criterion benchmarks of the Rust core
├── rodinia_flutter/              Flutter plugin
│   ├── lib/rodinia_store.dart    RodiniaStore, the public Dart API
│   ├── lib/src/value_cache.dart  ValueCache and CachedValue
│   ├── lib/src/rust/             generated bindings, do not edit
│   ├── test/                     Dart unit tests (ValueCache)
│   └── cargokit/                 builds rodinia_ffi from each platform's build system
├── examples/rodinia_flutter_example/
│   ├── lib/main.dart             demo app
│   └── integration_test/         the on-device test suite for RodiniaStore
├── benchmarks/                   Flutter app comparing rodinia with other storage packages
└── docs/                         this documentation
```

`rodinia_flutter` contains no copy of the Rust code: `flutter_rust_bridge.yaml`
points `rust_root` at `../rodinia_ffi`, and cargokit compiles that crate as part of
each platform build.

## Tools

- Rust (stable) with `cargo`.
- Flutter (the benchmarks were last run on 3.47.9).
- `flutter_rust_bridge_codegen` 2.12.0, matching the `flutter_rust_bridge`
  version pinned in both `Cargo.toml` and `pubspec.yaml`
  (`cargo install flutter_rust_bridge_codegen --version 2.12.0`).
- [`just`](https://github.com/casey/just) for the shortcuts below (optional).
- For Android: the Android SDK and NDK. The Rust targets for Android are added by
  cargokit on the first build.

## Commands

Run from `rodinia/`:

| Command | Does |
|---|---|
| `just build` | `cargo build` the Rust core |
| `just test` | Rust unit tests |
| `just lint` | `cargo clippy --all-targets -- -D warnings` |
| `just analyze` | `flutter analyze` the plugin and the example app |
| `just fmt` | format Rust and Dart |
| `just check` | lint, test, analyze: what CI should run |
| `just gen` | regenerate the Dart bindings |
| `just example` | run the demo app |
| `just bench` | Rust Criterion benchmarks |
| `just bench-flutter <device>` | the Flutter benchmark suite on one device (macOS by default) |
| `just bench-all <device>` | the full Flutter benchmark, including cold start, on an Android device |

## Changing the Rust API

1. Change or add functions in `rodinia_ffi/src/api/store.rs`. Everything under
   `src/api/` is exposed to Dart; nothing else is.
   - Mark a function `#[flutter_rust_bridge::frb(sync)]` to make it synchronous in
     Dart (it runs on the calling isolate). Leave it unmarked to get a `Future`
     that runs on a worker thread; use that for anything that may take long.
   - Use `i64`, not `u64` or `usize`, in signatures and returned structs: the
     generator maps unsigned 64-bit types to `BigInt` in Dart.
2. `just gen`. This rewrites `rodinia_flutter/lib/src/rust/**` and
   `rodinia_ffi/src/frb_generated.rs`; commit them with your change.
3. Use the new function from `rodinia_store.dart`. If it changes data, wrap it
   in `_mutate` with the right `changes` count, and make sure the Rust side
   increments the change counter once per change (see
   [Architecture](architecture.md#keeping-every-cache-correct-the-change-counter)).

`cargo test` needs none of this: iterate on the engine in Rust first.

## Tests

| Suite | Where | How to run | Covers |
|---|---|---|---|
| Rust unit tests | `#[cfg(test)]` modules in `rodinia_ffi/src/*.rs` | `just test` | record encoding, replay and torn-tail recovery, compaction, migration, encryption, expiry, lists, counters, the change counter, preload |
| Dart unit tests | `rodinia_flutter/test/value_cache_test.dart` | `cd rodinia_flutter && flutter test` | ValueCache eviction, size accounting, expiry, the complete flag |
| Device tests | `examples/rodinia_flutter_example/integration_test/store_test.dart` | see below | the whole stack on a real device: every API, cache coherence across handles and isolates, encryption and `lock`, preload |

Run the device tests in profile mode on a connected phone:

```sh
cd examples/rodinia_flutter_example
flutter drive --profile --android-skip-build-dependency-validation \
  -d <device-id> \
  --driver=test_driver/integration_test.dart \
  --target=integration_test/store_test.dart
```

- `--profile` builds the Rust code for one architecture only and runs AOT Dart;
  a debug build compiles the Rust core for three Android architectures and is much
  slower to build.
- `--android-skip-build-dependency-validation` is needed because the example
  app pins Gradle 8.10.2, below Flutter's current minimum.
- The two `watch` tests currently fail (see [Known issues](#known-issues)); every
  other test should pass.

## Benchmarks

There are two benchmark suites.

**Rust core** (`rodinia_ffi/benches/store.rs`, Criterion): `just bench`. It
measures the engine without Dart or the bridge: set, get, writes with and
without durability, lists, scans and the cost of reopening (replay). Compare
against a saved baseline with `cargo bench -- --baseline <name>`.

**Flutter, against other packages** (`benchmarks/`): compares rodinia with
`hive_ce`, `shared_preferences`, `sqflite`, `get_storage` and
`flutter_secure_storage` on a device:

```sh
cd benchmarks
tool/run_all.sh <device-id>      # suite + cold start for every store (~75 min)
tool/tables.py                   # Markdown tables from build/bench/results.tsv
```

How each scenario is measured, how to read the numbers fairly, and the latest
results are in [../benchmarks/README.md](../benchmarks/README.md).

## Android build notes

Things that have broken builds before:

- **`flutter drive` uninstalls the app when it finishes**, deleting its data,
  unless you pass `--keep-app-running`. The benchmark runner relies on this for
  its fill-then-open runs.
- **Gradle 9** removed `project.exec`; `cargokit/gradle/plugin.gradle` uses an
  injected `ExecOperations` instead.
- The plugin compiles against **compileSdk 34**; some AndroidX dependencies
  require it.
- The benchmark app pins **NDK 26.3.11579264** for itself and its plugins; an NDK
  directory that contains only installer metadata breaks the build.
- **On a machine with little RAM**, Gradle daemons grow across builds and push the
  system into swap, which can fill the disk. Keep `org.gradle.jvmargs` modest
  (the apps use `-Xmx2G`), and stop daemons with `./gradlew --stop` between long
  runs (`tool/run_all.sh` does this).
- **Keep the phone awake and unlocked** during device runs. A locked screen
  backgrounds the app, and Samsung phones freeze background apps, which drops the
  test connection: `adb shell settings put system screen_off_timeout 1800000`
  and `adb shell wm dismiss-keyguard`.

## Known issues

- **`watch` tests time out.** Cancelling a `watch` subscription appears never to
  complete (`await subscription.cancel()` on the bridge stream), so the two
  `watch` device tests hit their timeout. Events themselves are delivered.
- **Automatic compaction runs inline.** The write that triggers it pays for
  rewriting and syncing the file, 15–20 ms on a phone, under the write lock. It
  should move to a background thread.
- **First reads after open decode UTF-8.** Preloaded values are cached as bytes,
  so the first `get<String>` of each still decodes it; reading every key once
  right after start-up is slower than in stores that keep decoded strings.
- **`increment` clears expiry.** The Dart wrapper never passes a TTL, so each
  increment makes the key permanent.
- **One store per process.** A second `open` with a different path silently uses
  the first store.
- **The encryption key is not wiped from memory** when it is replaced or after
  `lock()`; only the buffer passed in is.
- **`CacheInvalidated` events are never sent.**
- **`export`/`import`** exist in the Rust `Store` but are not exposed to Dart.
