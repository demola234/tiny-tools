use criterion::{black_box, criterion_group, criterion_main, Criterion};
use rodinia_ffi::store::Store;
use std::sync::atomic::{AtomicUsize, Ordering};

fn temp_dir() -> String {
    static COUNTER: AtomicUsize = AtomicUsize::new(0);
    let n = COUNTER.fetch_add(1, Ordering::Relaxed);
    std::env::temp_dir()
        .join(format!("rodinia_bench_{}_{}", std::process::id(), n))
        .to_string_lossy()
        .into_owned()
}

fn bench_set(c: &mut Criterion) {
    let dir = temp_dir();
    let store = Store::open(&dir).unwrap();
    c.bench_function("store_set", |b| {
        let mut i: u64 = 0;
        b.iter(|| {
            i += 1;
            store
                .set(format!("key{i}"), black_box(b"value".to_vec()), None, false)
                .unwrap();
        });
    });
    let _ = std::fs::remove_dir_all(&dir);
}

fn bench_get(c: &mut Criterion) {
    let dir = temp_dir();
    let store = Store::open(&dir).unwrap();
    store.set("key".to_string(), b"value".to_vec(), None, false).unwrap();
    c.bench_function("store_get", |b| {
        b.iter(|| black_box(store.get("key").unwrap()));
    });
    let _ = std::fs::remove_dir_all(&dir);
}

fn bench_increment(c: &mut Criterion) {
    let dir = temp_dir();
    let store = Store::open(&dir).unwrap();
    c.bench_function("store_increment", |b| {
        b.iter(|| black_box(store.increment("counter", 1, None).unwrap()));
    });
    let _ = std::fs::remove_dir_all(&dir);
}

fn bench_contains_delete(c: &mut Criterion) {
    let dir = temp_dir();
    let store = Store::open(&dir).unwrap();
    c.bench_function("store_contains_delete", |b| {
        let mut i: u64 = 0;
        b.iter(|| {
            i += 1;
            let key = format!("key{i}");
            store.set(key.clone(), b"value".to_vec(), None, false).unwrap();
            black_box(store.contains(&key));
            store.delete(&key).unwrap();
        });
    });
    let _ = std::fs::remove_dir_all(&dir);
}

criterion_group!(benches, bench_set, bench_get, bench_increment, bench_contains_delete);
criterion_main!(benches);
