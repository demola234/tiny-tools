use crate::cache::{now_ms, IndexEntry, ValueLoc};
use crate::error::StoreError;
use crate::legacy;
use crate::record::{self, Record, RecordKind, MAGIC};
use std::collections::HashMap;
use std::fs::{File, OpenOptions};
use std::io::{BufReader, BufWriter, ErrorKind, Read, Write};
use std::path::{Path, PathBuf};

const FRAME_LEN: u64 = 8;
const MAX_RETAINED_BUFFER: usize = 1 << 20;

pub(crate) struct Replay {
    pub index: HashMap<String, IndexEntry>,
    pub live_bytes: u64,
}

pub(crate) struct Wal {
    path: PathBuf,
    writer: File,
    reader: File,
    len: u64,
    buf: Vec<u8>,
}

fn io_err(e: std::io::Error) -> StoreError {
    StoreError::Io(e.to_string())
}

fn tmp_path(path: &Path) -> PathBuf {
    path.with_extension("wal.tmp")
}

fn remove_if_present(path: &Path) -> std::io::Result<()> {
    match std::fs::remove_file(path) {
        Err(e) if e.kind() != ErrorKind::NotFound => Err(e),
        _ => Ok(()),
    }
}

fn sync_parent_dir(path: &Path) -> std::io::Result<()> {
    #[cfg(unix)]
    if let Some(parent) = path.parent() {
        File::open(parent)?.sync_all()?;
    }
    #[cfg(not(unix))]
    let _ = path;
    Ok(())
}

#[cfg(unix)]
fn read_exact_at(file: &File, buf: &mut [u8], offset: u64) -> std::io::Result<()> {
    use std::os::unix::fs::FileExt;
    file.read_exact_at(buf, offset)
}

#[cfg(windows)]
fn read_exact_at(file: &File, mut buf: &mut [u8], mut offset: u64) -> std::io::Result<()> {
    use std::os::windows::fs::FileExt;
    while !buf.is_empty() {
        match file.seek_read(buf, offset)? {
            0 => return Err(ErrorKind::UnexpectedEof.into()),
            n => {
                buf = &mut buf[n..];
                offset += n as u64;
            }
        }
    }
    Ok(())
}

enum Header {
    Current,
    Fresh,
    Legacy,
}

fn read_header(path: &Path) -> std::io::Result<Header> {
    let mut head = Vec::with_capacity(MAGIC.len());
    match File::open(path) {
        Ok(file) => file.take(MAGIC.len() as u64).read_to_end(&mut head)?,
        Err(e) if e.kind() == ErrorKind::NotFound => return Ok(Header::Fresh),
        Err(e) => return Err(e),
    };
    Ok(if head.len() == MAGIC.len() && head == MAGIC {
        Header::Current
    } else if MAGIC.starts_with(&head) {
        Header::Fresh
    } else {
        Header::Legacy
    })
}

fn write_fresh(path: &Path) -> std::io::Result<()> {
    let mut file = File::create(path)?;
    file.write_all(MAGIC)?;
    file.sync_data()
}

fn migrate_legacy(path: &Path) -> std::io::Result<()> {
    let entries = legacy::replay_live(path, now_ms())?;
    let tmp = tmp_path(path);
    let result = (|| {
        let mut out = BufWriter::new(File::create(&tmp)?);
        out.write_all(MAGIC)?;
        let mut buf = Vec::new();
        for (key, entry) in &entries {
            buf.clear();
            record::encode(
                &Record {
                    kind: RecordKind::Set,
                    key,
                    value: &entry.value,
                    encrypted: entry.encrypted,
                    expires_at: entry.expires_at,
                },
                &mut buf,
            );
            out.write_all(&buf)?;
        }
        out.into_inner()?.sync_data()?;
        std::fs::rename(&tmp, path)?;
        sync_parent_dir(path)
    })();
    if result.is_err() {
        let _ = std::fs::remove_file(&tmp);
    }
    result
}

