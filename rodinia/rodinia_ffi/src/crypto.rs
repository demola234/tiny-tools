use aes_gcm::{
    aead::{Aead, KeyInit, Payload},
    Aes256Gcm, Key, Nonce,
};
use rand::{rngs::OsRng, RngCore};

use crate::error::StoreError;

pub const KEY_LEN: usize = 32;
pub const NONCE_LEN: usize = 12;
pub const TAG_LEN: usize = 16;

const FORMAT_VERSION: u8 = 1;

/// Version + nonce + authentication tag.
const MIN_CIPHERTEXT_LEN: usize = 1 + NONCE_LEN + TAG_LEN;

/// A 256-bit encryption key.
#[derive(Clone)]
pub struct EncryptionKey([u8; KEY_LEN]);

impl EncryptionKey {
    /// Generate a cryptographically secure random AES-256 key.
    pub fn generate() -> Self {
        let mut bytes = [0u8; KEY_LEN];

        let mut rng = OsRng;
        rng.fill_bytes(&mut bytes);

        Self(bytes)
    }

    /// Create a key from exactly 32 bytes.
    pub fn from_bytes(bytes: [u8; KEY_LEN]) -> Self {
        Self(bytes)
    }

    /// Access the raw key bytes.
    pub fn as_bytes(&self) -> &[u8; KEY_LEN] {
        &self.0
    }
}

/// Encrypt a value using AES-256-GCM.
pub fn encrypt(
    key: &EncryptionKey,
    database_key: &[u8],
    plaintext: &[u8],
) -> Result<Vec<u8>, StoreError> {
    let cipher =
        Aes256Gcm::new(Key::<Aes256Gcm>::from_slice(key.as_bytes()));

    // Generate a fresh nonce for every encryption.
    let mut nonce_bytes = [0u8; NONCE_LEN];

    let mut rng = OsRng;
    rng.fill_bytes(&mut nonce_bytes);

    let nonce = Nonce::from_slice(&nonce_bytes);

    // The database key is authenticated but not encrypted.
    let ciphertext = cipher
        .encrypt(
            nonce,
            Payload {
                msg: plaintext,
                aad: database_key,
            },
        )
        .map_err(|_| StoreError::Encryption)?;

    let mut output =
        Vec::with_capacity(1 + NONCE_LEN + ciphertext.len());

    // Format version.
    output.push(FORMAT_VERSION);

    // Nonce.
    output.extend_from_slice(&nonce_bytes);

    // Ciphertext + authentication tag.
    output.extend_from_slice(&ciphertext);

    Ok(output)
}

/// Decrypt a value previously produced by [`encrypt`].
pub fn decrypt(
    key: &EncryptionKey,
    database_key: &[u8],
    data: &[u8],
) -> Result<Vec<u8>, StoreError> {
    if data.len() < MIN_CIPHERTEXT_LEN {
        return Err(StoreError::InvalidCiphertext);
    }

    let version = data[0];

    if version != FORMAT_VERSION {
        return Err(StoreError::UnsupportedEncryptionVersion(version));
    }

    let nonce_start = 1;
    let nonce_end = nonce_start + NONCE_LEN;

    let nonce_bytes = &data[nonce_start..nonce_end];
    let ciphertext = &data[nonce_end..];

    let cipher =
        Aes256Gcm::new(Key::<Aes256Gcm>::from_slice(key.as_bytes()));

    let nonce = Nonce::from_slice(nonce_bytes);

    cipher
        .decrypt(
            nonce,
            Payload {
                msg: ciphertext,
                aad: database_key,
            },
        )
        .map_err(|_| StoreError::AuthenticationFailed)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn roundtrip() {
        let key = EncryptionKey::generate();

        let encrypted =
            encrypt(&key, b"user:name", b"Ademola").unwrap();

        let decrypted =
            decrypt(&key, b"user:name", &encrypted).unwrap();

        assert_eq!(decrypted, b"Ademola");
    }

    #[test]
    fn encryption_is_randomized() {
        let key = EncryptionKey::generate();

        let a =
            encrypt(&key, b"user:name", b"same value").unwrap();

        let b =
            encrypt(&key, b"user:name", b"same value").unwrap();

        assert_ne!(a, b);
    }

    #[test]
    fn wrong_key_fails() {
        let key = EncryptionKey::generate();
        let wrong_key = EncryptionKey::generate();

        let encrypted =
            encrypt(&key, b"user:name", b"secret").unwrap();

        assert!(decrypt(
            &wrong_key,
            b"user:name",
            &encrypted
        )
        .is_err());
    }

    #[test]
    fn wrong_database_key_fails() {
        let key = EncryptionKey::generate();

        let encrypted =
            encrypt(&key, b"user:name", b"secret").unwrap();

        assert!(decrypt(
            &key,
            b"user:email",
            &encrypted
        )
        .is_err());
    }

    #[test]
    fn tampered_ciphertext_fails() {
        let key = EncryptionKey::generate();

        let mut encrypted =
            encrypt(&key, b"user:name", b"secret").unwrap();

        let last = encrypted.len() - 1;
        encrypted[last] ^= 0x01;

        assert!(matches!(
            decrypt(&key, b"user:name", &encrypted),
            Err(StoreError::AuthenticationFailed)
        ));
    }

    #[test]
    fn tampered_aad_fails() {
        let key = EncryptionKey::generate();

        let encrypted =
            encrypt(&key, b"user:name", b"secret").unwrap();

        assert!(matches!(
            decrypt(&key, b"user:email", &encrypted),
            Err(StoreError::AuthenticationFailed)
        ));
    }

    #[test]
    fn truncated_ciphertext_fails() {
        let key = EncryptionKey::generate();

        let encrypted =
            encrypt(&key, b"user:name", b"secret").unwrap();

        let truncated = &encrypted[..encrypted.len() - 1];

        assert!(decrypt(
            &key,
            b"user:name",
            truncated
        )
        .is_err());
    }

    #[test]
    fn invalid_version_fails() {
        let key = EncryptionKey::generate();

        let mut encrypted =
            encrypt(&key, b"user:name", b"secret").unwrap();

        encrypted[0] = 99;

        assert!(matches!(
            decrypt(&key, b"user:name", &encrypted),
            Err(StoreError::UnsupportedEncryptionVersion(99))
        ));
    }

    #[test]
    fn empty_plaintext_roundtrip() {
        let key = EncryptionKey::generate();

        let encrypted =
            encrypt(&key, b"empty", b"").unwrap();

        let decrypted =
            decrypt(&key, b"empty", &encrypted).unwrap();

        assert!(decrypted.is_empty());
    }

    #[test]
    fn different_plaintexts_produce_different_ciphertexts() {
        let key = EncryptionKey::generate();

        let a =
            encrypt(&key, b"key", b"hello").unwrap();

        let b =
            encrypt(&key, b"key", b"world").unwrap();

        assert_ne!(a, b);
    }
}