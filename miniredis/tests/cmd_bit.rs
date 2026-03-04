// Ported from ../miniredis/cmd_string_test.go (bit operations)
mod helpers;

#[tokio::test]
async fn test_getbit() {
    let (_m, mut c) = helpers::start().await;

    // \x08 = 0b00001000
    let _: () = redis::cmd("SET")
        .arg("findme")
        .arg(b"\x08" as &[u8])
        .query_async(&mut c)
        .await
        .unwrap();

    must_int!(c, "GETBIT", "findme", "0"; 0);
    must_int!(c, "GETBIT", "findme", "4"; 1);
    must_int!(c, "GETBIT", "findme", "5"; 0);

    // Non-existing key
    must_int!(c, "GETBIT", "nosuch", "1"; 0);
    must_int!(c, "GETBIT", "nosuch", "1000"; 0);

    // Errors
    must_fail!(c, "GETBIT", "foo"; "wrong number of arguments");
    must_fail!(c, "GETBIT", "foo", "noint"; "not an integer");
    must_ok!(c, "SET", "str", "val");
    // Not wrong type for getbit — strings are valid
}

#[tokio::test]
async fn test_setbit() {
    let (_m, mut c) = helpers::start().await;

    // \x08 = 0b00001000
    let _: () = redis::cmd("SET")
        .arg("findme")
        .arg(b"\x08" as &[u8])
        .query_async(&mut c)
        .await
        .unwrap();

    // Clear bit 4 (was 1)
    must_int!(c, "SETBIT", "findme", "4", "0"; 1);

    // Set bit 4 (was 0)
    must_int!(c, "SETBIT", "findme", "4", "1"; 0);

    // Non-existing key — creates it
    must_int!(c, "SETBIT", "nosuch", "0", "1"; 0);
    let v: Vec<u8> = redis::cmd("GET")
        .arg("nosuch")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, vec![0x80]);

    // Extends short values
    let _: () = redis::cmd("SET")
        .arg("short")
        .arg(b"\x00\x00" as &[u8])
        .query_async(&mut c)
        .await
        .unwrap();
    must_int!(c, "SETBIT", "short", "32", "1"; 0);
    let v: Vec<u8> = redis::cmd("GET")
        .arg("short")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v.len(), 5);
    assert_eq!(v[4], 0x80);

    // Errors
    must_fail!(c, "SETBIT", "foo"; "wrong number of arguments");
    must_fail!(c, "SETBIT", "foo", "noint", "1"; "not an integer");
    must_fail!(c, "SETBIT", "foo", "1", "noint"; "not an integer");
    must_fail!(c, "SETBIT", "foo", "-3", "0"; "not an integer");
    must_fail!(c, "SETBIT", "foo", "3", "2"; "out of range");
}

#[tokio::test]
async fn test_bitcount() {
    let (_m, mut c) = helpers::start().await;

    // 'a' = 0x61 = 0b01100001 → 3 bits
    must_ok!(c, "SET", "countme", "a");
    must_int!(c, "BITCOUNT", "countme"; 3);

    // 'aaaaa' → 15 bits
    must_ok!(c, "SET", "countme", "aaaaa");
    must_int!(c, "BITCOUNT", "countme"; 15);

    // Non-existing
    must_int!(c, "BITCOUNT", "nosuch"; 0);

    // With range: 'abcd'
    // a=0x61(3) b=0x62(3) c=0x63(4) d=0x64(3)
    must_ok!(c, "SET", "foo", "abcd");
    must_int!(c, "BITCOUNT", "foo", "0", "0"; 3);
    must_int!(c, "BITCOUNT", "foo", "0", "3"; 13);
    must_int!(c, "BITCOUNT", "foo", "2", "-2"; 4); // only 'c'

    // Errors
    must_fail!(c, "BITCOUNT"; "wrong number of arguments");
    must_fail!(c, "BITCOUNT", "foo", "noint", "12"; "not an integer");
    must_fail!(c, "BITCOUNT", "foo", "12", "noint"; "not an integer");
}