fn replay(path: &Path) -> std::io::Result<(Replay, u64)> {
    let file = File::open(path)?;
    let file_len = file.metadata()?.len();
    let mut reader = BufReader::with_capacity(64 * 1024, file);
    reader.read_exact(&mut [0u8; MAGIC.len()])?;

    let mut offset = MAGIC.len() as u64;
    let mut index = HashMap::new();
    let mut buf = Vec::new();
    loop {
        let mut frame = [0u8; FRAME_LEN as usize];
        if reader.read_exact(&mut frame).is_err() {
            break;
        }
        let body_len = u32::from_le_bytes(frame[..4].try_into().unwrap()) as u64;
        if offset + FRAME_LEN + body_len > file_len {
            break;
        }
        buf.clear();
        buf.extend_from_slice(&frame);
        buf.resize((FRAME_LEN + body_len) as usize, 0);
        if reader.read_exact(&mut buf[FRAME_LEN as usize..]).is_err() {
            break;
        }
        let Some(decoded) = record::decode(&buf) else {
            break;
        };
        match decoded.kind {
            RecordKind::Set => {
                index.insert(
                    decoded.key,
                    IndexEntry {
                        loc: ValueLoc {
                            offset: offset + decoded.value_start as u64,
                            len: decoded.value_len as u32,
                        },
                        record_len: decoded.total_len as u32,
                        encrypted: decoded.encrypted,
                        expires_at: decoded.expires_at,
                    },
                );
            }
            RecordKind::Delete => {
                index.remove(&decoded.key);
            }
            RecordKind::Clear => index.clear(),
        }
        offset += decoded.total_len as u64;
    }

    let now = now_ms();
    index.retain(|_, entry: &mut IndexEntry| !entry.is_expired(now));
    let live_bytes = index.values().map(|e| e.record_len as u64).sum();
    Ok((Replay { index, live_bytes }, offset))
}

fn open_handles(path: &Path) -> std::io::Result<(File, File)> {
    let writer = OpenOptions::new().append(true).open(path)?;
    let reader = File::open(path)?;
    Ok((writer, reader))
}

impl Wal {
    pub fn open(path: &Path) -> Result<(Self, Replay), StoreError> {
        Self::open_io(path).map_err(io_err)
    }

    fn open_io(path: &Path) -> std::io::Result<(Self, Replay)> {
        remove_if_present(&tmp_path(path))?;
        match read_header(path)? {
            Header::Current => {}
            Header::Fresh => write_fresh(path)?,
            Header::Legacy => migrate_legacy(path)?,
        }

        let (replay, good_len) = replay(path)?;
        if good_len < std::fs::metadata(path)?.len() {
            let file = OpenOptions::new().write(true).open(path)?;
            file.set_len(good_len)?;
            file.sync_data()?;
        }

        let (writer, reader) = open_handles(path)?;
        let wal = Self {
            path: path.to_path_buf(),
            writer,
            reader,
            len: good_len,
            buf: Vec::new(),
        };
        Ok((wal, replay))
    }

    #[cfg(test)]
    pub fn path(&self) -> &Path {
        &self.path
    }

    pub fn file_len(&self) -> u64 {
        self.len
    }

    pub fn append(
        &mut self,
        record: &Record,
        durable: bool,
    ) -> Result<(ValueLoc, u32), StoreError> {
        self.buf.clear();
        record::encode(record, &mut self.buf);
        let start = self.len;
        let record_len = self.buf.len();

        let written = self.writer.write_all(&self.buf).and_then(|()| {
            if durable {
                self.writer.sync_data()
            } else {
                Ok(())
            }
        });
        if self.buf.capacity() > MAX_RETAINED_BUFFER {
            self.buf = Vec::new();
        }
        if let Err(e) = written {
            let _ = self.writer.set_len(start);
            return Err(io_err(e));
        }

        self.len += record_len as u64;
        let loc = ValueLoc {
            offset: start + (record_len - record.value.len()) as u64,
            len: record.value.len() as u32,
        };
        Ok((loc, record_len as u32))
    }

    pub fn read_value(&self, loc: ValueLoc) -> Result<Vec<u8>, StoreError> {
        let mut value = vec![0u8; loc.len as usize];
        read_exact_at(&self.reader, &mut value, loc.offset).map_err(io_err)?;
        Ok(value)
    }

    pub fn compact(
        &mut self,
        index: &HashMap<String, IndexEntry>,
    ) -> Result<HashMap<String, ValueLoc>, StoreError> {
        let tmp = tmp_path(&self.path);
        let result = self.compact_into(&tmp, index);
        if result.is_err() {
            let _ = std::fs::remove_file(&tmp);
        }
        result.map_err(io_err)
    }

