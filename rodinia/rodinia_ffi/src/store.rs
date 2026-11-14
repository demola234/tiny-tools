use crate::cache::{now_ms, Entry, IndexEntry, Preload, StoredItem, StoredValue};
use crate::crypto::{self, Cipher, EncryptionKey};
use crate::error::StoreError;
use crate::events::{EventBus, StorageEvent};
use crate::frb_generated::StreamSink;
use crate::record::{Record, RecordKind, MAGIC};
use crate::wal::Wal;
use parking_lot::RwLock;
use std::collections::HashMap;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use zeroize::Zeroizing;

const AUTO_COMPACT_MIN_BYTES: u64 = 1024 * 1024;

struct State {
    wal: Wal,
    index: HashMap<String, IndexEntry>,
    live_bytes: u64,
}

impl State {
    fn compact(&mut self) -> Result<(), StoreError> {
        let locs = self.wal.compact(&self.index)?;
        for (key, loc) in locs {
            if let Some(entry) = self.index.get_mut(&key) {
                entry.loc = loc;
            }
        }
        Ok(())
    }

    fn compact_if_mostly_dead(&mut self) {
        let file_len = self.wal.file_len();
        let dead = file_len.saturating_sub(MAGIC.len() as u64 + self.live_bytes);
        if file_len > AUTO_COMPACT_MIN_BYTES && dead > self.live_bytes {
            let _ = self.compact();
        }
    }

    fn live(&self, key: &str, now: u64) -> Option<IndexEntry> {
        self.index.get(key).filter(|e| !e.is_expired(now)).copied()
    }

    fn read(&self, entry: &IndexEntry) -> Result<Vec<u8>, StoreError> {
        self.wal.read_value(entry.loc)
    }

    fn write_set(
        &mut self,
        key: &str,
        value: &[u8],
        encrypted: bool,
        expires_at: Option<u64>,
        durable: bool,
    ) -> Result<bool, StoreError> {
        let record = Record {
            kind: RecordKind::Set,
            key,
            value,
            encrypted,
            expires_at,
        };
        let (loc, record_len) = self.wal.append(&record, durable)?;
        let entry = IndexEntry {
            loc,
            record_len,
            encrypted,
            expires_at,
        };
        self.live_bytes += record_len as u64;
        let old = self.index.insert(key.to_string(), entry);
        if let Some(old) = old {
            self.live_bytes -= old.record_len as u64;
        }
        self.compact_if_mostly_dead();
        Ok(old.is_some())
    }

    fn write_delete(&mut self, key: &str, durable: bool) -> Result<bool, StoreError> {
        if !self.index.contains_key(key) {
            return Ok(false);
        }
        let record = Record {
            kind: RecordKind::Delete,
            key,
            value: &[],
            encrypted: false,
            expires_at: None,
        };
        self.wal.append(&record, durable)?;
        if let Some(old) = self.index.remove(key) {
            self.live_bytes -= old.record_len as u64;
        }
        self.compact_if_mostly_dead();
        Ok(true)
    }

    fn write_clear(&mut self, durable: bool) -> Result<(), StoreError> {
        let record = Record {
            kind: RecordKind::Clear,
            key: "",
            value: &[],
            encrypted: false,
            expires_at: None,
        };
        self.wal.append(&record, durable)?;
        self.index.clear();
        self.live_bytes = 0;
        self.compact_if_mostly_dead();
        Ok(())
    }
}

struct Keyring {
    key: EncryptionKey,
    cipher: Cipher,
}

pub struct Store {
    state: RwLock<State>,
    keyring: RwLock<Option<Keyring>>,
    generation: Box<AtomicU64>,
    events: EventBus,
}

fn encode_hex(bytes: &[u8]) -> String {
    bytes.iter().map(|b| format!("{b:02x}")).collect()
}

fn decode_hex(s: &str) -> Result<Vec<u8>, StoreError> {
    if !s.len().is_multiple_of(2) {
        return Err(StoreError::InvalidCiphertext);
    }
    (0..s.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&s[i..i + 2], 16).map_err(|_| StoreError::InvalidCiphertext))
        .collect()
}

fn expires_at_from_ttl(ttl_ms: Option<i64>) -> Option<u64> {
    ttl_ms.map(|ms| now_ms().saturating_add(ms.max(0) as u64))
}

fn decode_list(key: &str, bytes: &[u8]) -> Result<Vec<Vec<u8>>, StoreError> {
    bincode::deserialize(bytes)
        .map_err(|_| StoreError::Serialization(format!("value for key '{key}' is not a list")))
}

fn encode_list(list: &[Vec<u8>]) -> Result<Vec<u8>, StoreError> {
    bincode::serialize(list).map_err(|e| StoreError::Serialization(e.to_string()))
}

impl Store {
    pub fn open(dir: &str) -> Result<Self, StoreError> {
        let dir_path = PathBuf::from(dir);
        std::fs::create_dir_all(&dir_path)?;
        let (wal, replay) = Wal::open(&dir_path.join("rodinia.wal"))?;
        Ok(Self {
            state: RwLock::new(State {
                wal,
                index: replay.index,
                live_bytes: replay.live_bytes,
            }),
            keyring: RwLock::new(None),
            generation: Box::new(AtomicU64::new(0)),
            events: EventBus::new(),
        })
    }

