use thiserror::Error;

#[derive(Debug, Error)]
pub enum StoreError {
    #[error("store not initialized: call `open()` first")]
    NotInitialized,
    #[error("store already initialized")]
    AlreadyInitialized,
    #[error("io error: {0}")]
    Io(String),
    #[error("serialization error: {0}")]
    Serialization(String),
    #[error("encryption key not set: call `setEncryptionKey()` before using `encrypted: true`")]
    EncryptionKeyNotSet,
    #[error("invalid encryption key length: expected 32 bytes, got {0}")]
    InvalidKeyLength(usize),
    #[error("encryption error")]
    Encryption,
    #[error("ciphertext is invalid or corrupted")]
    InvalidCiphertext,
    #[error("unsupported encryption format version: {0}")]
    UnsupportedEncryptionVersion(u8),
    #[error("decryption authentication failed")]
    AuthenticationFailed,
    #[error("value for key '{0}' is not a valid stored counter")]
    InvalidCounterValue(String),
}

pub type AppError = StoreError;

impl From<std::io::Error> for StoreError {
    fn from(e: std::io::Error) -> Self {
        StoreError::Io(e.to_string())
    }
}
