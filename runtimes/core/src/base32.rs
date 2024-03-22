use std::cmp::min;

#[derive(Copy, Clone)]
pub enum Alphabet {
    RFC4648 { padding: bool },
    Crockford,
    Encore,
}

const RFC4648_ALPHABET: &'static [u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
const CROCKFORD_ALPHABET: &'static [u8] = b"0123456789ABCDEFGHJKMNPQRSTVWXYZ";
const ENCORE_ALPHABET: &'static [u8] = b"0123456789abcdefghijklmnopqrstuv";

pub fn encode(alphabet: Alphabet, data: &[u8]) -> String {
    let (alphabet, padding) = match alphabet {
        Alphabet::RFC4648 { padding } => (RFC4648_ALPHABET, padding),
        Alphabet::Crockford => (CROCKFORD_ALPHABET, false),
        Alphabet::Encore => (ENCORE_ALPHABET, false),
    };
    let mut ret = Vec::with_capacity((data.len() + 3) / 4 * 5);

    for chunk in data.chunks(5) {
        let buf = {
            let mut buf = [0u8; 5];
            for (i, &b) in chunk.iter().enumerate() {
                buf[i] = b;
            }
            buf
        };
        ret.push(alphabet[((buf[0] & 0xF8) >> 3) as usize]);
        ret.push(alphabet[(((buf[0] & 0x07) << 2) | ((buf[1] & 0xC0) >> 6)) as usize]);
        ret.push(alphabet[((buf[1] & 0x3E) >> 1) as usize]);
        ret.push(alphabet[(((buf[1] & 0x01) << 4) | ((buf[2] & 0xF0) >> 4)) as usize]);
        ret.push(alphabet[(((buf[2] & 0x0F) << 1) | (buf[3] >> 7)) as usize]);
        ret.push(alphabet[((buf[3] & 0x7C) >> 2) as usize]);
        ret.push(alphabet[(((buf[3] & 0x03) << 3) | ((buf[4] & 0xE0) >> 5)) as usize]);
        ret.push(alphabet[(buf[4] & 0x1F) as usize]);
    }

    if data.len() % 5 != 0 {
        let len = ret.len();
        let num_extra = 8 - (data.len() % 5 * 8 + 4) / 5;
        if padding {
            for i in 1..num_extra + 1 {
                ret[len - i] = b'=';
            }
        } else {
            ret.truncate(len - num_extra);
        }
    }

    String::from_utf8(ret).unwrap()
}

const RFC4648_INV_ALPHABET: [i8; 43] = [
    -1, -1, 26, 27, 28, 29, 30, 31, -1, -1, -1, -1, -1, 0, -1, -1, -1, 0, 1, 2, 3, 4, 5, 6, 7, 8,
    9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25,
];
const CROCKFORD_INV_ALPHABET: [i8; 43] = [
    0, 1, 2, 3, 4, 5, 6, 7, 8, 9, -1, -1, -1, -1, -1, -1, -1, 10, 11, 12, 13, 14, 15, 16, 17, 1,
    18, 19, 1, 20, 21, 0, 22, 23, 24, 25, 26, -1, 27, 28, 29, 30, 31,
];
const ENCORE_INV_ALPHABET: [i8; 43] = [
    0, 1, 2, 3, 4, 5, 6, 7, 8, 9, -1, -1, -1, -1, -1, -1, -1, 10, 11, 12, 13, 14, 15, 16, 17, 18,
    19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, -1, -1, -1, -1,
];