    pub fn generation(&self) -> u64 {
        self.generation.load(Ordering::Acquire)
    }

    pub fn generation_ptr(&self) -> *const AtomicU64 {
        &*self.generation
    }

    fn bump_generation(&self) {
        self.generation.fetch_add(1, Ordering::AcqRel);
    }

    pub fn set_encryption_key(&self, key: Vec<u8>) -> Result<(), StoreError> {
        if key.len() != crypto::KEY_LEN {
            return Err(StoreError::InvalidKeyLength(key.len()));
        }
        let key = Zeroizing::new(key);
        let mut bytes = [0u8; crypto::KEY_LEN];
        bytes.copy_from_slice(&key);
        let key = EncryptionKey::from_bytes(bytes);
        let cipher = Cipher::new(&key);
        *self.keyring.write() = Some(Keyring { key, cipher });
        self.bump_generation();
        Ok(())
    }

    pub fn lock(&self) {
        *self.keyring.write() = None;
        self.bump_generation();
    }

    fn with_cipher<T>(
        &self,
        f: impl FnOnce(&Cipher) -> Result<T, StoreError>,
    ) -> Result<T, StoreError> {
        let guard = self.keyring.read();
        let keyring = guard.as_ref().ok_or(StoreError::EncryptionKeyNotSet)?;
        f(&keyring.cipher)
    }

    pub fn decode_value(
        &self,
        key: &str,
        value: Vec<u8>,
        encrypted: bool,
    ) -> Result<Vec<u8>, StoreError> {
        if !encrypted {
            return Ok(value);
        }
        self.with_cipher(|cipher| cipher.decrypt(key.as_bytes(), &value))
    }

    pub fn encode_value(
        &self,
        key: &str,
        value: Vec<u8>,
        encrypted: bool,
    ) -> Result<Vec<u8>, StoreError> {
        if !encrypted {
            return Ok(value);
        }
        self.with_cipher(|cipher| cipher.encrypt(key.as_bytes(), &value))
    }

    pub fn encrypt_key(&self, key: &str) -> Result<String, StoreError> {
        let encrypted_key = self.with_cipher(|cipher| cipher.encrypt(&[], key.as_bytes()))?;
        Ok(encode_hex(&encrypted_key))
    }

    pub fn decrypt_key(&self, encrypted_key: &str) -> Result<String, StoreError> {
        let bytes = decode_hex(encrypted_key)?;
        let decrypted_key = self.with_cipher(|cipher| cipher.decrypt(&[], &bytes))?;
        String::from_utf8(decrypted_key).map_err(|_| StoreError::InvalidCiphertext)
    }

    pub fn get_encryption_key(&self) -> Result<Vec<u8>, StoreError> {
        let guard = self.keyring.read();
        let keyring = guard.as_ref().ok_or(StoreError::EncryptionKeyNotSet)?;
        Ok(keyring.key.as_bytes().to_vec())
    }

    fn publish_write(&self, key: String, existed: bool) {
        self.bump_generation();
        self.events.publish(if existed {
            StorageEvent::KeyUpdated { key }
        } else {
            StorageEvent::KeyCreated { key }
        });
    }

    pub fn set(
        &self,
        key: String,
        value: Vec<u8>,
        ttl_ms: Option<i64>,
        encrypted: bool,
        durable: bool,
    ) -> Result<(), StoreError> {
        let stored = self.encode_value(&key, value, encrypted)?;
        let existed = self.state.write().write_set(
            &key,
            &stored,
            encrypted,
            expires_at_from_ttl(ttl_ms),
            durable,
        )?;
        self.publish_write(key, existed);
        Ok(())
    }

    fn live_entries(&self) -> Result<HashMap<String, Entry>, StoreError> {
        let now = now_ms();
        let state = self.state.read();
        let mut result = HashMap::with_capacity(state.index.len());
        for (key, entry) in &state.index {
            if !entry.is_expired(now) {
                let value = state.read(entry)?;
                result.insert(
                    key.clone(),
                    Entry {
                        value,
                        encrypted: entry.encrypted,
                        expires_at: entry.expires_at,
                    },
                );
            }
        }
        Ok(result)
    }

    pub fn get_all(&self) -> Result<Vec<StoredItem>, StoreError> {
        let raw = self.live_entries()?;
        let keyring = self.keyring.read();
        raw.into_iter()
            .map(|(key, entry)| {
                let value = if entry.encrypted {
                    let keyring = keyring.as_ref().ok_or(StoreError::EncryptionKeyNotSet)?;
                    keyring.cipher.decrypt(key.as_bytes(), &entry.value)?
                } else {
                    entry.value
                };
                Ok(StoredItem {
                    key,
                    value,
                    encrypted: entry.encrypted,
                    expires_at_ms: entry.expires_at.map(|t| t as i64),
                })
            })
            .collect()
    }

