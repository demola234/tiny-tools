use crate::cache::Entry;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::fs::File;
use std::io::{BufReader, Read};
use std::path::Path;

#[derive(Debug, Serialize, Deserialize)]
pub(crate) enum WalRecord {
    Set { key: String, entry: Entry },
    Delete { key: String },
    Clear,
}

pub(crate) fn replay_live(path: &Path, now_ms: u64) -> std::io::Result<HashMap<String, Entry>> {
    let mut entries = HashMap::new();
    let mut reader = BufReader::new(File::open(path)?);
    loop {
        let mut len_buf = [0u8; 4];
        if reader.read_exact(&mut len_buf).is_err() {
            break;
        }
        let mut data = vec![0u8; u32::from_le_bytes(len_buf) as usize];
        if reader.read_exact(&mut data).is_err() {
            break;
        }
        match bincode::deserialize::<WalRecord>(&data) {
            Ok(WalRecord::Set { key, entry }) => {
                entries.insert(key, entry);
            }
            Ok(WalRecord::Delete { key }) => {
                entries.remove(&key);
            }
            Ok(WalRecord::Clear) => entries.clear(),
            Err(_) => break,
        }
    }
    entries.retain(|_, entry| !entry.is_expired(now_ms));
    Ok(entries)
}

#[cfg(test)]
pub(crate) fn write_log_for_test(path: &Path, records: &[WalRecord]) {
    use std::io::Write;
    let mut file = File::create(path).unwrap();
    for record in records {
        let bytes = bincode::serialize(record).unwrap();
        file.write_all(&(bytes.len() as u32).to_le_bytes()).unwrap();
        file.write_all(&bytes).unwrap();
    }
}
