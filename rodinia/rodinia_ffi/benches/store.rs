use criterion::{
    black_box, criterion_group, criterion_main, measurement::WallTime, Bencher, BenchmarkId,
    Criterion, Throughput,
};
use rodinia_ffi::store::Store;
use std::path::PathBuf;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::{Duration, Instant};

const VALUE_SIZES: [usize; 3] = [64, 1024, 16 * 1024];
const SCAN_SIZES: [usize; 3] = [100, 1_000, 10_000];
const LIST_SIZES: [usize; 3] = [10, 100, 1_000];
const RESET_EVERY: u64 = 1_000;

struct BenchDir(PathBuf);

impl BenchDir {
    fn new() -> Self {
        static COUNTER: AtomicUsize = AtomicUsize::new(0);
        let n = COUNTER.fetch_add(1, Ordering::Relaxed);
        Self(std::env::temp_dir().join(format!("rodinia_bench_{}_{}", std::process::id(), n)))
    }

    fn path(&self) -> &str {
        self.0.to_str().unwrap()
    }

    fn open(&self) -> Store {
        Store::open(self.path()).unwrap()
    }
}

impl Drop for BenchDir {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

fn encrypted_store(dir: &BenchDir) -> Store {
    let store = dir.open();
    store.set_encryption_key(vec![7u8; 32]).unwrap();
    store
}

fn fill(store: &Store, n: usize, ttl_ms: Option<i64>) {
    for i in 0..n {
        store
            .set(format!("key{i}"), b"value".to_vec(), ttl_ms, false, false)
            .unwrap();
    }
}

fn reset(store: &Store) {
    store.clear(false).unwrap();
    store.compact().unwrap();
}

fn iter_writes(b: &mut Bencher<WallTime>, store: &Store, mut op: impl FnMut(u64)) {
    b.iter_custom(|iters| {
        let mut total = Duration::ZERO;
        let mut done = 0;
        while done < iters {
            let end = iters.min(done + RESET_EVERY);
            let start = Instant::now();
            for i in done..end {
                op(i);
            }
            total += start.elapsed();
            done = end;
            reset(store);
        }
        total
    });
}

fn iter_writes_with_setup(
    b: &mut Bencher<WallTime>,
    store: &Store,
    mut setup: impl FnMut(),
    mut op: impl FnMut(),
) {
    b.iter_custom(|iters| {
        let mut total = Duration::ZERO;
        for i in 0..iters {
            setup();
            let start = Instant::now();
            op();
            total += start.elapsed();
            if (i + 1) % RESET_EVERY == 0 {
                reset(store);
            }
        }
        total
    });
}

fn bench_set(c: &mut Criterion) {
    let mut group = c.benchmark_group("set");
    for size in VALUE_SIZES {
        group.throughput(Throughput::Bytes(size as u64));
        for encrypted in [false, true] {
            let label = if encrypted { "encrypted" } else { "plain" };
            group.bench_with_input(BenchmarkId::new(label, size), &size, |b, &size| {
                let dir = BenchDir::new();
                let store = encrypted_store(&dir);
                let value = vec![0xABu8; size];
                iter_writes(b, &store, |i| {
                    store
                        .set(
                            format!("key{i}"),
                            black_box(value.clone()),
                            None,
                            encrypted,
                            false,
                        )
                        .unwrap();
                });
            });
        }
    }
    group.finish();
}

fn bench_get(c: &mut Criterion) {
    let mut group = c.benchmark_group("get");
    for size in VALUE_SIZES {
        group.throughput(Throughput::Bytes(size as u64));
        for encrypted in [false, true] {
            let label = if encrypted { "encrypted" } else { "plain" };
            group.bench_with_input(BenchmarkId::new(label, size), &size, |b, &size| {
                let dir = BenchDir::new();
                let store = encrypted_store(&dir);
                store
                    .set(
                        "key".to_string(),
                        vec![0xABu8; size],
                        None,
                        encrypted,
                        false,
                    )
                    .unwrap();
                b.iter(|| black_box(store.get(black_box("key")).unwrap()));
            });
        }
    }
    group.bench_function("miss", |b| {
        let dir = BenchDir::new();
        let store = dir.open();
        b.iter(|| black_box(store.get(black_box("absent")).unwrap()));
    });
    group.finish();
}

fn bench_writes(c: &mut Criterion) {
    let mut group = c.benchmark_group("writes");
    group.bench_function("set_with_ttl", |b| {
        let dir = BenchDir::new();
        let store = dir.open();
        iter_writes(b, &store, |i| {
            store
                .set(
                    format!("key{i}"),
                    b"value".to_vec(),
                    Some(60_000),
                    false,
                    false,
                )
                .unwrap();
        });
    });
    group.bench_function("set_overwrite", |b| {
        let dir = BenchDir::new();
        let store = dir.open();
        iter_writes(b, &store, |_| {
            store
                .set("key".to_string(), b"value".to_vec(), None, false, false)
                .unwrap();
        });
    });
    group.bench_function("set_if_absent_hit", |b| {
        let dir = BenchDir::new();
        let store = dir.open();
        store
            .set("key".to_string(), b"value".to_vec(), None, false, false)
            .unwrap();
        b.iter(|| {
            black_box(
                store
                    .set_if_absent("key".to_string(), b"v".to_vec(), None, false, false)
                    .unwrap(),
            )
        });
    });
    group.bench_function("set_if_absent_miss", |b| {
        let dir = BenchDir::new();
        let store = dir.open();
        iter_writes(b, &store, |i| {
            black_box(
                store
                    .set_if_absent(format!("key{i}"), b"v".to_vec(), None, false, false)
                    .unwrap(),
            );
        });
    });
    group.bench_function("increment", |b| {
        let dir = BenchDir::new();
        let store = dir.open();
        iter_writes(b, &store, |_| {
            black_box(store.increment("counter", 1, None, false).unwrap());
        });
    });
    group.bench_function("set_contains_delete", |b| {
        let dir = BenchDir::new();
        let store = dir.open();
        iter_writes(b, &store, |i| {
            let key = format!("key{i}");
            store
                .set(key.clone(), b"value".to_vec(), None, false, false)
                .unwrap();
            black_box(store.contains(&key));
            store.delete(&key, false).unwrap();
        });
    });
    group.finish();
}

fn bench_lists(c: &mut Criterion) {
    let mut group = c.benchmark_group("lists");
    for n in LIST_SIZES {
        let seed = bincode::serialize(&vec![b"item".to_vec(); n]).unwrap();
        group.bench_with_input(BenchmarkId::new("push", n), &n, |b, _| {
            let dir = BenchDir::new();
            let store = dir.open();
            iter_writes_with_setup(
                b,
                &store,
                || {
                    store
                        .set("list".to_string(), seed.clone(), None, false, false)
                        .unwrap()
                },
                || {
                    black_box(
                        store
                            .list_push("list", b"item".to_vec(), None, false, false)
                            .unwrap(),
                    );
                },
            );
        });
        group.bench_with_input(BenchmarkId::new("get", n), &n, |b, _| {
            let dir = BenchDir::new();
            let store = dir.open();
            store
                .set("list".to_string(), seed.clone(), None, false, false)
                .unwrap();
            b.iter(|| black_box(store.list_get("list").unwrap()));
        });
    }
    group.finish();
}

fn bench_scans(c: &mut Criterion) {
    let mut group = c.benchmark_group("scans");
    for n in SCAN_SIZES {
        group.throughput(Throughput::Elements(n as u64));
        let dir = BenchDir::new();
        let store = dir.open();
        fill(&store, n, None);
        group.bench_with_input(BenchmarkId::new("keys", n), &n, |b, _| {
            b.iter(|| black_box(store.keys()));
        });
        group.bench_with_input(BenchmarkId::new("get_all", n), &n, |b, _| {
            b.iter(|| black_box(store.get_all().unwrap()));
        });
        group.bench_with_input(BenchmarkId::new("purge_expired_none", n), &n, |b, _| {
            b.iter(|| black_box(store.purge_expired()));
        });
    }
    group.finish();
}

fn bench_persistence(c: &mut Criterion) {
    let mut group = c.benchmark_group("persistence");
    group
        .sample_size(20)
        .measurement_time(Duration::from_secs(5));
    for n in SCAN_SIZES {
        group.throughput(Throughput::Elements(n as u64));
        let dir = BenchDir::new();
        {
            let store = dir.open();
            fill(&store, n, None);
            fill(&store, n, None);
        }
        group.bench_with_input(BenchmarkId::new("open_replay", n), &n, |b, _| {
            b.iter(|| black_box(dir.open()));
        });
        let store = dir.open();
        group.bench_with_input(BenchmarkId::new("compact", n), &n, |b, _| {
            b.iter(|| store.compact().unwrap());
        });
    }
    group.finish();
}

criterion_group!(
    benches,
    bench_set,
    bench_get,
    bench_writes,
    bench_lists,
    bench_scans,
    bench_persistence
);
criterion_main!(benches);