    /// Reads live plain values in file order, so the disk is read in one
    /// forward pass, until their total size would pass `limit_bytes`.
    /// Encrypted values are left out. `complete` says whether every live key
    /// was returned. `generation` is read under the same lock as the values;
    /// writers bump it after changing the data, so a caller that finds the
    /// counter still equal to it knows the values are current.
    pub fn preload(&self, limit_bytes: u64) -> Result<Preload, StoreError> {
        let now = now_ms();
        let state = self.state.read();
        let generation = self.generation() as i64;
        let mut complete = true;
        let mut live = Vec::with_capacity(state.index.len());
        for (key, entry) in &state.index {
            if entry.is_expired(now) {
                continue;
            }
            if entry.encrypted {
                complete = false;
                continue;
            }
            live.push((key, entry));
        }
        live.sort_unstable_by_key(|(_, entry)| entry.loc.offset);

        let mut items = Vec::with_capacity(live.len());
        let mut total = 0u64;
        for (key, entry) in live {
            total += u64::from(entry.loc.len);
            if total > limit_bytes {
                complete = false;
                break;
            }
            items.push(StoredItem {
                key: key.clone(),
                value: state.read(entry)?,
                encrypted: false,
                expires_at_ms: entry.expires_at.map(|t| t as i64),
            });
        }
        Ok(Preload {
            generation,
            complete,
            items,
        })
    }

    fn read_live(&self, key: &str) -> Result<Option<(Vec<u8>, IndexEntry)>, StoreError> {
        let state = self.state.read();
        let Some(entry) = state.index.get(key).copied() else {
            return Ok(None);
        };
        if entry.is_expired(now_ms()) {
            drop(state);
            self.expire_key(key);
            return Ok(None);
        }
        Ok(Some((state.read(&entry)?, entry)))
    }

    pub fn get(&self, key: &str) -> Result<Option<Vec<u8>>, StoreError> {
        match self.read_live(key)? {
            Some((bytes, entry)) => self.decode_value(key, bytes, entry.encrypted).map(Some),
            None => Ok(None),
        }
    }

    pub fn get_entry(&self, key: &str) -> Result<Option<StoredValue>, StoreError> {
        let Some((bytes, entry)) = self.read_live(key)? else {
            return Ok(None);
        };
        Ok(Some(StoredValue {
            value: self.decode_value(key, bytes, entry.encrypted)?,
            encrypted: entry.encrypted,
            expires_at_ms: entry.expires_at.map(|t| t as i64),
        }))
    }

    pub fn list_push(
        &self,
        key: &str,
        item: Vec<u8>,
        ttl_ms: Option<i64>,
        encrypted: bool,
        durable: bool,
    ) -> Result<i64, StoreError> {
        let (was_stale, was_live, new_len) = {
            let mut state = self.state.write();
            let existing = state.index.get(key).copied();
            let live = state.live(key, now_ms());

            let mut list = match live {
                Some(entry) => {
                    let bytes = self.decode_value(key, state.read(&entry)?, entry.encrypted)?;
                    decode_list(key, &bytes)?
                }
                None => Vec::new(),
            };
            list.push(item);

            let expires_at = match ttl_ms {
                Some(_) => expires_at_from_ttl(ttl_ms),
                None => live.and_then(|e| e.expires_at),
            };
            let stored = self.encode_value(key, encode_list(&list)?, encrypted)?;
            state.write_set(key, &stored, encrypted, expires_at, durable)?;
            (
                existing.is_some() && live.is_none(),
                live.is_some(),
                list.len() as i64,
            )
        };

        if was_stale {
            self.events.publish(StorageEvent::KeyExpired {
                key: key.to_string(),
            });
        }
        self.publish_write(key.to_string(), was_live);
        Ok(new_len)
    }

    pub fn list_get(&self, key: &str) -> Result<Option<Vec<Vec<u8>>>, StoreError> {
        match self.read_live(key)? {
            Some((bytes, entry)) => {
                let decoded = self.decode_value(key, bytes, entry.encrypted)?;
                bincode::deserialize(&decoded)
                    .map(Some)
                    .map_err(|e| StoreError::Serialization(e.to_string()))
            }
            None => Ok(None),
        }
    }

    pub fn delete_from_list(
        &self,
        key: &str,
        item: Vec<u8>,
        ttl_ms: Option<i64>,
        encrypted: bool,
        durable: bool,
    ) -> Result<bool, StoreError> {
        let event = {
            let mut state = self.state.write();
            let Some(entry) = state.index.get(key).copied() else {
                return Ok(false);
            };

            if entry.is_expired(now_ms()) {
                state.write_delete(key, durable)?;
                Some(StorageEvent::KeyExpired {
                    key: key.to_string(),
                })
            } else {
                let bytes = self.decode_value(key, state.read(&entry)?, entry.encrypted)?;
                let mut list = decode_list(key, &bytes)?;
                let before = list.len();
                list.retain(|i| *i != item);
                if list.len() == before {
                    return Ok(false);
                }

                if list.is_empty() {
                    state.write_delete(key, durable)?;
                    Some(StorageEvent::KeyDeleted {
                        key: key.to_string(),
                    })
                } else {
                    let expires_at = match ttl_ms {
                        Some(_) => expires_at_from_ttl(ttl_ms),
                        None => entry.expires_at,
                    };
                    let stored = self.encode_value(key, encode_list(&list)?, encrypted)?;
                    state.write_set(key, &stored, encrypted, expires_at, durable)?;
                    Some(StorageEvent::KeyUpdated {
                        key: key.to_string(),
                    })
                }
            }
        };

        let removed_expired = matches!(event, Some(StorageEvent::KeyExpired { .. }));
        if let Some(event) = event {
            self.bump_generation();
            self.events.publish(event);
        }
        Ok(!removed_expired)
    }

