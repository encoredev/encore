use sha3::{Digest, Sha3_256};

pub fn hash_name(name: &str) -> String {
    let mut hasher = Sha3_256::new();
    hasher.update(name.as_bytes());
    let hash = hasher.finalize();

    // Encode the full 32-byte hash as 64 hex characters
    hex::encode(hash)
}
