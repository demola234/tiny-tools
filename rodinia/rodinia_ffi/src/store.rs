use crate::cache::{now_ms, Entry};
use crate::crypto::{self, EncryptionKey};
use crate::error::StoreError;
use crate::events::{EventBus, StorageEvent};
use crate::frb_generated::StreamSink;
use crate::wal::{Wal, WalRecord};
use parking_lot::{Mutex, RwLock};
use std::collections::HashMap;
use std::path::PathBuf;
use zeroize::Zeroizing;

pub struct Store {
    cache: RwLock<HashMap<String, Entry>>,
    wal: Mutex<Wal>,
    wal_path: PathBuf,
    encryption_key: RwLock<Option<EncryptionKey>>,
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

impl Store {
    pub fn open(dir: &str) -> Result<Self, StoreError> {
        // Get File Path passed from flutter and convert it to a PathBuf
        let dir_path = PathBuf::from(dir);
        std::fs::create_dir_all(&dir_path)?;

        // create path for wal file
        let wal_path = dir_path.join("rodinia.wal");
        let records = Wal::replay(&wal_path)?;

        let mut cache = HashMap::new();
        for record in records {
            match record {
                WalRecord::Set { key, entry } => {
                    cache.insert(key, entry);
                }
                WalRecord::Delete { key } => {
                    cache.remove(&key);
                }
                WalRecord::Clear => cache.clear(),
            }
        }
        let now = now_ms();
        cache.retain(|_, entry| !entry.is_expired(now));

        let wal = Wal::open(&wal_path)?;

        Ok(Self {
            cache: RwLock::new(cache),
            wal: Mutex::new(wal),
            wal_path,
            encryption_key: RwLock::new(None),
            events: EventBus::new(),
        })
    }

    pub fn set_encryption_key(&self, key: Vec<u8>) -> Result<(), StoreError> {
        if key.len() != crypto::KEY_LEN {
            return Err(StoreError::InvalidKeyLength(key.len()));
        }
        let key = Zeroizing::new(key);
        let mut bytes = [0u8; crypto::KEY_LEN];
        bytes.copy_from_slice(&key);
        *self.encryption_key.write() = Some(EncryptionKey::from_bytes(bytes));
        Ok(())
    }

    pub fn decode_value(&self, key: &str, entry: &Entry) -> Result<Vec<u8>, StoreError> {
        if !entry.encrypted {
            return Ok(entry.value.clone());
        }
        let guard = self.encryption_key.read();
        let enc_key = guard.as_ref().ok_or(StoreError::EncryptionKeyNotSet)?;
        crypto::decrypt(enc_key, key.as_bytes(), &entry.value)
    }

    pub fn encode_value(&self, key: &str, value: Vec<u8>, encrypted: bool) -> Result<Vec<u8>, StoreError> {
        if !encrypted {
            return Ok(value);
        }
        let guard = self.encryption_key.read();
        let enc_key = guard.as_ref().ok_or(StoreError::EncryptionKeyNotSet)?;
        crypto::encrypt(enc_key, key.as_bytes(), &value)
    }

    pub fn encrypt_key(&self, key: &str) -> Result<String, StoreError> {
        let guard = self.encryption_key.read();
        let enc_key = guard.as_ref().ok_or(StoreError::EncryptionKeyNotSet)?;
        let encrypted_key = crypto::encrypt(enc_key, &[], key.as_bytes())?;
        Ok(encode_hex(&encrypted_key))
    }

    pub fn decrypt_key(&self, encrypted_key: &str) -> Result<String, StoreError> {
        let guard = self.encryption_key.read();
        let enc_key = guard.as_ref().ok_or(StoreError::EncryptionKeyNotSet)?;
        let bytes = decode_hex(encrypted_key)?;
        let decrypted_key = crypto::decrypt(enc_key, &[], &bytes)?;
        String::from_utf8(decrypted_key).map_err(|_| StoreError::InvalidCiphertext)
    }

    pub fn get_encryption_key(&self) -> Result<Vec<u8>, StoreError> {
        let guard = self.encryption_key.read();
        let key = guard.as_ref().ok_or(StoreError::EncryptionKeyNotSet)?;
        Ok(key.as_bytes().to_vec())
    }


