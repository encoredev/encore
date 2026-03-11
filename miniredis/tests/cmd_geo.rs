mod helpers;
use helpers::*;

// ── GEOADD ───────────────────────────────────────────────────────────

#[tokio::test]
async fn test_geoadd() {
    let (_m, mut c) = start().await;

    // Add two locations
    must_int!(c, "GEOADD", "Sicily", "13.361389", "38.115556", "Palermo"; 1);
    must_int!(c, "GEOADD", "Sicily", "15.087269", "37.502669", "Catania"; 1);

    // Re-add same member → 0 (updated, not new)
    must_int!(c, "GEOADD", "Sicily", "13.361389", "38.115556", "Palermo"; 0);
}

#[tokio::test]
async fn test_geoadd_errors() {
    let (_m, mut c) = start().await;

    // Wrong number of args
    must_fail!(c, "GEOADD"; "wrong number of arguments");
    must_fail!(c, "GEOADD", "key"; "wrong number of arguments");
    must_fail!(c, "GEOADD", "key", "1", "2"; "wrong number of arguments");

    // Invalid longitude (out of range)
    must_fail!(c, "GEOADD", "broken", "-190.0", "10.0", "hi"; "invalid longitude,latitude pair");
    must_fail!(c, "GEOADD", "broken", "190.0", "10.0", "hi"; "invalid longitude,latitude pair");

    // Invalid latitude (out of range)
    must_fail!(c, "GEOADD", "broken", "10.0", "-86.0", "hi"; "invalid longitude,latitude pair");
    must_fail!(c, "GEOADD", "broken", "10.0", "86.0", "hi"; "invalid longitude,latitude pair");

    // Not a float
    must_fail!(c, "GEOADD", "broken", "notafloat", "10.0", "hi"; "not a valid float");
    must_fail!(c, "GEOADD", "broken", "10.0", "notafloat", "hi"; "not a valid float");
}

// ── GEOPOS ───────────────────────────────────────────────────────────

#[tokio::test]
async fn test_geopos() {
    let (_m, mut c) = start().await;

    must_int!(c, "GEOADD", "Sicily", "13.361389", "38.115556", "Palermo"; 1);

    // Get position — returns array of [lng, lat]
    let v: redis::Value = redis::cmd("GEOPOS")
        .arg("Sicily")
        .arg("Palermo")
        .query_async(&mut c)
        .await
        .unwrap();

    match v {
        redis::Value::Array(ref items) => {
            assert_eq!(items.len(), 1);
            // First element should be an array of [lng, lat]
            match &items[0] {
                redis::Value::Array(coords) => {
                    assert_eq!(coords.len(), 2);
                }
                _ => panic!("expected array for coords, got {:?}", items[0]),
            }
        }
        _ => panic!("expected array from GEOPOS, got {:?}", v),
    }

    // Non-existent member → nil
    let v: redis::Value = redis::cmd("GEOPOS")
        .arg("Sicily")
        .arg("Corleone")
        .query_async(&mut c)
        .await
        .unwrap();

    match v {
        redis::Value::Array(ref items) => {
            assert_eq!(items.len(), 1);
            assert_eq!(items[0], redis::Value::Nil);
        }
        _ => panic!("expected array from GEOPOS, got {:?}", v),
    }
}

#[tokio::test]
async fn test_geopos_errors() {
    let (_m, mut c) = start().await;

    must_fail!(c, "GEOPOS"; "wrong number of arguments");

    must_ok!(c, "SET", "foo", "bar");
    must_fail!(c, "GEOPOS", "foo"; "WRONGTYPE");
}

// ── GEODIST ──────────────────────────────────────────────────────────

#[tokio::test]
async fn test_geodist() {
    let (_m, mut c) = start().await;

    must_int!(c, "GEOADD", "Sicily", "13.361389", "38.115556", "Palermo"; 1);
    must_int!(c, "GEOADD", "Sicily", "15.087269", "37.502669", "Catania"; 1);

    // Default unit = meters
    let d: String = redis::cmd("GEODIST")
        .arg("Sicily")
        .arg("Palermo")
        .arg("Catania")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(d, "166274.1514");

    // In km
    let d: String = redis::cmd("GEODIST")
        .arg("Sicily")
        .arg("Palermo")
        .arg("Catania")
        .arg("km")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(d, "166.2742");
}

#[tokio::test]
async fn test_geodist_nil() {
    let (_m, mut c) = start().await;

    must_int!(c, "GEOADD", "Sicily", "13.361389", "38.115556", "Palermo"; 1);

    // Non-existent key
    must_nil!(c, "GEODIST", "nosuch", "a", "b");

    // Non-existent member
    must_nil!(c, "GEODIST", "Sicily", "Palermo", "nosuch");
    must_nil!(c, "GEODIST", "Sicily", "nosuch", "Palermo");
}