    fn compact_into(
        &mut self,
        tmp: &Path,
        index: &HashMap<String, IndexEntry>,
    ) -> std::io::Result<HashMap<String, ValueLoc>> {
        let mut out = BufWriter::new(File::create(tmp)?);
        out.write_all(MAGIC)?;
        let mut offset = MAGIC.len() as u64;
        let mut locs = HashMap::with_capacity(index.len());
        let mut buf = Vec::new();
        let mut value = Vec::new();
        for (key, entry) in index {
            value.resize(entry.loc.len as usize, 0);
            read_exact_at(&self.reader, &mut value, entry.loc.offset)?;
            buf.clear();
            record::encode(
                &Record {
                    kind: RecordKind::Set,
                    key,
                    value: &value,
                    encrypted: entry.encrypted,
                    expires_at: entry.expires_at,
                },
                &mut buf,
            );
            out.write_all(&buf)?;
            locs.insert(
                key.clone(),
                ValueLoc {
                    offset: offset + (buf.len() - value.len()) as u64,
                    len: entry.loc.len,
                },
            );
            offset += buf.len() as u64;
        }
        out.into_inner()?.sync_data()?;
        std::fs::rename(tmp, &self.path)?;
        sync_parent_dir(&self.path)?;

        let (writer, reader) = open_handles(&self.path)?;
        self.writer = writer;
        self.reader = reader;
        self.len = offset;
        Ok(locs)
    }

    pub fn close(&mut self) -> Result<(), StoreError> {
        self.writer.sync_data().map_err(io_err)
    }

    pub fn delete(&mut self) -> Result<(), StoreError> {
        remove_if_present(&self.path).map_err(io_err)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::cache::Entry;
    use crate::legacy::WalRecord;
    use std::sync::atomic::{AtomicUsize, Ordering};

    struct TempDir(PathBuf);

    impl TempDir {
        fn new() -> Self {
            static N: AtomicUsize = AtomicUsize::new(0);
            let dir = std::env::temp_dir().join(format!(
                "rodinia_wal_test_{}_{}",
                std::process::id(),
                N.fetch_add(1, Ordering::Relaxed)
            ));
            std::fs::create_dir_all(&dir).unwrap();
            Self(dir)
        }

        fn log(&self) -> PathBuf {
            self.0.join("rodinia.wal")
        }
    }

    impl Drop for TempDir {
        fn drop(&mut self) {
            let _ = std::fs::remove_dir_all(&self.0);
        }
    }

    fn set<'a>(key: &'a str, value: &'a [u8]) -> Record<'a> {
        Record {
            kind: RecordKind::Set,
            key,
            value,
            encrypted: false,
            expires_at: None,
        }
    }

    fn delete(key: &str) -> Record<'_> {
        Record {
            kind: RecordKind::Delete,
            key,
            value: &[],
            encrypted: false,
            expires_at: None,
        }
    }

    fn read(wal: &Wal, replay: &Replay, key: &str) -> Vec<u8> {
        wal.read_value(replay.index[key].loc).unwrap()
    }

    fn write_five(path: &Path) -> Vec<u64> {
        let (mut wal, _) = Wal::open(path).unwrap();
        let mut ends = Vec::new();
        for i in 0..5 {
            wal.append(&set(&format!("k{i}"), b"value"), false).unwrap();
            ends.push(wal.file_len());
        }
        ends
    }

    #[test]
    fn new_log_starts_with_magic() {
        let dir = TempDir::new();
        let (wal, replay) = Wal::open(&dir.log()).unwrap();
        assert!(replay.index.is_empty());
        assert_eq!(wal.file_len(), MAGIC.len() as u64);
        assert_eq!(&std::fs::read(dir.log()).unwrap()[..], &MAGIC[..]);
    }

    #[test]
    fn replay_rebuilds_index_and_reads_values() {
        let dir = TempDir::new();
        {
            let (mut wal, _) = Wal::open(&dir.log()).unwrap();
            wal.append(&set("a", b"1"), false).unwrap();
            wal.append(&set("b", b"22"), false).unwrap();
            wal.append(&set("c", b"333"), false).unwrap();
            wal.append(&delete("b"), false).unwrap();
        }
        let (wal, replay) = Wal::open(&dir.log()).unwrap();
        assert_eq!(replay.index.len(), 2);
        assert_eq!(read(&wal, &replay, "a"), b"1");
        assert_eq!(read(&wal, &replay, "c"), b"333");
    }

