use serde::{Deserialize, Serialize};
use std::time::{SystemTime, UNIX_EPOCH};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Entry {
    pub value: Vec<u8>,
    pub encrypted: bool,
    pub expires_at: Option<u64>,
}

impl Entry {
    pub fn is_expired(&self, now_ms: u64) -> bool {
        matches!(self.expires_at, Some(t) if t <= now_ms)
    }
}

pub fn now_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis() as u64
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_now_ms() {
        let now = now_ms();
        assert!(now > 0);
    }

    #[test]
    fn test_is_expired() {
        let entry = Entry {
            value: b"test".to_vec(),
            encrypted: false,
            expires_at: None,
        };
        assert!(!entry.is_expired(now_ms()));

        let now = now_ms();
        let entry = Entry {
            value: b"test".to_vec(),
            encrypted: false,
            expires_at: Some(now - 1000),
        };
        assert!(entry.is_expired(now_ms()));

        let entry = Entry {
            value: b"test".to_vec(),
            encrypted: false,
            expires_at: Some(now + 1000),
        };
        assert!(!entry.is_expired(now_ms()));
    }
}