#[tokio::test]
async fn test_geodist_errors() {
    let (_m, mut c) = start().await;

    must_int!(c, "GEOADD", "Sicily", "13.361389", "38.115556", "Palermo"; 1);
    must_int!(c, "GEOADD", "Sicily", "15.087269", "37.502669", "Catania"; 1);

    must_fail!(c, "GEODIST"; "wrong number of arguments");
    must_fail!(c, "GEODIST", "Sicily"; "wrong number of arguments");
    must_fail!(c, "GEODIST", "Sicily", "Palermo"; "wrong number of arguments");

    // Unsupported unit
    must_fail!(c, "GEODIST", "Sicily", "Palermo", "Catania", "miles"; "unsupported unit");

    // Too many args
    must_fail!(c, "GEODIST", "Sicily", "Palermo", "Catania", "m", "extra"; "syntax error");

    // Wrong type
    must_ok!(c, "SET", "foo", "bar");
    must_fail!(c, "GEODIST", "foo", "Palermo", "Catania"; "WRONGTYPE");
}

// ── GEORADIUS ────────────────────────────────────────────────────────

#[tokio::test]
async fn test_georadius_basic() {
    let (_m, mut c) = start().await;

    must_int!(c, "GEOADD", "Sicily", "13.361389", "38.115556", "Palermo"; 1);
    must_int!(c, "GEOADD", "Sicily", "15.087269", "37.502669", "Catania"; 1);

    // Basic radius query — returns member names
    let v: Vec<String> = redis::cmd("GEORADIUS")
        .arg("Sicily")
        .arg("15")
        .arg("37")
        .arg("200")
        .arg("km")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v.len(), 2);
    assert!(v.contains(&"Palermo".to_string()));
    assert!(v.contains(&"Catania".to_string()));

    // Too small radius — no results
    let v: Vec<String> = redis::cmd("GEORADIUS")
        .arg("Sicily")
        .arg("15")
        .arg("37")
        .arg("1")
        .arg("km")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v.len(), 0);
}

#[tokio::test]
async fn test_georadius_asc_desc() {
    let (_m, mut c) = start().await;

    must_int!(c, "GEOADD", "Sicily", "13.361389", "38.115556", "Palermo"; 1);
    must_int!(c, "GEOADD", "Sicily", "15.087269", "37.502669", "Catania"; 1);

    // ASC
    let v: Vec<String> = redis::cmd("GEORADIUS")
        .arg("Sicily")
        .arg("15")
        .arg("37")
        .arg("200")
        .arg("km")
        .arg("ASC")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, vec!["Catania", "Palermo"]);

    // DESC
    let v: Vec<String> = redis::cmd("GEORADIUS")
        .arg("Sicily")
        .arg("15")
        .arg("37")
        .arg("200")
        .arg("km")
        .arg("DESC")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, vec!["Palermo", "Catania"]);
}

#[tokio::test]
async fn test_georadius_count() {
    let (_m, mut c) = start().await;

    must_int!(c, "GEOADD", "Sicily", "13.361389", "38.115556", "Palermo"; 1);
    must_int!(c, "GEOADD", "Sicily", "15.087269", "37.502669", "Catania"; 1);

    // COUNT 1 + ASC
    let v: Vec<String> = redis::cmd("GEORADIUS")
        .arg("Sicily")
        .arg("15")
        .arg("37")
        .arg("200")
        .arg("km")
        .arg("ASC")
        .arg("COUNT")
        .arg("1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, vec!["Catania"]);

    // COUNT errors
    must_fail!(c, "GEORADIUS", "Sicily", "15", "37", "200", "km", "COUNT"; "syntax error");
    must_fail!(c, "GEORADIUS", "Sicily", "15", "37", "200", "km", "COUNT", "notanumber"; "not an integer");
    must_fail!(c, "GEORADIUS", "Sicily", "15", "37", "200", "km", "COUNT", "-12"; "COUNT must be > 0");
}

#[tokio::test]
async fn test_georadius_withdist() {
    let (_m, mut c) = start().await;

    must_int!(c, "GEOADD", "Sicily", "13.361389", "38.115556", "Palermo"; 1);
    must_int!(c, "GEOADD", "Sicily", "15.087269", "37.502669", "Catania"; 1);

    // WITHDIST in km
    let v: redis::Value = redis::cmd("GEORADIUS")
        .arg("Sicily")
        .arg("15")
        .arg("37")
        .arg("200")
        .arg("km")
        .arg("WITHDIST")
        .query_async(&mut c)
        .await
        .unwrap();

    match v {
        redis::Value::Array(ref items) => {
            assert_eq!(items.len(), 2);
        }
        _ => panic!("expected array from GEORADIUS WITHDIST, got {:?}", v),
    }
}

#[tokio::test]
async fn test_georadius_errors() {
    let (_m, mut c) = start().await;

    // Invalid unit
    must_fail!(c, "GEORADIUS", "Sicily", "15", "37", "200", "mm"; "wrong number of arguments");

    // Invalid float params
    must_fail!(c, "GEORADIUS", "Sicily", "abc", "def", "ghi", "m"; "wrong number of arguments");
}

#[tokio::test]
async fn test_georadius_ro() {
    let (_m, mut c) = start().await;

    must_int!(c, "GEOADD", "Sicily", "13.361389", "38.115556", "Palermo"; 1);
    must_int!(c, "GEOADD", "Sicily", "15.087269", "37.502669", "Catania"; 1);

    // GEORADIUS_RO works
    let v: Vec<String> = redis::cmd("GEORADIUS_RO")
        .arg("Sicily")
        .arg("15")
        .arg("37")
        .arg("200")
        .arg("km")
        .arg("ASC")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, vec!["Catania", "Palermo"]);

    // STORE not allowed in RO mode
    must_fail!(c, "GEORADIUS_RO", "Sicily", "15", "37", "200", "km", "STORE", "foo"; "syntax error");
    must_fail!(c, "GEORADIUS_RO", "Sicily", "15", "37", "200", "km", "STOREDIST", "foo"; "syntax error");
}

