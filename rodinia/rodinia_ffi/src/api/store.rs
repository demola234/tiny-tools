use crate::cache::Entry;
use crate::error::StoreError;
use crate::events::StorageEvent;
use crate::frb_generated::StreamSink;
use crate::store::Store;
use std::collections::HashMap;
use std::sync::OnceLock;

static STORE: OnceLock<Store> = OnceLock::new();

fn store() -> Result<&'static Store, StoreError> {
    STORE.get().ok_or(StoreError::NotInitialized)
}

#[flutter_rust_bridge::frb(init)]
pub fn init_app() {
    flutter_rust_bridge::setup_default_user_utils();
}

#[flutter_rust_bridge::frb(sync)]
pub fn store_open(path: String) -> Result<(), StoreError> {
   let opened = Store::open(&path)?;
   STORE
   .set(opened)
   .map_err(|_| StoreError::AlreadyInitialized)
}

#[flutter_rust_bridge::frb(sync)]
pub fn store_is_open() -> bool {
    STORE.get().is_some()
}

#[flutter_rust_bridge::frb(sync)]
pub fn store_set(key: String, value: Vec<u8>, ttl_ms: Option<i64>, encrypted: bool) -> Result<(), StoreError> {
    store()?.set(key, value, ttl_ms, encrypted)
}

#[flutter_rust_bridge::frb(sync)]
pub fn store_get(key: String) -> Result<Option<Vec<u8>>, StoreError> {
    store()?.get(&key)
}

#[flutter_rust_bridge::frb(sync)]
pub fn store_contains(key: String) -> Result<bool, StoreError> {
    Ok(store()?.contains(&key))
}

#[flutter_rust_bridge::frb(sync)]
pub fn store_delete(key: String) -> Result<bool, StoreError> {
    store()?.delete(&key)
}

#[flutter_rust_bridge::frb(sync)]
pub fn store_clear() -> Result<(), StoreError> {
    store()?.clear()
}

#[flutter_rust_bridge::frb(sync)]
pub fn store_keys() -> Result<Vec<String>, StoreError> {
    Ok(store()?.keys())
}

#[flutter_rust_bridge::frb(sync)]
pub fn store_len() -> Result<i64, StoreError> {
    Ok(store()?.len())
}

#[flutter_rust_bridge::frb(sync)]
pub fn store_get_all() -> Result<HashMap<String, Entry>, StoreError> {
    store()?.get_all()
}


#[flutter_rust_bridge::frb(sync)]
pub fn store_set_encryption_key(key: Vec<u8>) -> Result<(), StoreError> {
    store()?.set_encryption_key(key)
}

#[flutter_rust_bridge::frb(sync)]
pub fn store_generate_encryption_key() -> Vec<u8> {
    crate::crypto::EncryptionKey::generate().as_bytes().to_vec()
}

#[flutter_rust_bridge::frb(sync)]
pub fn store_increment(key: String, delta: i64, ttl_ms: Option<i64>) -> Result<i64, StoreError> {
    store()?.increment(&key, delta, ttl_ms)
}

#[flutter_rust_bridge::frb(sync)]
pub fn store_set_if_absent(key: String, value: Vec<u8>, ttl_ms: Option<i64>, encrypted: bool) -> Result<bool, StoreError> {
    store()?.set_if_absent(key, value, ttl_ms, encrypted)
}

#[flutter_rust_bridge::frb(sync)]
pub fn store_purge_expired() -> Result<i64, StoreError> {
    Ok(store()?.purge_expired())
}

#[flutter_rust_bridge::frb(sync)]
pub fn store_compact() -> Result<(), StoreError> {
    store()?.compact()
}

#[flutter_rust_bridge::frb(sync)]
pub fn store_watch(pattern: String, sink: StreamSink<StorageEvent>) -> Result<(), StoreError> {
    store()?.subscribe(pattern, sink);
    Ok(())
}