    pub fn set(
        &self, key: String, 
        value: Vec<u8>, 
        ttl_ms: Option<i64>, 
        encrypted: bool
    ) -> Result<(), StoreError> {
        let stored_value = self.encode_value(&key, value, encrypted)?;

        let entry = Entry {
            value: stored_value,
            encrypted,
            expires_at: expires_at_from_ttl(ttl_ms),
        };

        let mut cache = self.cache.write();
        let mut wal = self.wal.lock();
        wal.append(&WalRecord::Set {
            key: key.clone(),
            entry: entry.clone(),
        })?;
        let existed = cache.insert(key.clone(), entry).is_some();
        drop(wal);
        drop(cache);

        self.events.publish(if existed {
            StorageEvent::KeyUpdated { key }
        } else {
            StorageEvent::KeyCreated { key }
        });
        Ok(())
    }

    pub fn get_all(&self) -> Result<HashMap<String, Entry>, StoreError> {
        let mut result = HashMap::new();

        let guard = self.cache.read();
        for (key, entry) in guard.iter() {
            if !entry.is_expired(now_ms()) {
                result.insert(key.clone(), entry.clone());
            }
        }
        drop(guard);

        Ok(result)
    }

    pub fn get(&self, key: &str) -> Result<Option<Vec<u8>>, StoreError> {
        let entry = {
            let cache = self.cache.read();
            cache.get(key).cloned()
        };

        let Some(entry) = entry else {
            return Ok(None);
        };
        if entry.is_expired(now_ms()) {
            self.expire_key(key);
            return Ok(None);
        }
        self.decode_value(key, &entry).map(Some)
    }


    pub fn list_push(
        &self,
        key: &str,
        item: Vec<u8>,
        ttl_ms: Option<i64>,
        encrypted: bool,
    ) -> Result<i64, StoreError> {
        let mut cache = self.cache.write();
        let mut wal = self.wal.lock();

        let now = now_ms();
        let existing = cache.get(key).cloned();
        let live = existing.as_ref().filter(|e| !e.is_expired(now));

        let mut list: Vec<Vec<u8>> = match live {
            Some(e) => {
                let decoded = self.decode_value(key, e)?;
                bincode::deserialize(&decoded).map_err(|_| {
                    StoreError::Serialization(format!("value for key '{key}' is not a list"))
                })?
            }
            None => Vec::new(),
        };
        list.push(item);
        let new_len = list.len() as i64;

        let expires_at = match ttl_ms {
            Some(_) => expires_at_from_ttl(ttl_ms),
            None => live.and_then(|e| e.expires_at),
        };

        let serialized =
            bincode::serialize(&list).map_err(|e| StoreError::Serialization(e.to_string()))?;
        let stored_value = self.encode_value(key, serialized, encrypted)?;

        let entry = Entry {
            value: stored_value,
            encrypted,
            expires_at,
        };
        wal.append(&WalRecord::Set {
            key: key.to_string(),
            entry: entry.clone(),
        })?;
        cache.insert(key.to_string(), entry);
        drop(wal);
        drop(cache);

        let was_stale = existing.is_some() && live.is_none();
        if was_stale {
            self.events.publish(StorageEvent::KeyExpired {
                key: key.to_string(),
            });
        }
        self.events.publish(if live.is_some() {
            StorageEvent::KeyUpdated {
                key: key.to_string(),
            }
        } else {
            StorageEvent::KeyCreated {
                key: key.to_string(),
            }
        });
        Ok(new_len)
    }

    pub fn list_get(&self, key: &str) -> Result<Option<Vec<Vec<u8>>>, StoreError> {
        let cache = self.cache.read();
        let entry = cache.get(key).cloned();
        drop(cache);

        let Some(entry) = entry else {
            return Ok(None);
        };
        if entry.is_expired(now_ms()) {
            self.expire_key(key);
            return Ok(None);
        }
        let decoded = self.decode_value(key, &entry)?;
        bincode::deserialize(&decoded)
            .map(Some)
            .map_err(|e| StoreError::Serialization(e.to_string()))
    }