pub fn decode(alphabet: Alphabet, data: &str) -> Option<Vec<u8>> {
    if !data.is_ascii() {
        return None;
    }
    let data = data.as_bytes();
    let alphabet = match alphabet {
        Alphabet::RFC4648 { .. } => RFC4648_INV_ALPHABET,
        Alphabet::Crockford => CROCKFORD_INV_ALPHABET,
        Alphabet::Encore => ENCORE_INV_ALPHABET,
    };
    let mut unpadded_data_length = data.len();
    for i in 1..min(6, data.len()) + 1 {
        if data[data.len() - i] != b'=' {
            break;
        }
        unpadded_data_length -= 1;
    }
    let output_length = unpadded_data_length * 5 / 8;
    let mut ret = Vec::with_capacity((output_length + 4) / 5 * 5);
    for chunk in data.chunks(8) {
        let buf = {
            let mut buf = [0u8; 8];
            for (i, &c) in chunk.iter().enumerate() {
                match alphabet.get(c.to_ascii_uppercase().wrapping_sub(b'0') as usize) {
                    Some(&-1) | None => return None,
                    Some(&value) => buf[i] = value as u8,
                };
            }
            buf
        };
        ret.push((buf[0] << 3) | (buf[1] >> 2));
        ret.push((buf[1] << 6) | (buf[2] << 1) | (buf[3] >> 4));
        ret.push((buf[3] << 4) | (buf[4] >> 1));
        ret.push((buf[4] << 7) | (buf[5] << 2) | (buf[6] >> 3));
        ret.push((buf[6] << 5) | buf[7]);
    }
    ret.truncate(output_length);
    Some(ret)
}

#[cfg(test)]
#[allow(dead_code, unused_attributes)]
mod test {
    use super::Alphabet::{Crockford, Encore, RFC4648};
    use super::{decode, encode};
    use quickcheck;
    use std;

    #[derive(Clone)]
    struct B32 {
        c: u8,
    }

    impl quickcheck::Arbitrary for B32 {
        fn arbitrary(g: &mut quickcheck::Gen) -> B32 {
            let alphabet = b"0123456789ABCDEFGHJKMNPQRSTVWXYZ";
            B32 {
                c: g.choose(alphabet).unwrap().clone(),
            }
        }
    }

    impl std::fmt::Debug for B32 {
        fn fmt(&self, f: &mut std::fmt::Formatter) -> Result<(), std::fmt::Error> {
            (self.c as char).fmt(f)
        }
    }

    #[test]
    fn masks_crockford() {
        assert_eq!(
            encode(Crockford, &[0xF8, 0x3E, 0x0F, 0x83, 0xE0]),
            "Z0Z0Z0Z0"
        );
        assert_eq!(
            encode(Crockford, &[0x07, 0xC1, 0xF0, 0x7C, 0x1F]),
            "0Z0Z0Z0Z"
        );
        assert_eq!(
            decode(Crockford, "Z0Z0Z0Z0").unwrap(),
            [0xF8, 0x3E, 0x0F, 0x83, 0xE0]
        );
        assert_eq!(
            decode(Crockford, "0Z0Z0Z0Z").unwrap(),
            [0x07, 0xC1, 0xF0, 0x7C, 0x1F]
        );
    }

    #[test]
    fn masks_rfc4648() {
        assert_eq!(
            encode(RFC4648 { padding: true }, &[0xF8, 0x3E, 0x7F, 0x83, 0xE7]),
            "7A7H7A7H"
        );
        assert_eq!(
            encode(RFC4648 { padding: true }, &[0x77, 0xC1, 0xF7, 0x7C, 0x1F]),
            "O7A7O7A7"
        );
        assert_eq!(
            decode(RFC4648 { padding: true }, "7A7H7A7H").unwrap(),
            [0xF8, 0x3E, 0x7F, 0x83, 0xE7]
        );
        assert_eq!(
            decode(RFC4648 { padding: true }, "O7A7O7A7").unwrap(),
            [0x77, 0xC1, 0xF7, 0x7C, 0x1F]
        );
        assert_eq!(
            encode(RFC4648 { padding: true }, &[0xF8, 0x3E, 0x7F, 0x83]),
            "7A7H7AY="
        );
    }

