/// Geospatial encoding/decoding and distance calculations.
///
/// Implements 52-bit integer geohash encoding (matching Redis's geohash_helper.c)
/// and Haversine distance formula.
use std::f64::consts::PI;

const ENC_LAT: f64 = 85.05112878;
const ENC_LONG: f64 = 180.0;
const EXP2_32: f64 = 4294967296.0; // 2^32

/// Earth radius in meters (matching Redis src/geohash_helper.c).
const EARTH_RADIUS: f64 = 6372797.560856;

// ── Range encoding ──────────────────────────────────────────────────

/// Encode the position of x within the range [-r, r] as a 32-bit integer.
fn encode_range(x: f64, r: f64) -> u32 {
    let p = (x + r) / (2.0 * r);
    (p * EXP2_32) as u32
}

/// Decode the 32-bit range encoding back to a value in [-r, r].
fn decode_range(x: u32, r: f64) -> f64 {
    let p = x as f64 / EXP2_32;
    2.0 * r * p - r
}

// ── Bit interleaving ────────────────────────────────────────────────

/// Spread 32 bits into the even bit positions of a 64-bit word.
fn spread(x: u32) -> u64 {
    let mut v = x as u64;
    v = (v | (v << 16)) & 0x0000ffff0000ffff;
    v = (v | (v << 8)) & 0x00ff00ff00ff00ff;
    v = (v | (v << 4)) & 0x0f0f0f0f0f0f0f0f;
    v = (v | (v << 2)) & 0x3333333333333333;
    v = (v | (v << 1)) & 0x5555555555555555;
    v
}

/// Squash the even bit positions of a 64-bit word into 32 bits.
fn squash(x: u64) -> u32 {
    let mut v = x & 0x5555555555555555;
    v = (v | (v >> 1)) & 0x3333333333333333;
    v = (v | (v >> 2)) & 0x0f0f0f0f0f0f0f0f;
    v = (v | (v >> 4)) & 0x00ff00ff00ff00ff;
    v = (v | (v >> 8)) & 0x0000ffff0000ffff;
    v = (v | (v >> 16)) & 0x00000000ffffffff;
    v as u32
}

/// Interleave the bits of x (lat) and y (lng). x occupies even bit positions,
/// y occupies odd bit positions.
fn interleave(x: u32, y: u32) -> u64 {
    spread(x) | (spread(y) << 1)
}

/// Deinterleave: extract even and odd bit positions into two 32-bit words.
fn deinterleave(v: u64) -> (u32, u32) {
    (squash(v), squash(v >> 1))
}

// ── Geohash encode/decode ───────────────────────────────────────────

/// Encode latitude and longitude into a full 64-bit integer geohash.
fn encode_int(lat: f64, lng: f64) -> u64 {
    let lat_int = encode_range(lat, ENC_LAT);
    let lng_int = encode_range(lng, ENC_LONG);
    interleave(lat_int, lng_int)
}

/// Encode coordinates as a 52-bit geohash (stored as the upper 52 bits of
/// a 64-bit value, i.e. right-shifted by 12).
pub fn to_geohash(longitude: f64, latitude: f64) -> u64 {
    encode_int(latitude, longitude) >> (64 - 52)
}

/// Decode a 52-bit geohash back to (longitude, latitude).
pub fn from_geohash(hash: u64) -> (f64, f64) {
    let full_hash = hash << (64 - 52);
    let (lat_int, lng_int) = deinterleave(full_hash);
    let lat = decode_range(lat_int, ENC_LAT);
    let lng = decode_range(lng_int, ENC_LONG);
    // Bounding box center: add half the error
    let lat_bits = 52 / 2; // 26
    let lng_bits = 52 - lat_bits; // 26
    let lat_err = 180.0 * 2.0f64.powi(-lat_bits);
    let lng_err = 360.0 * 2.0f64.powi(-lng_bits);
    (lng + lng_err / 2.0, lat + lat_err / 2.0)
}

// ── Haversine distance ──────────────────────────────────────────────

/// Haversine helper: sin²(θ/2).
fn hsin(theta: f64) -> f64 {
    let s = (theta / 2.0).sin();
    s * s
}

/// Calculate the great-circle distance in meters between two points given
/// as (latitude, longitude) in degrees, using the Haversine formula.
pub fn haversine_distance(lat1: f64, lon1: f64, lat2: f64, lon2: f64) -> f64 {
    let la1 = lat1 * PI / 180.0;
    let lo1 = lon1 * PI / 180.0;
    let la2 = lat2 * PI / 180.0;
    let lo2 = lon2 * PI / 180.0;

    let h = hsin(la2 - la1) + la1.cos() * la2.cos() * hsin(lo2 - lo1);
    2.0 * EARTH_RADIUS * h.sqrt().asin()
}

// ── Unit conversion ─────────────────────────────────────────────────

/// Parse a distance unit string and return the conversion factor to meters.
/// Returns None for unrecognized units.
pub fn parse_unit(unit: &str) -> Option<f64> {
    match unit.to_lowercase().as_str() {
        "m" => Some(1.0),
        "km" => Some(1000.0),
        "mi" => Some(1609.34),
        "ft" => Some(0.3048),
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_geohash_roundtrip() {
        let lng = 13.361_389_338_970_184;
        let lat = 38.115_556_395_496_3;
        let hash = to_geohash(lng, lat);
        assert_eq!(hash, 3479099956230698);

        let (lng_back, lat_back) = from_geohash(hash);
        assert!(
            (lng - lng_back).abs() < 0.000001,
            "longitude: {} vs {}",
            lng,
            lng_back
        );
        assert!(
            (lat - lat_back).abs() < 0.000001,
            "latitude: {} vs {}",
            lat,
            lat_back
        );
    }

    #[test]
    fn test_haversine_palermo_catania() {
        // Palermo: 13.361389, 38.115556
        // Catania: 15.087269, 37.502669
        let d = haversine_distance(38.115556, 13.361389, 37.502669, 15.087269);
        // Expected ~166274 meters
        assert!((d - 166274.0).abs() < 100.0, "distance: {}", d);
    }

    #[test]
    fn test_parse_unit() {
        assert_eq!(parse_unit("m"), Some(1.0));
        assert_eq!(parse_unit("km"), Some(1000.0));
        assert_eq!(parse_unit("mi"), Some(1609.34));
        assert_eq!(parse_unit("ft"), Some(0.3048));
        assert_eq!(parse_unit("mm"), None);
        assert_eq!(parse_unit("M"), Some(1.0));
        assert_eq!(parse_unit("KM"), Some(1000.0));
    }
}