    pub fn delete_from_list(
        &self,
        key: &str,
        item: Vec<u8>,
        ttl_ms: Option<i64>,
        encrypted: bool,
    ) -> Result<bool, StoreError> {
        let mut cache = self.cache.write();
        let mut wal = self.wal.lock();

        let now = now_ms();
        let Some(entry) = cache.get(key).cloned() else {
            return Ok(false);
        };

        if entry.is_expired(now) {
            cache.remove(key);
            wal.append(&WalRecord::Delete {
                key: key.to_string(),
            })?;
            drop(wal);
            drop(cache);
            self.events.publish(StorageEvent::KeyExpired {
                key: key.to_string(),
            });
            return Ok(false);
        }

        let decoded = self.decode_value(key, &entry)?;
        let mut list: Vec<Vec<u8>> = bincode::deserialize(&decoded).map_err(|_| {
            StoreError::Serialization(format!("value for key '{key}' is not a list"))
        })?;

        let before = list.len();
        list.retain(|i| *i != item);
        if list.len() == before {
            return Ok(false);
        }

        if list.is_empty() {
            cache.remove(key);
            wal.append(&WalRecord::Delete {
                key: key.to_string(),
            })?;
            drop(wal);
            drop(cache);
            self.events.publish(StorageEvent::KeyDeleted {
                key: key.to_string(),
            });
            return Ok(true);
        }

        let expires_at = match ttl_ms {
            Some(_) => expires_at_from_ttl(ttl_ms),
            None => entry.expires_at,
        };

        let serialized =
            bincode::serialize(&list).map_err(|e| StoreError::Serialization(e.to_string()))?;
        let stored_value = self.encode_value(key, serialized, encrypted)?;
        let new_entry = Entry {
            value: stored_value,
            encrypted,
            expires_at,
        };
        wal.append(&WalRecord::Set {
            key: key.to_string(),
            entry: new_entry.clone(),
        })?;
        cache.insert(key.to_string(), new_entry);
        drop(wal);
        drop(cache);

        self.events.publish(StorageEvent::KeyUpdated {
            key: key.to_string(),
        });
        Ok(true)
    }


    fn expire_key(&self, key: &str) {
        let removed = {
            let mut cache = self.cache.write();
            cache.remove(key).is_some()
        };
        if removed {
            let mut wal = self.wal.lock();
            let _ = wal.append(&WalRecord::Delete {
                key: key.to_string(),
            });
            drop(wal);
            self.events.publish(StorageEvent::KeyExpired {
                key: key.to_string(),
            });
        }
    }

    pub fn contains(&self, key: &str) -> bool {
        let entry = {
            let cache = self.cache.read();
            cache.get(key).cloned()
        };

        match entry {
            Some(e) if e.is_expired(now_ms()) => {
                self.expire_key(key);
                false
            }
            Some(_) => true,
            None => false,
        }
    }

    pub fn delete(&self, key: &str) -> Result<bool, StoreError> {
        let mut cache = self.cache.write();
        let mut wal = self.wal.lock();
        let existed = cache.remove(key).is_some();
        if existed {
            wal.append(&WalRecord::Delete {
                key: key.to_string(),
            })?;
        }
        drop(wal);
        drop(cache);
        if existed {
            self.events.publish(StorageEvent::KeyDeleted {
                key: key.to_string(),
            });
        }
        Ok(existed)
    }

    pub fn clear(&self) -> Result<(), StoreError> {
        let mut cache = self.cache.write();
        let mut wal = self.wal.lock();
        wal.append(&WalRecord::Clear)?;
        cache.clear();
        drop(wal);
        drop(cache);
        self.events.publish(StorageEvent::StorageCleared);
        Ok(())
    }

   pub fn keys(&self) -> Vec<String> {
        let now = now_ms();
        self.cache
            .read()
            .iter()
            .filter(|(_, e)| !e.is_expired(now))
            .map(|(k, _)| k.clone())
            .collect()
    }

    pub fn export(&self) -> Result<HashMap<String, Entry>, StoreError> {
        let mut result = HashMap::new();
        let guard = self.cache.read();
        for (key, entry) in guard.iter() {
            if !entry.is_expired(now_ms()) {
                result.insert(key.clone(), entry.clone());
            }
        }
        drop(guard);
        Ok(result)
    }

    pub fn import(&self, data: HashMap<String, Entry>) -> Result<(), StoreError> {
        let mut cache = self.cache.write();
        let mut wal = self.wal.lock();
        for (key, entry) in data.into_iter() {
            wal.append(&WalRecord::Set {
                key: key.clone(),
                entry: entry.clone(),
            })?;
            cache.insert(key, entry);
        }
        drop(wal);
        drop(cache);
        Ok(())
    }