    #[test]
    fn masks_unpadded_rfc4648() {
        assert_eq!(
            encode(RFC4648 { padding: false }, &[0xF8, 0x3E, 0x7F, 0x83, 0xE7]),
            "7A7H7A7H"
        );
        assert_eq!(
            encode(RFC4648 { padding: false }, &[0x77, 0xC1, 0xF7, 0x7C, 0x1F]),
            "O7A7O7A7"
        );
        assert_eq!(
            decode(RFC4648 { padding: false }, "7A7H7A7H").unwrap(),
            [0xF8, 0x3E, 0x7F, 0x83, 0xE7]
        );
        assert_eq!(
            decode(RFC4648 { padding: false }, "O7A7O7A7").unwrap(),
            [0x77, 0xC1, 0xF7, 0x7C, 0x1F]
        );
        assert_eq!(
            encode(RFC4648 { padding: false }, &[0xF8, 0x3E, 0x7F, 0x83]),
            "7A7H7AY"
        );
    }

    #[test]
    fn padding() {
        let num_padding = [0, 6, 4, 3, 1];
        for i in 1..6 {
            let encoded = encode(
                RFC4648 { padding: true },
                (0..(i as u8)).collect::<Vec<u8>>().as_ref(),
            );
            assert_eq!(encoded.len(), 8);
            for j in 0..(num_padding[i % 5]) {
                assert_eq!(encoded.as_bytes()[encoded.len() - j - 1], b'=');
            }
            for j in 0..(8 - num_padding[i % 5]) {
                assert!(encoded.as_bytes()[j] != b'=');
            }
        }
    }

    #[test]
    fn invertible_encore() {
        fn test(data: Vec<u8>) -> bool {
            decode(Encore, encode(Encore, data.as_ref()).as_ref()).unwrap() == data
        }
        quickcheck::quickcheck(test as fn(Vec<u8>) -> bool)
    }

    #[test]
    fn invertible_crockford() {
        fn test(data: Vec<u8>) -> bool {
            decode(Crockford, encode(Crockford, data.as_ref()).as_ref()).unwrap() == data
        }
        quickcheck::quickcheck(test as fn(Vec<u8>) -> bool)
    }

    #[test]
    fn invertible_rfc4648() {
        fn test(data: Vec<u8>) -> bool {
            decode(
                RFC4648 { padding: true },
                encode(RFC4648 { padding: true }, data.as_ref()).as_ref(),
            )
            .unwrap()
                == data
        }
        quickcheck::quickcheck(test as fn(Vec<u8>) -> bool)
    }

    #[test]
    fn invertible_unpadded_rfc4648() {
        fn test(data: Vec<u8>) -> bool {
            decode(
                RFC4648 { padding: false },
                encode(RFC4648 { padding: false }, data.as_ref()).as_ref(),
            )
            .unwrap()
                == data
        }
        quickcheck::quickcheck(test as fn(Vec<u8>) -> bool)
    }

    #[test]
    fn lower_case() {
        fn test(data: Vec<B32>) -> bool {
            let data: String = data.iter().map(|e| e.c as char).collect();
            decode(Crockford, data.as_ref())
                == decode(Crockford, data.to_ascii_lowercase().as_ref())
        }
        quickcheck::quickcheck(test as fn(Vec<B32>) -> bool)
    }

    #[test]
    #[allow(non_snake_case)]
    fn iIlL1_oO0() {
        assert_eq!(decode(Crockford, "IiLlOo"), decode(Crockford, "111100"));
    }

    #[test]
    fn invalid_chars_crockford() {
        assert_eq!(decode(Crockford, ","), None)
    }

    #[test]
    fn invalid_chars_rfc4648() {
        assert_eq!(decode(RFC4648 { padding: true }, ","), None)
    }

    #[test]
    fn invalid_chars_unpadded_rfc4648() {
        assert_eq!(decode(RFC4648 { padding: false }, ","), None)
    }
}