// ── GEORADIUSBYMEMBER ────────────────────────────────────────────────

#[tokio::test]
async fn test_georadiusbymember_basic() {
    let (_m, mut c) = start().await;

    must_int!(c, "GEOADD", "Sicily", "13.361389", "38.115556", "Palermo"; 1);
    must_int!(c, "GEOADD", "Sicily", "15.087269", "37.502669", "Catania"; 1);

    // Basic query
    let v: Vec<String> = redis::cmd("GEORADIUSBYMEMBER")
        .arg("Sicily")
        .arg("Palermo")
        .arg("200")
        .arg("km")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v.len(), 2);

    // ASC
    let v: Vec<String> = redis::cmd("GEORADIUSBYMEMBER")
        .arg("Sicily")
        .arg("Palermo")
        .arg("200")
        .arg("km")
        .arg("ASC")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, vec!["Palermo", "Catania"]);

    // DESC
    let v: Vec<String> = redis::cmd("GEORADIUSBYMEMBER")
        .arg("Sicily")
        .arg("Palermo")
        .arg("200")
        .arg("km")
        .arg("DESC")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, vec!["Catania", "Palermo"]);
}

#[tokio::test]
async fn test_georadiusbymember_count() {
    let (_m, mut c) = start().await;

    must_int!(c, "GEOADD", "Sicily", "13.361389", "38.115556", "Palermo"; 1);
    must_int!(c, "GEOADD", "Sicily", "15.087269", "37.502669", "Catania"; 1);

    let v: Vec<String> = redis::cmd("GEORADIUSBYMEMBER")
        .arg("Sicily")
        .arg("Palermo")
        .arg("200")
        .arg("km")
        .arg("ASC")
        .arg("COUNT")
        .arg("1")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, vec!["Palermo"]);
}

#[tokio::test]
async fn test_georadiusbymember_missing() {
    let (_m, mut c) = start().await;

    must_int!(c, "GEOADD", "Sicily", "13.361389", "38.115556", "Palermo"; 1);

    // Non-existent key → nil
    must_nil!(c, "GEORADIUSBYMEMBER", "Capri", "Palermo", "200", "km");

    // Missing member → error
    must_fail!(c, "GEORADIUSBYMEMBER", "Sicily", "nosuch", "200", "km"; "could not decode requested zset member");

    // Invalid unit
    must_fail!(c, "GEORADIUSBYMEMBER", "Sicily", "Palermo", "200", "mm"; "wrong number of arguments");
}

#[tokio::test]
async fn test_georadiusbymember_ro() {
    let (_m, mut c) = start().await;

    must_int!(c, "GEOADD", "Sicily", "13.361389", "38.115556", "Palermo"; 1);
    must_int!(c, "GEOADD", "Sicily", "15.087269", "37.502669", "Catania"; 1);

    // GEORADIUSBYMEMBER_RO works
    let v: Vec<String> = redis::cmd("GEORADIUSBYMEMBER_RO")
        .arg("Sicily")
        .arg("Palermo")
        .arg("200")
        .arg("km")
        .arg("ASC")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, vec!["Palermo", "Catania"]);

    // STORE not allowed
    must_fail!(c, "GEORADIUSBYMEMBER_RO", "Sicily", "Palermo", "200", "km", "STORE", "foo"; "syntax error");
    must_fail!(c, "GEORADIUSBYMEMBER_RO", "Sicily", "Palermo", "200", "km", "STOREDIST", "foo"; "syntax error");
}
