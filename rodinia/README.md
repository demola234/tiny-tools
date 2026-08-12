# Rodinia

Fast, synchronous, encrypted key-value storage for Flutter, backed by a Rust core.

## Layout

```
rodinia/
├── rodinia_ffi/                       # Rust crate — the storage engine + flutter_rust_bridge API
├── rodinia_flutter/                   # Flutter plugin — wraps rodinia_ffi for Dart consumers
└── examples/
    └── rodinia_flutter_example/       # Demo app, depends on rodinia_flutter by path
```

`rodinia_ffi` is a standalone Rust crate (`cargo build`/`cargo test` work on their own,
no Flutter toolchain required). `rodinia_flutter` exposes it to Dart via
[flutter_rust_bridge](https://cbeuw.github.io/flutter_rust_bridge/): its
`flutter_rust_bridge.yaml` points `rust_root` at `../rodinia_ffi`, and the
Android/iOS/macOS/Linux/Windows build configs invoke `rodinia_ffi`'s `Cargo.toml`
directly via `cargokit` — there's no copy of the Rust source inside the plugin.

## Getting started

```sh
just build      # cargo build the Rust core
just gen         # regenerate Dart bindings after changing rodinia_ffi's API
just analyze      # flutter analyze the plugin + example
just example      # run the example app on a connected device/simulator
```

See the [justfile](justfile) for the full command list.

## Workflow

1. Implement/change the storage engine in `rodinia_ffi/src/` (anything under
   `src/api/` is the surface exposed to Dart; everything else is internal).
2. `just gen` from the repo root to regenerate `rodinia_flutter`'s Dart bindings.
3. `just example` to try it in the demo app.

`cargo test` in `rodinia_ffi` doesn't need step 2 — iterate on the Rust logic
in isolation, then regenerate bindings once it's ready to expose to Dart.
