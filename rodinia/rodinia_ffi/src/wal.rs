use crate::cache::Entry;
use crate::error::StoreError;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::fs::{File, OpenOptions};
use std::io::{BufReader, BufWriter, Read, Write};
use std::path::{Path, PathBuf};

#[derive(Debug, Serialize, Deserialize)]
pub(crate) enum WalRecord {
    Set { key: String, entry: Entry },
    Delete { key: String },
    Clear,
}

/// Append-only, length-prefixed log of every mutation. Replayed on startup to
/// rebuild the in-memory index, and rewritten wholesale by [`Wal::compact`]
/// once dead entries (deleted/overwritten keys) make it worth reclaiming.
pub(crate) struct Wal {
    path: PathBuf,
    writer: BufWriter<File>,
}

fn io_err(e: std::io::Error) -> StoreError {
    StoreError::Io(e.to_string())
}

fn write_record(writer: &mut impl Write, record: &WalRecord) -> Result<(), StoreError> {
    let bytes = bincode::serialize(record).map_err(|e| StoreError::Serialization(e.to_string()))?;
    writer
        .write_all(&(bytes.len() as u32).to_le_bytes())
        .map_err(io_err)?;
    writer.write_all(&bytes).map_err(io_err)?;
    Ok(())
}

impl Wal {
    pub fn open(path: &Path) -> Result<Self, StoreError> {
        let file = OpenOptions::new()
            .create(true)
            .append(true)
            .open(path)
            .map_err(io_err)?;
        Ok(Self {
            path: path.to_path_buf(),
            writer: BufWriter::new(file),
        })
    }

    pub fn append(&mut self, record: &WalRecord) -> Result<(), StoreError> {
        write_record(&mut self.writer, record)?;
        self.writer.flush().map_err(io_err)
    }

    /// Replays every well-formed record in `path`. A truncated final record
    /// (e.g. from a crash mid-write) is treated as the end of the log rather
    /// than an error, since everything before it is still valid.
    pub fn replay(path: &Path) -> Result<Vec<WalRecord>, StoreError> {
        let mut records = Vec::new();
        let file = match File::open(path) {
            Ok(f) => f,
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(records),
            Err(e) => return Err(io_err(e)),
        };
        let mut reader = BufReader::new(file);
        loop {
            let mut len_buf = [0u8; 4];
            if reader.read_exact(&mut len_buf).is_err() {
                break;
            }
            let len = u32::from_le_bytes(len_buf) as usize;
            let mut data = vec![0u8; len];
            if reader.read_exact(&mut data).is_err() {
                break;
            }
            match bincode::deserialize::<WalRecord>(&data) {
                Ok(record) => records.push(record),
                Err(_) => break,
            }
        }
        Ok(records)
    }

    /// Rewrites the log to contain exactly one `Set` per live entry, dropping
    /// history for deleted/overwritten/expired keys. Writes to a temp file
    /// first and renames over the original so a crash mid-compaction can
    /// never corrupt or lose the existing log.
    pub fn compact(path: &Path, entries: &HashMap<String, Entry>) -> Result<(), StoreError> {
        let tmp_path = path.with_extension("wal.tmp");
        {
            let file = OpenOptions::new()
                .create(true)
                .write(true)
                .truncate(true)
                .open(&tmp_path)
                .map_err(io_err)?;
            let mut writer = BufWriter::new(file);
            for (key, entry) in entries {
                let record = WalRecord::Set {
                    key: key.clone(),
                    entry: entry.clone(),
                };
                write_record(&mut writer, &record)?;
            }
            writer.flush().map_err(io_err)?;
        }
        std::fs::rename(&tmp_path, path).map_err(io_err)
    }

    /// Reopens the log file for appending; used after [`Wal::compact`] has
    /// replaced the file this writer was pointing at.
    pub fn reopen(&mut self) -> Result<(), StoreError> {
        let file = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&self.path)
            .map_err(io_err)?;
        self.writer = BufWriter::new(file);
        Ok(())
    }

    /// Flushes any buffered writes to disk.
    pub fn close(&mut self) -> Result<(), StoreError> {
        self.writer.flush().map_err(io_err)
    }

    /// Flushes and removes the log file from disk.
    pub fn delete(&mut self) -> Result<(), StoreError> {
        self.writer.flush().map_err(io_err)?;
        match std::fs::remove_file(&self.path) {
            Ok(()) => Ok(()),
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(()),
            Err(e) => Err(io_err(e)),
        }
    }
}