    #[test]
    fn append_returns_location_of_value() {
        let dir = TempDir::new();
        let (mut wal, _) = Wal::open(&dir.log()).unwrap();
        let (loc, record_len) = wal.append(&set("key", b"hello"), false).unwrap();
        assert_eq!(wal.read_value(loc).unwrap(), b"hello");
        assert_eq!(wal.file_len(), MAGIC.len() as u64 + record_len as u64);
    }

    #[test]
    fn replay_keeps_record_metadata_and_live_bytes() {
        let dir = TempDir::new();
        let record_len = {
            let (mut wal, _) = Wal::open(&dir.log()).unwrap();
            wal.append(&set("k", b"old"), false).unwrap();
            let (_, len) = wal
                .append(
                    &Record {
                        kind: RecordKind::Set,
                        key: "k",
                        value: b"new",
                        encrypted: true,
                        expires_at: Some(u64::MAX),
                    },
                    false,
                )
                .unwrap();
            len
        };
        let (_, replay) = Wal::open(&dir.log()).unwrap();
        let entry = replay.index["k"];
        assert!(entry.encrypted);
        assert_eq!(entry.expires_at, Some(u64::MAX));
        assert_eq!(entry.record_len, record_len);
        assert_eq!(replay.live_bytes, record_len as u64);
    }

    #[test]
    fn clear_record_empties_index_on_replay() {
        let dir = TempDir::new();
        {
            let (mut wal, _) = Wal::open(&dir.log()).unwrap();
            wal.append(&set("a", b"1"), false).unwrap();
            wal.append(
                &Record {
                    kind: RecordKind::Clear,
                    key: "",
                    value: &[],
                    encrypted: false,
                    expires_at: None,
                },
                false,
            )
            .unwrap();
            wal.append(&set("b", b"2"), false).unwrap();
        }
        let (_, replay) = Wal::open(&dir.log()).unwrap();
        assert_eq!(replay.index.keys().collect::<Vec<_>>(), vec!["b"]);
    }

    #[test]
    fn expired_entries_are_dropped_on_replay() {
        let dir = TempDir::new();
        {
            let (mut wal, _) = Wal::open(&dir.log()).unwrap();
            wal.append(
                &Record {
                    kind: RecordKind::Set,
                    key: "old",
                    value: b"v",
                    encrypted: false,
                    expires_at: Some(1),
                },
                false,
            )
            .unwrap();
        }
        let (_, replay) = Wal::open(&dir.log()).unwrap();
        assert!(replay.index.is_empty());
        assert_eq!(replay.live_bytes, 0);
    }

    #[test]
    fn torn_tail_reopens_to_prefix() {
        let dir = TempDir::new();
        let ends = write_five(&dir.log());
        for cut in ends[3] + 1..ends[4] {
            std::fs::copy(dir.log(), dir.0.join("full")).unwrap();
            let file = std::fs::OpenOptions::new()
                .write(true)
                .open(dir.log())
                .unwrap();
            file.set_len(cut).unwrap();
            drop(file);
            let (wal, replay) = Wal::open(&dir.log()).unwrap();
            assert_eq!(replay.index.len(), 4, "cut at {cut}");
            assert_eq!(wal.file_len(), ends[3]);
            assert_eq!(std::fs::metadata(dir.log()).unwrap().len(), ends[3]);
            drop(wal);
            std::fs::copy(dir.0.join("full"), dir.log()).unwrap();
        }
    }

    #[test]
    fn corrupt_tail_reopens_to_prefix() {
        let dir = TempDir::new();
        let ends = write_five(&dir.log());
        let mut bytes = std::fs::read(dir.log()).unwrap();
        bytes[ends[4] as usize - 2] ^= 0xFF;
        std::fs::write(dir.log(), &bytes).unwrap();
        let (wal, replay) = Wal::open(&dir.log()).unwrap();
        assert_eq!(replay.index.len(), 4);
        assert!(!replay.index.contains_key("k4"));
        assert_eq!(wal.file_len(), ends[3]);
    }

