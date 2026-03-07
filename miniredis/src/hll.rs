/// HyperLogLog probabilistic cardinality estimator.
///
/// Dense-only implementation with p=14 (16,384 registers).
const P: u8 = 14;
const M: usize = 1 << P; // 16384 registers

/// Alpha constant for bias correction.
fn alpha_m(m: f64) -> f64 {
    match m as u64 {
        16 => 0.673,
        32 => 0.697,
        64 => 0.709,
        _ => 0.7213 / (1.0 + 1.079 / m),
    }
}

/// Bias correction polynomial for p=14.
fn beta14(ez: f64) -> f64 {
    let zl = (ez + 1.0).ln();
    -0.370393911 * ez
        + 0.070471823 * zl
        + 0.17393686 * zl.powi(2)
        + 0.16339839 * zl.powi(3)
        + -0.09237745 * zl.powi(4)
        + 0.03738027 * zl.powi(5)
        + -0.005384159 * zl.powi(6)
        + 0.00042419 * zl.powi(7)
}

/// 64-bit hash function (FNV-1a with avalanche mixing).
fn hash64(data: &[u8]) -> u64 {
    let mut hash: u64 = 0xcbf29ce484222325;
    for &byte in data {
        hash ^= byte as u64;
        hash = hash.wrapping_mul(0x100000001b3);
    }
    // Avalanche: ensure all output bits depend on all input bits
    hash ^= hash >> 33;
    hash = hash.wrapping_mul(0xff51afd7ed558ccd);
    hash ^= hash >> 33;
    hash = hash.wrapping_mul(0xc4ceb9fe1a85ec53);
    hash ^= hash >> 33;
    hash
}

/// Extract register index and rho (leading zeros + 1) from a hash.
fn get_pos_val(hash: u64) -> (usize, u8) {
    // Top P bits → register index
    let idx = (hash >> (64 - P)) as usize;
    // Remaining bits: count leading zeros + 1
    // Set bit (P-1) to guarantee at least one 1-bit
    let w = (hash << P) | (1u64 << (P - 1));
    let rho = w.leading_zeros() as u8 + 1;
    (idx, rho)
}

/// HyperLogLog probabilistic set cardinality estimator.
#[derive(Clone, Debug)]
pub struct HyperLogLog {
    registers: Vec<u8>,
}

impl Default for HyperLogLog {
    fn default() -> Self {
        Self::new()
    }
}

impl HyperLogLog {
    pub fn new() -> Self {
        HyperLogLog {
            registers: vec![0; M],
        }
    }

    /// Add an element. Returns true if the internal state changed
    /// (i.e., the approximated cardinality changed).
    pub fn add(&mut self, element: &[u8]) -> bool {
        let hash = hash64(element);
        let (idx, rho) = get_pos_val(hash);
        if rho > self.registers[idx] {
            self.registers[idx] = rho;
            true
        } else {
            false
        }
    }

    /// Estimate the cardinality (number of unique elements).
    pub fn count(&self) -> u64 {
        let m = M as f64;
        let alpha = alpha_m(m);

        let mut sum = 0.0f64;
        let mut zeros = 0.0f64;

        for &reg in &self.registers {
            sum += 2.0f64.powi(-(reg as i32));
            if reg == 0 {
                zeros += 1.0;
            }
        }

        let est = alpha * m * (m - zeros) / (sum + beta14(zeros));
        (est + 0.5) as u64
    }

    /// Merge another HLL into this one (element-wise max of registers).
    pub fn merge(&mut self, other: &HyperLogLog) {
        for i in 0..M {
            if other.registers[i] > self.registers[i] {
                self.registers[i] = other.registers[i];
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_hll_empty() {
        let hll = HyperLogLog::new();
        assert_eq!(hll.count(), 0);
    }

    #[test]
    fn test_hll_add_single() {
        let mut hll = HyperLogLog::new();
        assert!(hll.add(b"hello"));
        assert!(!hll.add(b"hello")); // duplicate
        assert_eq!(hll.count(), 1);
    }

    #[test]
    fn test_hll_add_multiple() {
        let mut hll = HyperLogLog::new();
        for i in 0..100 {
            hll.add(format!("item-{}", i).as_bytes());
        }
        let count = hll.count();
        // Should be approximately 100 (within 10% for p=14)
        assert!((90..=110).contains(&count), "count was {}", count);
    }

    #[test]
    fn test_hll_duplicate_returns_false() {
        let mut hll = HyperLogLog::new();
        assert!(hll.add(b"a"));
        assert!(hll.add(b"b"));
        assert!(!hll.add(b"a")); // already added
        assert!(!hll.add(b"b")); // already added
    }

    #[test]
    fn test_hll_merge() {
        let mut hll1 = HyperLogLog::new();
        let mut hll2 = HyperLogLog::new();

        for i in 0..50 {
            hll1.add(format!("item-{}", i).as_bytes());
        }
        for i in 50..100 {
            hll2.add(format!("item-{}", i).as_bytes());
        }

        hll1.merge(&hll2);
        let count = hll1.count();
        // Should be approximately 100
        assert!((90..=110).contains(&count), "count was {}", count);
    }

    #[test]
    fn test_hll_merge_overlap() {
        let mut hll1 = HyperLogLog::new();
        let mut hll2 = HyperLogLog::new();

        // Add same elements to both
        for i in 0..50 {
            hll1.add(format!("item-{}", i).as_bytes());
            hll2.add(format!("item-{}", i).as_bytes());
        }

        hll1.merge(&hll2);
        let count = hll1.count();
        // Should still be approximately 50
        assert!((45..=55).contains(&count), "count was {}", count);
    }

    #[test]
    fn test_hll_large_cardinality() {
        let mut hll = HyperLogLog::new();
        for i in 0..10000 {
            hll.add(format!("element-{}", i).as_bytes());
        }
        let count = hll.count();
        // p=14 gives ~0.8% standard error, allow 5% margin
        assert!((9500..=10500).contains(&count), "count was {}", count);
    }
}
