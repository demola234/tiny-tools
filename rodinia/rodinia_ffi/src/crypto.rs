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

/// AES-256-GCM cipher built once per key and reused for every value.
pub struct Cipher(Aes256Gcm);

impl Cipher {
    pub fn new(key: &EncryptionKey) -> Self {
        Self(Aes256Gcm::new(Key::<Aes256Gcm>::from_slice(key.as_bytes())))
    }

    /// Encrypts `plaintext`, authenticating `database_key` as associated data.
    /// Output layout: format version, random nonce, ciphertext + tag.
    pub fn encrypt(&self, database_key: &[u8], plaintext: &[u8]) -> Result<Vec<u8>, StoreError> {
        let mut nonce_bytes = [0u8; NONCE_LEN];
        OsRng.fill_bytes(&mut nonce_bytes);

        let ciphertext = self
            .0
            .encrypt(
                Nonce::from_slice(&nonce_bytes),
                Payload {
                    msg: plaintext,
                    aad: database_key,
                },
            )
            .map_err(|_| StoreError::Encryption)?;

        let mut output = Vec::with_capacity(1 + NONCE_LEN + ciphertext.len());
        output.push(FORMAT_VERSION);
        output.extend_from_slice(&nonce_bytes);
        output.extend_from_slice(&ciphertext);
        Ok(output)
    }

    /// Decrypts a value produced by [`Cipher::encrypt`] or [`encrypt`].
    pub fn decrypt(&self, database_key: &[u8], data: &[u8]) -> Result<Vec<u8>, StoreError> {
        if data.len() < MIN_CIPHERTEXT_LEN {
            return Err(StoreError::InvalidCiphertext);
        }
        let version = data[0];
        if version != FORMAT_VERSION {
            return Err(StoreError::UnsupportedEncryptionVersion(version));
        }
        let (nonce_bytes, ciphertext) = data[1..].split_at(NONCE_LEN);
        self.0
            .decrypt(
                Nonce::from_slice(nonce_bytes),
                Payload {
                    msg: ciphertext,
                    aad: database_key,
                },
            )
            .map_err(|_| StoreError::AuthenticationFailed)
    }
}

/// Encrypt a value using AES-256-GCM.
#[cfg(test)]
pub fn encrypt(
    key: &EncryptionKey,
    database_key: &[u8],
    plaintext: &[u8],
) -> Result<Vec<u8>, StoreError> {
    Cipher::new(key).encrypt(database_key, plaintext)
}

/// Decrypt a value previously produced by [`encrypt`].
#[cfg(test)]
pub fn decrypt(
    key: &EncryptionKey,
    database_key: &[u8],
    data: &[u8],
) -> Result<Vec<u8>, StoreError> {
    Cipher::new(key).decrypt(database_key, data)
}

#[cfg(test)]
mod tests {
    use super::*;

    const FIXTURE: &str = "0157e9919a914e264ff47d13c6041983ef06cdc249d8ae6f9da866015bfc0c16dfdc";

    fn from_hex(s: &str) -> Vec<u8> {
        (0..s.len())
            .step_by(2)
            .map(|i| u8::from_str_radix(&s[i..i + 2], 16).unwrap())
            .collect()
    }

    #[test]
    fn cipher_decrypts_ciphertext_from_previous_code() {
        let cipher = Cipher::new(&EncryptionKey::from_bytes([7u8; KEY_LEN]));
        assert_eq!(cipher.decrypt(b"k", &from_hex(FIXTURE)).unwrap(), b"value");
    }

    #[test]
    fn cipher_matches_free_functions() {
        let key = EncryptionKey::from_bytes([9u8; KEY_LEN]);
        let cipher = Cipher::new(&key);
        let a = cipher.encrypt(b"aad", b"hello").unwrap();
        assert_eq!(decrypt(&key, b"aad", &a).unwrap(), b"hello");
        let b = encrypt(&key, b"aad", b"world").unwrap();
        assert_eq!(cipher.decrypt(b"aad", &b).unwrap(), b"world");
        assert!(matches!(
            cipher.decrypt(b"other", &b),
            Err(StoreError::AuthenticationFailed)
        ));
    }

    #[test]
    fn roundtrip() {
        let key = EncryptionKey::generate();

        let encrypted = encrypt(&key, b"user:name", b"Ademola").unwrap();

        let decrypted = decrypt(&key, b"user:name", &encrypted).unwrap();

        assert_eq!(decrypted, b"Ademola");
    }

    #[test]
    fn encryption_is_randomized() {
        let key = EncryptionKey::generate();

        let a = encrypt(&key, b"user:name", b"same value").unwrap();

        let b = encrypt(&key, b"user:name", b"same value").unwrap();

        assert_ne!(a, b);
    }

    #[test]
    fn wrong_key_fails() {
        let key = EncryptionKey::generate();
        let wrong_key = EncryptionKey::generate();

        let encrypted = encrypt(&key, b"user:name", b"secret").unwrap();

        assert!(decrypt(&wrong_key, b"user:name", &encrypted).is_err());
    }

    #[test]
    fn wrong_database_key_fails() {
        let key = EncryptionKey::generate();

        let encrypted = encrypt(&key, b"user:name", b"secret").unwrap();

        assert!(decrypt(&key, b"user:email", &encrypted).is_err());
    }

    #[test]
    fn tampered_ciphertext_fails() {
        let key = EncryptionKey::generate();

        let mut encrypted = encrypt(&key, b"user:name", b"secret").unwrap();

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

        let encrypted = encrypt(&key, b"user:name", b"secret").unwrap();

        assert!(matches!(
            decrypt(&key, b"user:email", &encrypted),
            Err(StoreError::AuthenticationFailed)
        ));
    }

    #[test]
    fn truncated_ciphertext_fails() {
        let key = EncryptionKey::generate();

        let encrypted = encrypt(&key, b"user:name", b"secret").unwrap();

        let truncated = &encrypted[..encrypted.len() - 1];

        assert!(decrypt(&key, b"user:name", truncated).is_err());
    }

    #[test]
    fn invalid_version_fails() {
        let key = EncryptionKey::generate();

        let mut encrypted = encrypt(&key, b"user:name", b"secret").unwrap();

        encrypted[0] = 99;

        assert!(matches!(
            decrypt(&key, b"user:name", &encrypted),
            Err(StoreError::UnsupportedEncryptionVersion(99))
        ));
    }

    #[test]
    fn empty_plaintext_roundtrip() {
        let key = EncryptionKey::generate();

        let encrypted = encrypt(&key, b"empty", b"").unwrap();

        let decrypted = decrypt(&key, b"empty", &encrypted).unwrap();

        assert!(decrypted.is_empty());
    }

    #[test]
    fn different_plaintexts_produce_different_ciphertexts() {
        let key = EncryptionKey::generate();

        let a = encrypt(&key, b"key", b"hello").unwrap();

        let b = encrypt(&key, b"key", b"world").unwrap();

        assert_ne!(a, b);
    }
}
