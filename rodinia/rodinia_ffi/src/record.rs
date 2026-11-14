pub(crate) const MAGIC: &[u8; 5] = b"RDNA\x02";

const FRAME_LEN: usize = 8;
const BODY_HEADER_LEN: usize = 14;
const FLAG_ENCRYPTED: u8 = 1;
const FLAG_HAS_EXPIRY: u8 = 2;

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub(crate) enum RecordKind {
    Set = 1,
    Delete = 2,
    Clear = 3,
}

impl RecordKind {
    fn from_byte(byte: u8) -> Option<Self> {
        match byte {
            1 => Some(Self::Set),
            2 => Some(Self::Delete),
            3 => Some(Self::Clear),
            _ => None,
        }
    }
}

pub(crate) struct Record<'a> {
    pub kind: RecordKind,
    pub key: &'a str,
    pub value: &'a [u8],
    pub encrypted: bool,
    pub expires_at: Option<u64>,
}

#[derive(Debug)]
pub(crate) struct Decoded {
    pub kind: RecordKind,
    pub key: String,
    pub value_start: usize,
    pub value_len: usize,
    pub encrypted: bool,
    pub expires_at: Option<u64>,
    pub total_len: usize,
}

pub(crate) fn encode(record: &Record, out: &mut Vec<u8>) {
    let body_len = BODY_HEADER_LEN + record.key.len() + record.value.len();
    out.reserve(FRAME_LEN + body_len);
    out.extend_from_slice(&(body_len as u32).to_le_bytes());
    let crc_at = out.len();
    out.extend_from_slice(&[0; 4]);
    let body_at = out.len();

    let mut flags = 0;
    if record.encrypted {
        flags |= FLAG_ENCRYPTED;
    }
    if record.expires_at.is_some() {
        flags |= FLAG_HAS_EXPIRY;
    }
    out.push(record.kind as u8);
    out.push(flags);
    out.extend_from_slice(&record.expires_at.unwrap_or(0).to_le_bytes());
    out.extend_from_slice(&(record.key.len() as u32).to_le_bytes());
    out.extend_from_slice(record.key.as_bytes());
    out.extend_from_slice(record.value);

    let crc = crc32fast::hash(&out[body_at..]);
    out[crc_at..body_at].copy_from_slice(&crc.to_le_bytes());
}

fn u32_at(buf: &[u8], at: usize) -> usize {
    u32::from_le_bytes(buf[at..at + 4].try_into().unwrap()) as usize
}

pub(crate) fn decode(buf: &[u8]) -> Option<Decoded> {
    if buf.len() < FRAME_LEN {
        return None;
    }
    let body_len = u32_at(buf, 0);
    let total_len = FRAME_LEN.checked_add(body_len)?;
    if body_len < BODY_HEADER_LEN || buf.len() < total_len {
        return None;
    }
    let body = &buf[FRAME_LEN..total_len];
    if crc32fast::hash(body) != u32_at(buf, 4) as u32 {
        return None;
    }

    let kind = RecordKind::from_byte(body[0])?;
    let flags = body[1];
    let expires_at = u64::from_le_bytes(body[2..10].try_into().unwrap());
    let key_len = u32_at(body, 10);
    let key_end = BODY_HEADER_LEN.checked_add(key_len)?;
    if key_end > body.len() {
        return None;
    }
    let key = std::str::from_utf8(&body[BODY_HEADER_LEN..key_end])
        .ok()?
        .to_owned();

    Some(Decoded {
        kind,
        key,
        value_start: FRAME_LEN + key_end,
        value_len: body.len() - key_end,
        encrypted: flags & FLAG_ENCRYPTED != 0,
        expires_at: (flags & FLAG_HAS_EXPIRY != 0).then_some(expires_at),
        total_len,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    fn encoded(record: &Record) -> Vec<u8> {
        let mut buf = Vec::new();
        encode(record, &mut buf);
        buf
    }

    fn set(key: &str, value: &[u8]) -> Record<'static> {
        Record {
            kind: RecordKind::Set,
            key: Box::leak(key.to_string().into_boxed_str()),
            value: Box::leak(value.to_vec().into_boxed_slice()),
            encrypted: false,
            expires_at: None,
        }
    }

    #[test]
    fn set_record_roundtrips() {
        let record = Record {
            kind: RecordKind::Set,
            key: "k",
            value: b"v",
            encrypted: true,
            expires_at: Some(42),
        };
        let buf = encoded(&record);
        let d = decode(&buf).unwrap();
        assert_eq!(d.kind, RecordKind::Set);
        assert_eq!(d.key, "k");
        assert!(d.encrypted);
        assert_eq!(d.expires_at, Some(42));
        assert_eq!(&buf[d.value_start..][..d.value_len], b"v");
        assert_eq!(d.total_len, buf.len());
    }

    #[test]
    fn delete_and_clear_roundtrip() {
        for kind in [RecordKind::Delete, RecordKind::Clear] {
            let record = Record {
                kind,
                key: if kind == RecordKind::Delete {
                    "gone"
                } else {
                    ""
                },
                value: &[],
                encrypted: false,
                expires_at: None,
            };
            let buf = encoded(&record);
            let d = decode(&buf).unwrap();
            assert_eq!(d.kind, kind);
            assert_eq!(d.key, record.key);
            assert_eq!(d.value_len, 0);
            assert_eq!(d.expires_at, None);
        }
    }

    #[test]
    fn unusual_keys_roundtrip() {
        let long = "x".repeat(65_536);
        for key in ["", "ключ🔑", long.as_str()] {
            let buf = encoded(&set(key, b"value"));
            let d = decode(&buf).unwrap();
            assert_eq!(d.key, key);
            assert_eq!(&buf[d.value_start..][..d.value_len], b"value");
        }
    }

    #[test]
    fn truncated_record_is_none() {
        let buf = encoded(&set("key", b"value"));
        for cut in 0..buf.len() {
            assert!(decode(&buf[..cut]).is_none(), "cut at {cut}");
        }
    }

    #[test]
    fn flipped_byte_is_none() {
        let buf = encoded(&set("key", b"value"));
        for i in 0..buf.len() {
            let mut corrupt = buf.clone();
            corrupt[i] ^= 0xFF;
            assert!(decode(&corrupt).is_none(), "flipped byte {i}");
        }
    }

    #[test]
    fn decodes_first_of_several_records() {
        let mut buf = encoded(&set("a", b"1"));
        let first_len = buf.len();
        encode(&set("b", b"22"), &mut buf);
        let d = decode(&buf).unwrap();
        assert_eq!(d.key, "a");
        assert_eq!(d.total_len, first_len);
        let d2 = decode(&buf[first_len..]).unwrap();
        assert_eq!(d2.key, "b");
    }
}