    fn expire_key(&self, key: &str) {
        let removed = {
            let mut state = self.state.write();
            match state.index.get(key) {
                Some(entry) if entry.is_expired(now_ms()) => {
                    state.write_delete(key, false).unwrap_or(false)
                }
                _ => false,
            }
        };
        if removed {
            self.bump_generation();
            self.events.publish(StorageEvent::KeyExpired {
                key: key.to_string(),
            });
        }
    }

    pub fn contains(&self, key: &str) -> bool {
        let entry = self.state.read().index.get(key).copied();
        match entry {
            Some(e) if e.is_expired(now_ms()) => {
                self.expire_key(key);
                false
            }
            Some(_) => true,
            None => false,
        }
    }

    pub fn delete(&self, key: &str, durable: bool) -> Result<bool, StoreError> {
        let existed = self.state.write().write_delete(key, durable)?;
        if existed {
            self.bump_generation();
            self.events.publish(StorageEvent::KeyDeleted {
                key: key.to_string(),
            });
        }
        Ok(existed)
    }

    pub fn clear(&self, durable: bool) -> Result<(), StoreError> {
        self.state.write().write_clear(durable)?;
        self.bump_generation();
        self.events.publish(StorageEvent::StorageCleared);
        Ok(())
    }

    pub fn keys(&self) -> Vec<String> {
        let now = now_ms();
        self.state
            .read()
            .index
            .iter()
            .filter(|(_, e)| !e.is_expired(now))
            .map(|(k, _)| k.clone())
            .collect()
    }

    pub fn export(&self) -> Result<HashMap<String, Entry>, StoreError> {
        self.live_entries()
    }

    pub fn import(&self, data: HashMap<String, Entry>) -> Result<(), StoreError> {
        let mut state = self.state.write();
        for (key, entry) in data {
            state.write_set(&key, &entry.value, entry.encrypted, entry.expires_at, false)?;
        }
        drop(state);
        self.bump_generation();
        Ok(())
    }