    pub fn len(&self) -> i64 {
        let now = now_ms();
        self.cache
            .read()
            .values()
            .filter(|e| !e.is_expired(now))
            .count() as i64
    }

    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }

    pub fn close(&self) -> Result<(), StoreError> {
        let mut wal = self.wal.lock();
        wal.close()
    }

    pub fn drop(&self) -> Result<(), StoreError> {
        let mut wal = self.wal.lock();
        wal.delete()
    }

    pub fn increment(&self, key: &str, delta: i64, ttl_ms: Option<i64>) -> Result<i64, StoreError> {
        let mut cache = self.cache.write();
        let mut wal = self.wal.lock();
       
       let now = now_ms();
       
    let current = match cache.get(key) {
            Some(e) if !e.is_expired(now) => {
                let text = std::str::from_utf8(&e.value)
                    .map_err(|_| StoreError::InvalidCounterValue(key.to_string()))?;
                text.parse::<i64>()
                    .map_err(|_| StoreError::InvalidCounterValue(key.to_string()))?
            }
            _ => 0,
        };

        let next = current.saturating_add(delta);
        let entry = Entry {
            value: next.to_string().into_bytes(),
            encrypted: false,
            expires_at: expires_at_from_ttl(ttl_ms),
        };
        wal.append(&WalRecord::Set {
            key: key.to_string(),
            entry: entry.clone(),
        })?;
        let existed = cache.insert(key.to_string(), entry).is_some();
        drop(wal);
        drop(cache);

        self.events.publish(if existed {
            StorageEvent::KeyUpdated {
                key: key.to_string(),
            }
        } else {
            StorageEvent::KeyCreated {
                key: key.to_string(),
            }
        });
        Ok(next)
    }

    pub fn set_if_absent(
        &self,
        key: String,
        value: Vec<u8>,
        ttl_ms: Option<i64>,
        encrypted: bool,
    ) -> Result<bool, StoreError> {
        let mut cache = self.cache.write();
        let now = now_ms();
        if let Some(existing) = cache.get(&key) {
            if !existing.is_expired(now) {
                return Ok(false);
            }
        }
        let stored_value = self.encode_value(&key, value, encrypted)?;
        let entry = Entry {
            value: stored_value,
            encrypted,
            expires_at: expires_at_from_ttl(ttl_ms),
        };

        let mut wal = self.wal.lock();
        wal.append(&WalRecord::Set {
            key: key.clone(),
            entry: entry.clone(),
        })?;
        cache.insert(key.clone(), entry);
        drop(wal);
        drop(cache);

        self.events.publish(StorageEvent::KeyCreated { key });
        Ok(true)
    }

    pub fn purge_expired(&self) -> i64 {
        let now = now_ms();
        let expired_keys: Vec<String> = {
            let cache = self.cache.read();
            cache
                .iter()
                .filter(|(_, e)| e.is_expired(now))
                .map(|(k, _)| k.clone())
                .collect()
        };
        for key in &expired_keys {
            self.expire_key(key);
        }
        expired_keys.len() as i64
    }

    /// Rewrites the WAL to hold only current, live entries.
    pub fn compact(&self) -> Result<(), StoreError> {
        self.purge_expired();
        let cache = self.cache.read();
        Wal::compact(&self.wal_path, &cache)?;
        drop(cache);
        let mut wal = self.wal.lock();
        wal.reopen()
    }