#[tokio::test]
async fn test_bitop() {
    let (_m, mut c) = helpers::start().await;

    // AND
    must_ok!(c, "SET", "a", "a"); // 0x61
    must_ok!(c, "SET", "b", "b"); // 0x62
    must_int!(c, "BITOP", "AND", "bitand", "a", "b"; 1);
    let v: Vec<u8> = redis::cmd("GET")
        .arg("bitand")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, vec![0x60]); // '`'

    // AND with different lengths
    must_ok!(c, "SET", "a2", "aa");
    must_ok!(c, "SET", "b2", "bbbb");
    must_int!(c, "BITOP", "AND", "bitand2", "a2", "b2"; 4);
    let v: Vec<u8> = redis::cmd("GET")
        .arg("bitand2")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, vec![0x60, 0x60, 0x00, 0x00]);

    // OR
    must_int!(c, "BITOP", "OR", "bitor", "a2", "b2"; 4);
    let v: Vec<u8> = redis::cmd("GET")
        .arg("bitor")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, vec![0x63, 0x63, 0x62, 0x62]); // "ccbb"

    // XOR
    must_int!(c, "BITOP", "XOR", "bitxor", "a2", "b2"; 4);
    let v: Vec<u8> = redis::cmd("GET")
        .arg("bitxor")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, vec![0x03, 0x03, 0x62, 0x62]);

    // NOT
    must_int!(c, "BITOP", "NOT", "bitnot", "a"; 1);
    let v: Vec<u8> = redis::cmd("GET")
        .arg("bitnot")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, vec![0x9e]);

    // Single arg copy
    must_int!(c, "BITOP", "AND", "copy", "a"; 1);
    let v: String = redis::cmd("GET")
        .arg("copy")
        .query_async(&mut c)
        .await
        .unwrap();
    assert_eq!(v, "a");

    // Errors
    must_fail!(c, "BITOP"; "wrong number of arguments");
    must_fail!(c, "BITOP", "AND"; "wrong number of arguments");
    must_fail!(c, "BITOP", "WHAT", "dest", "key"; "syntax error");
    must_fail!(c, "BITOP", "NOT", "foo", "bar", "baz"; "BITOP NOT");
}

#[tokio::test]
async fn test_bitpos() {
    let (_m, mut c) = helpers::start().await;

    // \xff\xf0\x00 = all 1s, 4 more 1s, all 0s
    let _: () = redis::cmd("SET")
        .arg("findme")
        .arg(b"\xff\xf0\x00" as &[u8])
        .query_async(&mut c)
        .await
        .unwrap();

    must_int!(c, "BITPOS", "findme", "0"; 12);
    must_int!(c, "BITPOS", "findme", "1"; 0);
    must_int!(c, "BITPOS", "findme", "1", "1"; 8);
    must_int!(c, "BITPOS", "findme", "0", "1"; 12);

    // Only zeros
    let _: () = redis::cmd("SET")
        .arg("zero")
        .arg(b"\x00\x00" as &[u8])
        .query_async(&mut c)
        .await
        .unwrap();
    must_int!(c, "BITPOS", "zero", "1"; -1);
    must_int!(c, "BITPOS", "zero", "0"; 0);

    // Only ones
    let _: () = redis::cmd("SET")
        .arg("one")
        .arg(b"\xff\xff" as &[u8])
        .query_async(&mut c)
        .await
        .unwrap();
    must_int!(c, "BITPOS", "one", "1"; 0);
    must_int!(c, "BITPOS", "one", "1", "1"; 8);
    must_int!(c, "BITPOS", "one", "0"; 16); // special: past the end

    // Non-existing
    must_int!(c, "BITPOS", "nosuch", "1"; -1);
    must_int!(c, "BITPOS", "nosuch", "0"; 0);

    // Errors
    must_fail!(c, "BITPOS"; "wrong number of arguments");
    must_fail!(c, "BITPOS", "foo"; "wrong number of arguments");
    must_fail!(c, "BITPOS", "foo", "noint"; "not an integer");
}
