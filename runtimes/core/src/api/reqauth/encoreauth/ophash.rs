use anyhow::Context;
use sha3::{Digest, Sha3_256};
use std::str::FromStr;

pub struct OperationHash {
    output: sha3::digest::Output<Sha3_256>,
    hex: String,
}

impl OperationHash {
    pub fn new<'a>(
        obj: &[u8],
        action: &[u8],
        payload: Option<&[u8]>,
        additional_context: impl Iterator<Item = &'a [u8]>,
    ) -> Self {
        let mut hasher = <Sha3_256 as Digest>::new();
        hasher.update(obj);
        hasher.update(action);

        if let Some(payload) = payload {
            hasher.update(b"\0");
            hasher.update((payload.len() as u32).to_le_bytes());
            hasher.update(payload);
        }

        for c in additional_context {
            hasher.update(b"\0");
            hasher.update((c.len() as u32).to_le_bytes());
            hasher.update(c);
        }

        let output = hasher.finalize();
        let hex = hex::encode(output.as_slice());
        Self { output, hex }
    }

    pub fn as_hex(&self) -> &str {
        &self.hex
    }

    pub fn ct_eq(&self, other: &Self) -> bool {
        use subtle::ConstantTimeEq;
        self.output.ct_eq(&other.output).into()
    }
}

impl FromStr for OperationHash {
    type Err = anyhow::Error;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        let raw = hex::decode(s).context("invalid hex")?;
        let output = <sha3::digest::Output<Sha3_256>>::from_exact_iter(raw.into_iter())
            .context("invalid hash length")?;
        Ok(Self {
            output,
            hex: s.to_string(),
        })
    }
}