    #[test]
    fn appends_after_repaired_tail_are_replayed() {
        let dir = TempDir::new();
        let ends = write_five(&dir.log());
        let file = std::fs::OpenOptions::new()
            .write(true)
            .open(dir.log())
            .unwrap();
        file.set_len(ends[4] - 1).unwrap();
        drop(file);
        {
            let (mut wal, _) = Wal::open(&dir.log()).unwrap();
            wal.append(&set("after", b"x"), false).unwrap();
        }
        let (wal, replay) = Wal::open(&dir.log()).unwrap();
        assert_eq!(read(&wal, &replay, "after"), b"x");
        assert_eq!(replay.index.len(), 5);
    }

    #[test]
    fn durable_append_succeeds() {
        let dir = TempDir::new();
        let (mut wal, _) = Wal::open(&dir.log()).unwrap();
        let (loc, _) = wal.append(&set("k", b"safe"), true).unwrap();
        assert_eq!(wal.read_value(loc).unwrap(), b"safe");
    }

    #[test]
    fn stale_tmp_file_is_removed_on_open() {
        let dir = TempDir::new();
        {
            let (mut wal, _) = Wal::open(&dir.log()).unwrap();
            wal.append(&set("k", b"real"), false).unwrap();
        }
        let tmp = dir.log().with_extension("wal.tmp");
        std::fs::write(&tmp, b"junk").unwrap();
        let (wal, replay) = Wal::open(&dir.log()).unwrap();
        assert!(!tmp.exists());
        assert_eq!(read(&wal, &replay, "k"), b"real");
    }

    #[test]
    fn legacy_log_is_migrated() {
        let dir = TempDir::new();
        let entry = |value: &[u8], expires_at| Entry {
            value: value.to_vec(),
            encrypted: false,
            expires_at,
        };
        legacy::write_log_for_test(
            &dir.log(),
            &[
                WalRecord::Set {
                    key: "a".into(),
                    entry: entry(b"1", None),
                },
                WalRecord::Set {
                    key: "gone".into(),
                    entry: entry(b"x", None),
                },
                WalRecord::Delete { key: "gone".into() },
                WalRecord::Set {
                    key: "expired".into(),
                    entry: entry(b"x", Some(1)),
                },
                WalRecord::Set {
                    key: "ключ🔑".into(),
                    entry: Entry {
                        value: b"secret".to_vec(),
                        encrypted: true,
                        expires_at: Some(u64::MAX),
                    },
                },
            ],
        );
        {
            let (wal, replay) = Wal::open(&dir.log()).unwrap();
            assert_eq!(replay.index.len(), 2);
            assert_eq!(read(&wal, &replay, "a"), b"1");
            assert!(replay.index["ключ🔑"].encrypted);
        }
        assert_eq!(
            &std::fs::read(dir.log()).unwrap()[..MAGIC.len()],
            &MAGIC[..]
        );
        let (wal, replay) = Wal::open(&dir.log()).unwrap();
        assert_eq!(read(&wal, &replay, "ключ🔑"), b"secret");
        assert_eq!(replay.index["ключ🔑"].expires_at, Some(u64::MAX));
    }

    #[test]
    fn compact_keeps_live_values_and_drops_history() {
        let dir = TempDir::new();
        let (mut wal, _) = Wal::open(&dir.log()).unwrap();
        for i in 0..100 {
            wal.append(&set("k", format!("v{i}").as_bytes()), false)
                .unwrap();
        }
        wal.append(&set("other", b"o"), false).unwrap();
        drop(wal);
        let (mut wal, replay) = Wal::open(&dir.log()).unwrap();
        let before = wal.file_len();
        let locs = wal.compact(&replay.index).unwrap();
        assert!(wal.file_len() < before / 10);
        assert_eq!(wal.file_len(), MAGIC.len() as u64 + replay.live_bytes);
        assert_eq!(wal.read_value(locs["k"]).unwrap(), b"v99");
        assert_eq!(wal.read_value(locs["other"]).unwrap(), b"o");
        wal.append(&set("new", b"n"), false).unwrap();
        drop(wal);
        let (wal, replay) = Wal::open(&dir.log()).unwrap();
        assert_eq!(read(&wal, &replay, "k"), b"v99");
        assert_eq!(read(&wal, &replay, "new"), b"n");
        assert!(!dir.log().with_extension("wal.tmp").exists());
    }
}