    pub fn len(&self) -> i64 {
        let now = now_ms();
        self.state
            .read()
            .index
            .values()
            .filter(|e| !e.is_expired(now))
            .count() as i64
    }

    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }

    pub fn close(&self) -> Result<(), StoreError> {
        self.state.write().wal.close()
    }

    pub fn drop(&self) -> Result<(), StoreError> {
        self.state.write().wal.delete()
    }

    pub fn increment(
        &self,
        key: &str,
        delta: i64,
        ttl_ms: Option<i64>,
        durable: bool,
    ) -> Result<i64, StoreError> {
        let (next, existed) = {
            let mut state = self.state.write();
            let current = match state.live(key, now_ms()) {
                Some(entry) => {
                    let bytes = state.read(&entry)?;
                    std::str::from_utf8(&bytes)
                        .ok()
                        .and_then(|text| text.parse::<i64>().ok())
                        .ok_or_else(|| StoreError::InvalidCounterValue(key.to_string()))?
                }
                None => 0,
            };
            let next = current.saturating_add(delta);
            let existed = state.write_set(
                key,
                next.to_string().as_bytes(),
                false,
                expires_at_from_ttl(ttl_ms),
                durable,
            )?;
            (next, existed)
        };
        self.publish_write(key.to_string(), existed);
        Ok(next)
    }

    pub fn set_if_absent(
        &self,
        key: String,
        value: Vec<u8>,
        ttl_ms: Option<i64>,
        encrypted: bool,
        durable: bool,
    ) -> Result<bool, StoreError> {
        {
            let mut state = self.state.write();
            if state.live(&key, now_ms()).is_some() {
                return Ok(false);
            }
            let stored = self.encode_value(&key, value, encrypted)?;
            state.write_set(
                &key,
                &stored,
                encrypted,
                expires_at_from_ttl(ttl_ms),
                durable,
            )?;
        }
        self.bump_generation();
        self.events.publish(StorageEvent::KeyCreated { key });
        Ok(true)
    }

    pub fn purge_expired(&self) -> i64 {
        let now = now_ms();
        let expired_keys: Vec<String> = self
            .state
            .read()
            .index
            .iter()
            .filter(|(_, e)| e.is_expired(now))
            .map(|(k, _)| k.clone())
            .collect();
        for key in &expired_keys {
            self.expire_key(key);
        }
        expired_keys.len() as i64
    }

    pub fn compact(&self) -> Result<(), StoreError> {
        self.purge_expired();
        self.state.write().compact()
    }

    pub fn subscribe(&self, pattern: String, sink: StreamSink<StorageEvent>) {
        self.events.subscribe(pattern, sink);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::atomic::{AtomicUsize, Ordering};

    struct TestDir(String);

    impl TestDir {
        fn new() -> Self {
            static COUNTER: AtomicUsize = AtomicUsize::new(0);
            let n = COUNTER.fetch_add(1, Ordering::Relaxed);
            Self(
                std::env::temp_dir()
                    .join(format!("rodinia_test_{}_{}", std::process::id(), n))
                    .to_string_lossy()
                    .into_owned(),
            )
        }

        fn open(&self) -> Store {
            Store::open(&self.0).unwrap()
        }
    }

    impl Drop for TestDir {
        fn drop(&mut self) {
            let _ = std::fs::remove_dir_all(&self.0);
        }
    }

    fn put(store: &Store, key: &str, value: &[u8]) {
        store
            .set(key.to_string(), value.to_vec(), None, false, false)
            .unwrap();
    }

    #[test]
    fn test_open() {
        let dir = TestDir::new();
        let store = dir.open();
        assert!(store.state.read().index.is_empty());
        assert!(store.state.read().wal.path().exists());
    }

    #[test]
    fn test_directory_not_overwritten() {
        let dir = TestDir::new();
        let path1 = dir.open().state.read().wal.path().to_path_buf();
        let path2 = dir.open().state.read().wal.path().to_path_buf();
        assert_eq!(path1, path2);
    }

    #[test]
    fn test_cache_is_populated_from_wal_on_open() {
        let dir = TestDir::new();
        put(&dir.open(), "key1", b"value1");
        assert_eq!(dir.open().get("key1").unwrap(), Some(b"value1".to_vec()));
    }

    #[test]
    fn test_set_and_get() {
        let dir = TestDir::new();
        let store = dir.open();
        put(&store, "key1", b"value1");
        assert_eq!(store.get("key1").unwrap(), Some(b"value1".to_vec()));
        assert_eq!(store.get("missing").unwrap(), None);
    }

    #[test]
    fn test_contains() {
        let dir = TestDir::new();
        let store = dir.open();
        put(&store, "key1", b"value1");
        assert!(store.contains("key1"));
        assert!(!store.contains("key2"));
    }

    #[test]
    fn test_delete() {
        let dir = TestDir::new();
        let store = dir.open();
        put(&store, "key1", b"value1");
        assert!(store.delete("key1", false).unwrap());
        assert!(!store.delete("key1", false).unwrap());
        assert!(!store.contains("key1"));
    }

    #[test]
    fn test_clear() {
        let dir = TestDir::new();
        let store = dir.open();
        put(&store, "key1", b"value1");
        put(&store, "key2", b"value2");
        store.clear(false).unwrap();
        assert!(store.state.read().index.is_empty());
        assert!(dir.open().is_empty());
    }

    #[test]
    fn test_keys() {
        let dir = TestDir::new();
        let store = dir.open();
        put(&store, "key1", b"value1");
        put(&store, "key2", b"value2");
        let keys = store.keys();
        assert_eq!(keys.len(), 2);
        assert!(keys.contains(&"key1".to_string()));
        assert!(keys.contains(&"key2".to_string()));
    }

    #[test]
    fn test_export_import() {
        let dir = TestDir::new();
        let store = dir.open();
        put(&store, "key1", b"value1");
        put(&store, "key2", b"value2");
        let exported = store.export().unwrap();
        let other = TestDir::new();
        let store2 = other.open();
        store2.import(exported).unwrap();
        assert_eq!(store2.get("key1").unwrap(), Some(b"value1".to_vec()));
        assert_eq!(store2.get("key2").unwrap(), Some(b"value2".to_vec()));
    }

    #[test]
    fn test_len() {
        let dir = TestDir::new();
        let store = dir.open();
        put(&store, "key1", b"value1");
        put(&store, "key2", b"value2");
        assert_eq!(store.len(), 2);
    }

    #[test]
    fn test_encrypt_decrypt_key_roundtrip() {
        let dir = TestDir::new();
        let store = dir.open();
        store
            .set_encryption_key(vec![7u8; crypto::KEY_LEN])
            .unwrap();
        let encrypted = store.encrypt_key("secret-key").unwrap();
        assert_eq!(store.decrypt_key(&encrypted).unwrap(), "secret-key");
    }

    #[test]
    fn test_encrypted_value_survives_reopen() {
        let dir = TestDir::new();
        {
            let store = dir.open();
            store
                .set_encryption_key(vec![7u8; crypto::KEY_LEN])
                .unwrap();
            store
                .set("secret".into(), b"value".to_vec(), None, true, false)
                .unwrap();
        }
        let store = dir.open();
        assert!(matches!(
            store.get("secret"),
            Err(StoreError::EncryptionKeyNotSet)
        ));
        store
            .set_encryption_key(vec![7u8; crypto::KEY_LEN])
            .unwrap();
        assert_eq!(store.get("secret").unwrap(), Some(b"value".to_vec()));
    }

    #[test]
    fn test_close_drop() {
        let dir = TestDir::new();
        let store = dir.open();
        store.close().unwrap();
        store.drop().unwrap();
    }

    #[test]
    fn test_increment() {
        let dir = TestDir::new();
        let store = dir.open();
        assert_eq!(store.increment("counter1", 1, None, false).unwrap(), 1);
        assert_eq!(store.increment("counter1", 5, None, false).unwrap(), 6);
        assert_eq!(store.get("counter1").unwrap(), Some(b"6".to_vec()));
        put(&store, "text", b"abc");
        assert!(matches!(
            store.increment("text", 1, None, false),
            Err(StoreError::InvalidCounterValue(_))
        ));
    }

    #[test]
    fn test_set_if_absent() {
        let dir = TestDir::new();
        let store = dir.open();
        assert!(store
            .set_if_absent("key1".into(), b"value1".to_vec(), None, false, false)
            .unwrap());
        assert!(!store
            .set_if_absent("key1".into(), b"value2".to_vec(), None, false, false)
            .unwrap());
        assert_eq!(store.get("key1").unwrap(), Some(b"value1".to_vec()));
    }

    #[test]
    fn test_purge_expired() {
        let dir = TestDir::new();
        let store = dir.open();
        store
            .set("key1".into(), b"value1".to_vec(), Some(-1), false, false)
            .unwrap();
        assert_eq!(store.purge_expired(), 1);
        assert!(!store.contains("key1"));
    }

    #[test]
    fn test_expired_get_returns_none() {
        let dir = TestDir::new();
        let store = dir.open();
        store
            .set("key1".into(), b"value1".to_vec(), Some(-1), false, false)
            .unwrap();
        assert_eq!(store.get("key1").unwrap(), None);
        assert!(store.state.read().index.is_empty());
    }

    #[test]
    fn test_compact() {
        let dir = TestDir::new();
        let store = dir.open();
        for i in 0..50 {
            put(&store, "key1", format!("value{i}").as_bytes());
        }
        let before = store.state.read().wal.file_len();
        store.compact().unwrap();
        assert!(store.state.read().wal.file_len() < before);
        assert_eq!(store.get("key1").unwrap(), Some(b"value49".to_vec()));
        put(&store, "key2", b"after");
        let reopened = dir.open();
        assert_eq!(reopened.get("key1").unwrap(), Some(b"value49".to_vec()));
        assert_eq!(reopened.get("key2").unwrap(), Some(b"after".to_vec()));
    }

    #[test]
    fn test_add_to_list() {
        let dir = TestDir::new();
        let store = dir.open();
        assert_eq!(
            store
                .list_push("list1", b"value1".to_vec(), None, false, false)
                .unwrap(),
            1
        );
        assert_eq!(
            store
                .list_push("list1", b"value2".to_vec(), None, false, false)
                .unwrap(),
            2
        );
        assert_eq!(
            dir.open().list_get("list1").unwrap(),
            Some(vec![b"value1".to_vec(), b"value2".to_vec()])
        );
    }

    #[test]
    fn test_delete_from_list() {
        let dir = TestDir::new();
        let store = dir.open();
        store
            .list_push("list1", b"a".to_vec(), None, false, false)
            .unwrap();
        store
            .list_push("list1", b"b".to_vec(), None, false, false)
            .unwrap();
        assert!(store
            .delete_from_list("list1", b"a".to_vec(), None, false, false)
            .unwrap());
        assert!(!store
            .delete_from_list("list1", b"zzz".to_vec(), None, false, false)
            .unwrap());
        assert_eq!(store.list_get("list1").unwrap(), Some(vec![b"b".to_vec()]));
        assert!(store
            .delete_from_list("list1", b"b".to_vec(), None, false, false)
            .unwrap());
        assert_eq!(store.list_get("list1").unwrap(), None);
    }

    #[test]
    fn test_list_get_missing() {
        let dir = TestDir::new();
        assert_eq!(dir.open().list_get("nope").unwrap(), None);
    }

    #[test]
    fn index_holds_no_values() {
        let dir = TestDir::new();
        let store = dir.open();
        assert!(std::mem::size_of::<IndexEntry>() < 64);
        let big = vec![0xABu8; 1024];
        put(&store, "big", &big);
        assert_eq!(store.get("big").unwrap(), Some(big));
    }

    #[test]
    fn reopen_reads_values_from_disk() {
        let dir = TestDir::new();
        let long_key = "k".repeat(65_536);
        {
            let store = dir.open();
            put(&store, "plain", b"1");
            put(&store, "ключ🔑", b"2");
            put(&store, &long_key, b"3");
            put(&store, "", b"4");
        }
        let store = dir.open();
        assert_eq!(store.get("plain").unwrap(), Some(b"1".to_vec()));
        assert_eq!(store.get("ключ🔑").unwrap(), Some(b"2".to_vec()));
        assert_eq!(store.get(&long_key).unwrap(), Some(b"3".to_vec()));
        assert_eq!(store.get("").unwrap(), Some(b"4".to_vec()));
    }

    #[test]
    fn durable_set_survives_reopen() {
        let dir = TestDir::new();
        dir.open()
            .set("k".into(), b"safe".to_vec(), None, false, true)
            .unwrap();
        assert_eq!(dir.open().get("k").unwrap(), Some(b"safe".to_vec()));
    }

    #[test]
    fn live_bytes_tracks_index() {
        let dir = TestDir::new();
        let store = dir.open();
        put(&store, "k", b"first");
        put(&store, "k", b"second");
        put(&store, "other", b"x");
        store.delete("other", false).unwrap();
        let state = store.state.read();
        assert_eq!(state.live_bytes, state.index["k"].record_len as u64);
        drop(state);
        assert_eq!(
            dir.open().state.read().live_bytes,
            store.state.read().live_bytes
        );
    }

    fn by_key(items: Vec<StoredItem>) -> HashMap<String, StoredItem> {
        items.into_iter().map(|i| (i.key.clone(), i)).collect()
    }

    fn encrypted_store(dir: &TestDir) -> Store {
        let store = dir.open();
        store
            .set_encryption_key(vec![7u8; crypto::KEY_LEN])
            .unwrap();
        store
    }

    #[test]
    fn get_all_returns_only_live_entries() {
        let dir = TestDir::new();
        let store = dir.open();
        put(&store, "a", b"1");
        store
            .set("old".into(), b"x".to_vec(), Some(-1), false, false)
            .unwrap();
        let all = by_key(store.get_all().unwrap());
        assert_eq!(all.len(), 1);
        assert_eq!(all["a"].value, b"1");
    }

    #[test]
    fn get_all_returns_decrypted_values() {
        let dir = TestDir::new();
        let store = encrypted_store(&dir);
        store
            .set(
                "secret".into(),
                b"s3cret".to_vec(),
                Some(60_000),
                true,
                false,
            )
            .unwrap();
        put(&store, "plain", b"p");
        let all = by_key(store.get_all().unwrap());
        assert_eq!(all["secret"].value, b"s3cret");
        assert!(all["secret"].encrypted);
        assert!(all["secret"].expires_at_ms.is_some());
        assert_eq!(all["plain"].value, b"p");
        assert!(!all["plain"].encrypted);
    }

    #[test]
    fn get_all_without_key_errors() {
        let dir = TestDir::new();
        let store = encrypted_store(&dir);
        store
            .set("secret".into(), b"s".to_vec(), None, true, false)
            .unwrap();
        store.lock();
        assert!(matches!(
            store.get_all(),
            Err(StoreError::EncryptionKeyNotSet)
        ));
    }

    #[test]
    fn get_entry_reports_ttl_and_encrypted() {
        let dir = TestDir::new();
        let store = encrypted_store(&dir);
        store
            .set("secret".into(), b"s".to_vec(), Some(60_000), true, false)
            .unwrap();
        let entry = store.get_entry("secret").unwrap().unwrap();
        assert_eq!(entry.value, b"s");
        assert!(entry.encrypted);
        assert!(entry.expires_at_ms.unwrap() > now_ms() as i64);
        assert!(store.get_entry("missing").unwrap().is_none());
        store
            .set("old".into(), b"x".to_vec(), Some(-1), false, false)
            .unwrap();
        assert!(store.get_entry("old").unwrap().is_none());
    }

    #[test]
    fn lock_forgets_key_keeps_data() {
        let dir = TestDir::new();
        let store = encrypted_store(&dir);
        store
            .set("secret".into(), b"s".to_vec(), None, true, false)
            .unwrap();
        put(&store, "plain", b"p");
        store.lock();
        assert!(matches!(
            store.get("secret"),
            Err(StoreError::EncryptionKeyNotSet)
        ));
        assert_eq!(store.get("plain").unwrap(), Some(b"p".to_vec()));
        store
            .set_encryption_key(vec![7u8; crypto::KEY_LEN])
            .unwrap();
        assert_eq!(store.get("secret").unwrap(), Some(b"s".to_vec()));
    }

    #[test]
    fn wrong_key_fails_authentication() {
        let dir = TestDir::new();
        let store = encrypted_store(&dir);
        store
            .set("secret".into(), b"s".to_vec(), None, true, false)
            .unwrap();
        store
            .set_encryption_key(vec![8u8; crypto::KEY_LEN])
            .unwrap();
        assert!(store.get("secret").is_err());
    }

    #[test]
    fn generation_increments_once_per_change() {
        let dir = TestDir::new();
        let store = dir.open();
        let mut expected = store.generation();
        let mut step = |label: &str, store: &Store| {
            expected += 1;
            assert_eq!(store.generation(), expected, "{label}");
        };
        put(&store, "a", b"1");
        step("set new", &store);
        put(&store, "a", b"2");
        step("set overwrite", &store);
        store.increment("n", 1, None, false).unwrap();
        step("increment", &store);
        store
            .set_if_absent("b".into(), b"1".to_vec(), None, false, false)
            .unwrap();
        step("set_if_absent hit", &store);
        store
            .list_push("l", b"x".to_vec(), None, false, false)
            .unwrap();
        step("list_push", &store);
        store
            .delete_from_list("l", b"x".to_vec(), None, false, false)
            .unwrap();
        step("delete_from_list", &store);
        store.delete("a", false).unwrap();
        step("delete", &store);
        store
            .set("t".into(), b"x".to_vec(), Some(-1), false, false)
            .unwrap();
        step("set expired", &store);
        assert_eq!(store.get("t").unwrap(), None);
        step("expire on read", &store);
        store
            .set_encryption_key(vec![7u8; crypto::KEY_LEN])
            .unwrap();
        step("set key", &store);
        store.lock();
        step("lock", &store);
        let mut data = HashMap::new();
        data.insert(
            "imported".to_string(),
            Entry {
                value: b"i".to_vec(),
                encrypted: false,
                expires_at: None,
            },
        );
        store.import(data).unwrap();
        step("import", &store);
        store.clear(false).unwrap();
        step("clear", &store);
    }

    #[test]
    fn generation_unchanged_by_noops_and_compaction() {
        let dir = TestDir::new();
        let store = dir.open();
        put(&store, "a", b"1");
        store
            .list_push("l", b"x".to_vec(), None, false, false)
            .unwrap();
        let before = store.generation();
        store.delete("missing", false).unwrap();
        store
            .set_if_absent("a".into(), b"2".to_vec(), None, false, false)
            .unwrap();
        store
            .delete_from_list("l", b"absent".to_vec(), None, false, false)
            .unwrap();
        store.compact().unwrap();
        assert_eq!(store.get("a").unwrap(), Some(b"1".to_vec()));
        assert!(store.contains("a"));
        assert_eq!(store.generation(), before);
    }

    #[test]
    fn generation_ptr_tracks_generation() {
        let dir = TestDir::new();
        let store = dir.open();
        let ptr = store.generation_ptr();
        put(&store, "a", b"1");
        let seen = unsafe { (*ptr).load(std::sync::atomic::Ordering::Acquire) };
        assert_eq!(seen, store.generation());
    }

    #[test]
    fn auto_compaction_shrinks_file_and_keeps_values() {
        let dir = TestDir::new();
        let store = dir.open();
        let mut written = 0u64;
        for round in 0..200 {
            for k in 0..10 {
                let value = vec![round as u8; 1024];
                put(&store, &format!("k{k}"), &value);
                written += 1024;
            }
        }
        let file_len = store.state.read().wal.file_len();
        assert!(
            file_len < written / 2,
            "file {file_len} vs written {written}"
        );
        assert!(file_len <= AUTO_COMPACT_MIN_BYTES + 2048);
        for k in 0..10 {
            assert_eq!(
                store.get(&format!("k{k}")).unwrap(),
                Some(vec![199u8; 1024])
            );
        }
        let reopened = dir.open();
        assert_eq!(reopened.get("k0").unwrap(), Some(vec![199u8; 1024]));
    }

    #[test]
    fn small_file_is_not_auto_compacted() {
        let dir = TestDir::new();
        let store = dir.open();
        let mut last = store.state.read().wal.file_len();
        for _ in 0..500 {
            put(&store, "k", &[1u8; 1024]);
            let now = store.state.read().wal.file_len();
            assert!(now > last);
            last = now;
        }
    }

    #[test]
    fn compaction_preserves_encrypted_and_ttl_entries() {
        let dir = TestDir::new();
        {
            let store = dir.open();
            store
                .set_encryption_key(vec![7u8; crypto::KEY_LEN])
                .unwrap();
            store
                .set("secret".into(), b"s".to_vec(), None, true, false)
                .unwrap();
            store
                .set("ttl".into(), b"t".to_vec(), Some(3_600_000), false, false)
                .unwrap();
            put(&store, "secret", b"overwritten-then-restored");
            store
                .set("secret".into(), b"s".to_vec(), None, true, false)
                .unwrap();
            store.compact().unwrap();
            assert_eq!(store.get("secret").unwrap(), Some(b"s".to_vec()));
        }
        let store = dir.open();
        store
            .set_encryption_key(vec![7u8; crypto::KEY_LEN])
            .unwrap();
        let state = store.state.read();
        assert!(state.index["secret"].encrypted);
        assert!(state.index["ttl"].expires_at.is_some());
        drop(state);
        assert_eq!(store.get("secret").unwrap(), Some(b"s".to_vec()));
        assert_eq!(store.get("ttl").unwrap(), Some(b"t".to_vec()));
    }

    fn keys_of(preload: &Preload) -> Vec<&str> {
        preload.items.iter().map(|i| i.key.as_str()).collect()
    }

    #[test]
    fn test_preload_returns_live_plain_values_in_file_order() {
        let dir = TestDir::new();
        let store = dir.open();
        put(&store, "b", b"2");
        put(&store, "a", b"1");
        store
            .set("ttl".into(), b"t".to_vec(), Some(60_000), false, false)
            .unwrap();
        put(&store, "b", b"22");
        let preload = store.preload(u64::MAX).unwrap();
        assert_eq!(keys_of(&preload), ["a", "ttl", "b"]);
        assert_eq!(preload.items[0].value, b"1");
        assert_eq!(preload.items[2].value, b"22");
        assert!(preload.items[1].expires_at_ms.is_some());
        assert!(preload.items.iter().all(|i| !i.encrypted));
        assert!(preload.complete);
        assert_eq!(preload.generation, store.generation() as i64);
    }

    #[test]
    fn test_preload_skips_expired_and_encrypted() {
        let dir = TestDir::new();
        let store = dir.open();
        store
            .set_encryption_key(vec![7u8; crypto::KEY_LEN])
            .unwrap();
        put(&store, "plain", b"p");
        store
            .set("secret".into(), b"s".to_vec(), None, true, false)
            .unwrap();
        store
            .set("gone".into(), b"g".to_vec(), Some(-1), false, false)
            .unwrap();
        let preload = store.preload(u64::MAX).unwrap();
        assert_eq!(keys_of(&preload), ["plain"]);
        assert!(!preload.complete);
    }

    #[test]
    fn test_preload_without_encrypted_or_limit_is_complete_after_expiry() {
        let dir = TestDir::new();
        let store = dir.open();
        put(&store, "plain", b"p");
        store
            .set("gone".into(), b"g".to_vec(), Some(-1), false, false)
            .unwrap();
        assert!(store.preload(u64::MAX).unwrap().complete);
    }

    #[test]
    fn test_preload_stops_at_the_byte_limit() {
        let dir = TestDir::new();
        let store = dir.open();
        put(&store, "a", &[1; 10]);
        put(&store, "b", &[2; 10]);
        put(&store, "c", &[3; 10]);
        let preload = store.preload(25).unwrap();
        assert_eq!(keys_of(&preload), ["a", "b"]);
        assert!(!preload.complete);
        assert!(store.preload(30).unwrap().complete);
    }

    #[test]
    fn test_preload_of_empty_store_is_complete() {
        let dir = TestDir::new();
        let preload = dir.open().preload(0).unwrap();
        assert!(preload.items.is_empty());
        assert!(preload.complete);
    }
}