    pub fn subscribe(&self, pattern: String, sink: StreamSink<StorageEvent>) {
        self.events.subscribe(pattern, sink);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::atomic::{AtomicUsize, Ordering};

    fn temp_test_dir() -> String {
        static COUNTER: AtomicUsize = AtomicUsize::new(0);
        let n = COUNTER.fetch_add(1, Ordering::Relaxed);
        std::env::temp_dir()
            .join(format!("rodinia_test_{}_{}", std::process::id(), n))
            .to_string_lossy()
            .into_owned()
    }

    #[test]
    fn test_open() {
        let test_dir = temp_test_dir();
        let store = Store::open(&test_dir).unwrap();
        assert!(store.cache.read().is_empty());
        assert!(store.wal_path.exists());
        let _ = std::fs::remove_dir_all(&test_dir);
    }

    #[test]
    fn test_directory_not_overwritten() {
        let test_dir = temp_test_dir();
        let store1 = Store::open(&test_dir).unwrap();
        let wal_path_1 = store1.wal_path;
        let store2 = Store::open(&test_dir).unwrap();
        let wal_path_2 = store2.wal_path;
        assert_eq!(wal_path_1, wal_path_2);
        let _ = std::fs::remove_dir_all(&test_dir);
    }

    #[test]
    fn test_cache_is_populated_from_wal_on_open() {
        let test_dir = temp_test_dir();
        let store1 = Store::open(&test_dir).unwrap();
        store1.set("key1".to_string(), b"value1".to_vec(), None, false).unwrap();
        let store2 = Store::open(&test_dir).unwrap();
        assert_eq!(store2.get("key1").unwrap(), Some(b"value1".to_vec()));
        let _ = std::fs::remove_dir_all(&test_dir);
    }

    #[test]
    fn test_set() -> Result<(), StoreError> {
        let test_dir = temp_test_dir();
        let store = Store::open(&test_dir).unwrap();
        store.set("key1".to_string(), b"value1".to_vec(), None, false).unwrap();
        assert_eq!(store.get("key1").unwrap(), Some(b"value1".to_vec()));
        let _ = std::fs::remove_dir_all(&test_dir);
        Ok(())
    }

    #[test]
    fn test_get() -> Result<(), StoreError> {
        let test_dir = temp_test_dir();
        let store = Store::open(&test_dir).unwrap();
        store.set("key1".to_string(), b"value1".to_vec(), None, false).unwrap();
        assert_eq!(store.get("key1").unwrap(), Some(b"value1".to_vec()));
        let _ = std::fs::remove_dir_all(&test_dir);
        Ok(())
    }

    #[test]
    fn test_contains() -> Result<(), StoreError> {
        let test_dir = temp_test_dir();
        let store = Store::open(&test_dir).unwrap();
        store.set("key1".to_string(), b"value1".to_vec(), None, false).unwrap();
        assert!(store.contains("key1"));
        let _ = std::fs::remove_dir_all(&test_dir);
        Ok(())
    }

    #[test]
    fn test_delete() -> Result<(), StoreError> {
        let test_dir = temp_test_dir();
        let store = Store::open(&test_dir).unwrap();
        store.set("key1".to_string(), b"value1".to_vec(), None, false).unwrap();
        assert!(store.delete("key1").unwrap());
        assert!(!store.contains("key1"));
        let _ = std::fs::remove_dir_all(&test_dir);
        Ok(())
    }

    #[test]
    fn test_clear() -> Result<(), StoreError> {
        let test_dir = temp_test_dir();
        let store = Store::open(&test_dir).unwrap();
        store.set("key1".to_string(), b"value1".to_vec(), None, false).unwrap();
        store.set("key2".to_string(), b"value2".to_vec(), None, false).unwrap();
        store.clear().unwrap();
        assert!(store.cache.read().is_empty());
        let _ = std::fs::remove_dir_all(&test_dir);
        Ok(())
    }

    #[test]
    fn test_keys() -> Result<(), StoreError> {
        let test_dir = temp_test_dir();
        let store = Store::open(&test_dir).unwrap();
        store.set("key1".to_string(), b"value1".to_vec(), None, false).unwrap();
        store.set("key2".to_string(), b"value2".to_vec(), None, false).unwrap();
        let keys = store.keys();
        assert_eq!(keys.len(), 2);
        assert!(keys.contains(&"key1".to_string()));
        assert!(keys.contains(&"key2".to_string()));
        let _ = std::fs::remove_dir_all(&test_dir);
        Ok(())
    }

    #[test]
    fn test_export_import() -> Result<(), StoreError> {
        let test_dir = temp_test_dir();
        let store = Store::open(&test_dir).unwrap();
        store.set("key1".to_string(), b"value1".to_vec(), None, false).unwrap();
        store.set("key2".to_string(), b"value2".to_vec(), None, false).unwrap();
        let exported = store.export().unwrap();
        let store2 = Store::open(&test_dir).unwrap();
        store2.import(exported).unwrap();
        assert_eq!(store2.get("key1").unwrap(), Some(b"value1".to_vec()));
        assert_eq!(store2.get("key2").unwrap(), Some(b"value2".to_vec()));
        let _ = std::fs::remove_dir_all(&test_dir);
        Ok(())
    }

    #[test]
    fn test_len() -> Result<(), StoreError> {
        let test_dir = temp_test_dir();
        let store = Store::open(&test_dir).unwrap();
        store.set("key1".to_string(), b"value1".to_vec(), None, false).unwrap();
        store.set("key2".to_string(), b"value2".to_vec(), None, false).unwrap();
        assert_eq!(store.len(), 2);
        let _ = std::fs::remove_dir_all(&test_dir);
        Ok(())
    }

    #[test]
    fn test_encrypt_decrypt_key_roundtrip() -> Result<(), StoreError> {
        let test_dir = temp_test_dir();
        let store = Store::open(&test_dir).unwrap();
        store.set_encryption_key(vec![7u8; crypto::KEY_LEN]).unwrap();
        let encrypted = store.encrypt_key("secret-key").unwrap();
        assert_eq!(store.decrypt_key(&encrypted).unwrap(), "secret-key");
        let _ = std::fs::remove_dir_all(&test_dir);
        Ok(())
    }

    #[test]
    fn test_close_drop() -> Result<(), StoreError> {
        let test_dir = temp_test_dir();
        let store = Store::open(&test_dir).unwrap();
        store.close().unwrap();
        store.drop().unwrap();
        let _ = std::fs::remove_dir_all(&test_dir);
        Ok(())
    }

    #[test]
    fn test_increment() -> Result<(), StoreError> {
        let test_dir = temp_test_dir();
        let store = Store::open(&test_dir).unwrap();
        store.increment("counter1", 1, None).unwrap();
        assert_eq!(store.get("counter1").unwrap(), Some(b"1".to_vec()));
        let _ = std::fs::remove_dir_all(&test_dir);
        Ok(())
    }

    #[test]
    fn test_set_if_absent() -> Result<(), StoreError> {
        let test_dir = temp_test_dir();
        let store = Store::open(&test_dir).unwrap();
        let result = store.set_if_absent("key1".to_string(), b"value1".to_vec(), None, false).unwrap();
        assert!(result);
        let result2 = store.set_if_absent("key1".to_string(), b"value2".to_vec(), None, false).unwrap();
        assert!(!result2);
        let _ = std::fs::remove_dir_all(&test_dir);
        Ok(())
    }

    #[test]
    fn test_purge_expired() -> Result<(), StoreError> {
        let test_dir = temp_test_dir();
        let store = Store::open(&test_dir).unwrap();
        store.set("key1".to_string(), b"value1".to_vec(), Some(-1), false).unwrap();
        let expired_count = store.purge_expired();
        assert_eq!(expired_count, 1);
        assert!(!store.contains("key1"));
        let _ = std::fs::remove_dir_all(&test_dir);
        Ok(())
    }

    #[test]
    fn test_compact() -> Result<(), StoreError> {
        let test_dir = temp_test_dir();
        let store = Store::open(&test_dir).unwrap();
        store.set("key1".to_string(), b"value1".to_vec(), None, false).unwrap();
        store.compact().unwrap();
        assert!(store.wal_path.exists());
        let _ = std::fs::remove_dir_all(&test_dir);
        Ok(())
    }
    
    #[test]
    fn test_add_to_list() -> Result<(), StoreError> {
        let test_dir = temp_test_dir();
        let store = Store::open(&test_dir).unwrap();
        store.list_push("list1", b"value1".to_vec(), None, false).unwrap();
        assert_eq!(store.list_get("list1").unwrap(), Some(vec![b"value1".to_vec()]));
        let _ = std::fs::remove_dir_all(&test_dir);
        Ok(())
    }
    #[test]
    fn test_delete_from_list() -> Result<(), StoreError> {
        let test_dir = temp_test_dir();
        let store = Store::open(&test_dir).unwrap();
        store.list_push("list1", b"value1".to_vec(), None, false).unwrap();
        let result = store.delete_from_list("list1", b"value1".to_vec(), None, false).unwrap();
        assert!(result);
        let _ = std::fs::remove_dir_all(&test_dir);
        Ok(())    
    }

    #[test]
    fn test_list_get() -> Result<(), StoreError> {
        let test_dir = temp_test_dir();
        let store = Store::open(&test_dir).unwrap();
        store.list_push("list1", b"value1".to_vec(), None, false).unwrap();
        assert_eq!(store.list_get("list1").unwrap(), Some(vec![b"value1".to_vec()]));
        let _ = std::fs::remove_dir_all(&test_dir);
        Ok(())
    }
}