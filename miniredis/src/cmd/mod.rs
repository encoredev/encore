// Command handler modules.
//
// Each module implements a category of Redis commands.
// Commands are registered in the dispatch table (src/dispatch.rs).

pub mod client; // CLIENT SETNAME/GETNAME
pub mod cluster; // CLUSTER SLOTS/KEYSLOT/NODES/SHARDS (mocked)
pub mod connection; // PING, ECHO, QUIT, SELECT, AUTH, HELLO
pub mod generic; // DEL, EXISTS, EXPIRE, TTL, KEYS, SCAN, etc.
pub mod geo; // GEOADD, GEODIST, GEOPOS, GEORADIUS, etc.
pub mod hash; // HSET, HGET, HDEL, HGETALL, etc.
pub mod hll; // PFADD, PFCOUNT, PFMERGE
pub mod list; // LPUSH, RPUSH, LPOP, RPOP, BLPOP, etc.
pub mod object;
pub mod pubsub; // SUBSCRIBE, PUBLISH, PSUBSCRIBE, etc.
pub mod scripting; // EVAL, EVALSHA, SCRIPT
pub mod server; // DBSIZE, FLUSHDB, INFO, TIME, etc.
pub mod set; // SADD, SREM, SMEMBERS, SINTER, etc.
pub mod sorted_set; // ZADD, ZRANGE, ZSCORE, ZRANK, etc.
pub mod stream; // XADD, XREAD, XREADGROUP, XACK, etc.
pub mod string; // GET, SET, MGET, MSET, INCR, etc.
pub mod transactions; // MULTI, EXEC, WATCH, DISCARD // OBJECT IDLETIME

pub(crate) fn parse_int(bytes: &[u8]) -> Option<i64> {
    String::from_utf8_lossy(bytes).parse::<i64>().ok()
}

pub(crate) fn parse_float(bytes: &[u8]) -> Option<f64> {
    let s = String::from_utf8_lossy(bytes);
    match s.to_lowercase().as_str() {
        "+inf" | "inf" => Some(f64::INFINITY),
        "-inf" => Some(f64::NEG_INFINITY),
        _ => s.parse::<f64>().ok(),
    }
}

use crate::dispatch::{MSG_INVALID_INT, MSG_SYNTAX_ERROR};
use crate::frame::Frame;

pub(crate) struct ScanOpts {
    pub pattern: Option<String>,
    pub count: Option<i64>,
    pub type_filter: Option<String>,
}

/// Parse MATCH/COUNT/TYPE options common to SCAN, SSCAN, HSCAN, ZSCAN.
/// `allow_type`: only SCAN supports the TYPE filter.
pub(crate) fn parse_scan_opts(args: &[Vec<u8>], allow_type: bool) -> Result<ScanOpts, Frame> {
    let mut pattern = None;
    let mut count = None;
    let mut type_filter = None;

    let mut i = 0;
    while i < args.len() {
        let opt = String::from_utf8_lossy(&args[i]).to_uppercase();
        match opt.as_str() {
            "MATCH" => {
                i += 1;
                if i >= args.len() {
                    return Err(Frame::error(MSG_SYNTAX_ERROR));
                }
                pattern = Some(String::from_utf8_lossy(&args[i]).into_owned());
            }
            "COUNT" => {
                i += 1;
                if i >= args.len() {
                    return Err(Frame::error(MSG_SYNTAX_ERROR));
                }
                let n = String::from_utf8_lossy(&args[i])
                    .parse::<i64>()
                    .map_err(|_| Frame::error(MSG_INVALID_INT))?;
                if n <= 0 {
                    return Err(Frame::error(MSG_SYNTAX_ERROR));
                }
                count = Some(n);
            }
            "TYPE" if allow_type => {
                i += 1;
                if i >= args.len() {
                    return Err(Frame::error(MSG_SYNTAX_ERROR));
                }
                type_filter = Some(String::from_utf8_lossy(&args[i]).to_lowercase());
            }
            _ => return Err(Frame::error(MSG_SYNTAX_ERROR)),
        }
        i += 1;
    }

    Ok(ScanOpts {
        pattern,
        count,
        type_filter,
    })
}
