use std::sync::Arc;

use crate::connection::ConnCtx;
use crate::db::SharedState;
use crate::dispatch::{
    CommandTable, MSG_INVALID_INT, MSG_SYNTAX_ERROR, MSG_WRONG_TYPE, err_wrong_number,
};
use crate::frame::Frame;
use crate::geo::{from_geohash, haversine_distance, parse_unit, to_geohash};
use crate::types::{Direction, KeyType};

const MSG_UNSUPPORTED_UNIT: &str = "ERR unsupported unit provided. please use M, KM, FT, MI";

pub fn register(table: &mut CommandTable) {
    table.add("GEOADD", cmd_geoadd, false, -5);
    table.add("GEODIST", cmd_geodist, true, -4);
    table.add("GEOPOS", cmd_geopos, true, -2);
    table.add("GEORADIUS", cmd_georadius, false, -6);
    table.add("GEORADIUS_RO", cmd_georadius_ro, true, -6);
    table.add("GEORADIUSBYMEMBER", cmd_georadiusbymember, false, -5);
    table.add("GEORADIUSBYMEMBER_RO", cmd_georadiusbymember_ro, true, -5);
}

/// GEOADD key longitude latitude member [longitude latitude member ...]
fn cmd_geoadd(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = to_str(&args[0]);
    let triplets = &args[1..];

    if !triplets.len().is_multiple_of(3) {
        return Frame::error(err_wrong_number("geoadd"));
    }

    let mut entries = Vec::new();
    let mut i = 0;
    while i + 2 < triplets.len() {
        let raw_long = to_str(&triplets[i]);
        let raw_lat = to_str(&triplets[i + 1]);
        let name = to_str(&triplets[i + 2]);
        i += 3;

        let longitude: f64 = match raw_long.parse() {
            Ok(v) => v,
            Err(_) => return Frame::error("ERR value is not a valid float"),
        };
        let latitude: f64 = match raw_lat.parse() {
            Ok(v) => v,
            Err(_) => return Frame::error("ERR value is not a valid float"),
        };

        if !(-85.05112878..=85.05112878).contains(&latitude)
            || !(-180.0..=180.0).contains(&longitude)
        {
            return Frame::error(format!(
                "ERR invalid longitude,latitude pair {:.6},{:.6}",
                longitude, latitude
            ));
        }

        let score = to_geohash(longitude, latitude) as f64;
        entries.push((name, score));
    }

    let mut inner = state.lock();
    let now = inner.effective_now();
    let db = inner.db_mut(ctx.selected_db);

    if let Some(kt) = db.keys.get(&key)
        && *kt != KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let mut added = 0i64;
    for (name, score) in &entries {
        if db.sset_add(&key, *score, name, now) {
            added += 1;
        }
    }

    Frame::Integer(added)
}

/// GEODIST key member1 member2 [unit]
fn cmd_geodist(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = to_str(&args[0]);
    let from = to_str(&args[1]);
    let to = to_str(&args[2]);
    let remaining = &args[3..];

    let unit = if !remaining.is_empty() {
        to_str(&remaining[0])
    } else {
        "m".to_string()
    };

    if remaining.len() > 1 {
        return Frame::error(MSG_SYNTAX_ERROR);
    }

    let to_meter = match parse_unit(&unit) {
        Some(v) => v,
        None => return Frame::error(MSG_UNSUPPORTED_UNIT),
    };

    let inner = state.lock();
    let db = inner.db(ctx.selected_db);

    if !db.keys.contains_key(&key) {
        return Frame::Null;
    }
    if db.keys.get(&key) != Some(&KeyType::SortedSet) {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let from_score = match db.sset_score(&key, &from) {
        Some(s) => s,
        None => return Frame::Null,
    };
    let to_score = match db.sset_score(&key, &to) {
        Some(s) => s,
        None => return Frame::Null,
    };

    let (from_lng, from_lat) = from_geohash(from_score as u64);
    let (to_lng, to_lat) = from_geohash(to_score as u64);

    let dist = haversine_distance(from_lat, from_lng, to_lat, to_lng) / to_meter;
    Frame::Bulk(format!("{:.4}", dist).into())
}

/// GEOPOS key member [member ...]
fn cmd_geopos(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    let key = to_str(&args[0]);
    let members = &args[1..];

    let inner = state.lock();
    let db = inner.db(ctx.selected_db);

    if let Some(kt) = db.keys.get(&key)
        && *kt != KeyType::SortedSet
    {
        return Frame::error(MSG_WRONG_TYPE);
    }

    let mut results = Vec::with_capacity(members.len());
    for member_arg in members {
        let member = to_str(member_arg);
        match db.sset_score(&key, &member) {
            Some(score) => {
                let (lng, lat) = from_geohash(score as u64);
                results.push(Frame::Array(vec![
                    Frame::Bulk(format!("{:.6}", lng).into()),
                    Frame::Bulk(format!("{:.6}", lat).into()),
                ]));
            }
            None => {
                results.push(Frame::NullArray);
            }
        }
    }

    Frame::Array(results)
}

// ── Shared radius search types and helpers ──────────────────────────

struct GeoMatch {
    name: String,
    score: f64,
    distance: f64,
    longitude: f64,
    latitude: f64,
}

#[derive(PartialEq)]
enum SortDir {
    Unsorted,
    Asc,
    Desc,
}

struct RadiusOpts {
    with_dist: bool,
    with_coord: bool,
    direction: SortDir,
    count: usize,
    store_key: Option<String>,
    storedist_key: Option<String>,
}

fn within_radius(
    state: &Arc<SharedState>,
    db_idx: usize,
    key: &str,
    longitude: f64,
    latitude: f64,
    radius_meters: f64,
) -> Vec<GeoMatch> {
    let inner = state.lock();
    let db = inner.db(db_idx);

    let ss = match db.sorted_set_keys.get(key) {
        Some(ss) => ss,
        None => return Vec::new(),
    };

    let elems = ss.by_score(Direction::Asc);
    let mut matches = Vec::new();
    for el in &elems {
        let (el_lng, el_lat) = from_geohash(el.score as u64);
        let d = haversine_distance(latitude, longitude, el_lat, el_lng);
        if d <= radius_meters {
            matches.push(GeoMatch {
                name: el.member.clone(),
                score: el.score,
                distance: d,
                longitude: el_lng,
                latitude: el_lat,
            });
        }
    }
    matches
}

fn parse_radius_opts(args: &[Vec<u8>], read_only: bool) -> Result<RadiusOpts, Frame> {
    let mut opts = RadiusOpts {
        with_dist: false,
        with_coord: false,
        direction: SortDir::Unsorted,
        count: 0,
        store_key: None,
        storedist_key: None,
    };

    let mut i = 0;
    while i < args.len() {
        let arg = to_str(&args[i]).to_uppercase();
        match arg.as_str() {
            "WITHCOORD" => opts.with_coord = true,
            "WITHDIST" => opts.with_dist = true,
            "ASC" => opts.direction = SortDir::Asc,
            "DESC" => opts.direction = SortDir::Desc,
            "COUNT" => {
                i += 1;
                if i >= args.len() {
                    return Err(Frame::error(MSG_SYNTAX_ERROR));
                }
                let n: i64 = match to_str(&args[i]).parse() {
                    Ok(v) => v,
                    Err(_) => return Err(Frame::error(MSG_INVALID_INT)),
                };
                if n <= 0 {
                    return Err(Frame::error("ERR COUNT must be > 0"));
                }
                opts.count = n as usize;
            }
            "STORE" => {
                if read_only {
                    return Err(Frame::error(MSG_SYNTAX_ERROR));
                }
                i += 1;
                if i >= args.len() {
                    return Err(Frame::error(MSG_SYNTAX_ERROR));
                }
                opts.store_key = Some(to_str(&args[i]));
            }
            "STOREDIST" => {
                if read_only {
                    return Err(Frame::error(MSG_SYNTAX_ERROR));
                }
                i += 1;
                if i >= args.len() {
                    return Err(Frame::error(MSG_SYNTAX_ERROR));
                }
                opts.storedist_key = Some(to_str(&args[i]));
            }
            _ => return Err(Frame::error(MSG_SYNTAX_ERROR)),
        }
        i += 1;
    }

    Ok(opts)
}

fn format_radius_results(matches: &[GeoMatch], opts: &RadiusOpts, to_meter: f64) -> Frame {
    let mut frames = Vec::with_capacity(matches.len());
    for m in matches {
        if !opts.with_dist && !opts.with_coord {
            frames.push(Frame::bulk_string(&m.name));
        } else {
            let mut inner = Vec::new();
            inner.push(Frame::bulk_string(&m.name));
            if opts.with_dist {
                inner.push(Frame::Bulk(format!("{:.4}", m.distance / to_meter).into()));
            }
            if opts.with_coord {
                inner.push(Frame::Array(vec![
                    Frame::Bulk(format!("{:.6}", m.longitude).into()),
                    Frame::Bulk(format!("{:.6}", m.latitude).into()),
                ]));
            }
            frames.push(Frame::Array(inner));
        }
    }
    Frame::Array(frames)
}

fn apply_sort_and_count(matches: &mut Vec<GeoMatch>, opts: &RadiusOpts) {
    if opts.direction != SortDir::Unsorted {
        matches.sort_by(|a, b| {
            if opts.direction == SortDir::Desc {
                b.distance
                    .partial_cmp(&a.distance)
                    .unwrap_or(std::cmp::Ordering::Equal)
            } else {
                a.distance
                    .partial_cmp(&b.distance)
                    .unwrap_or(std::cmp::Ordering::Equal)
            }
        });
    }
    if opts.count > 0 && matches.len() > opts.count {
        matches.truncate(opts.count);
    }
}

// ── GEORADIUS / GEORADIUS_RO ────────────────────────────────────────

fn cmd_georadius_impl(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    args: &[Vec<u8>],
    read_only: bool,
    cmd_name: &str,
) -> Frame {
    let key = to_str(&args[0]);

    let longitude: f64 = match to_str(&args[1]).parse() {
        Ok(v) => v,
        Err(_) => return Frame::error(err_wrong_number(cmd_name)),
    };
    let latitude: f64 = match to_str(&args[2]).parse() {
        Ok(v) => v,
        Err(_) => return Frame::error(err_wrong_number(cmd_name)),
    };
    let radius: f64 = match to_str(&args[3]).parse() {
        Ok(v) if v >= 0.0 => v,
        _ => return Frame::error(err_wrong_number(cmd_name)),
    };
    let to_meter = match parse_unit(&to_str(&args[4])) {
        Some(v) => v,
        None => return Frame::error(err_wrong_number(cmd_name)),
    };

    let opts = match parse_radius_opts(&args[5..], read_only) {
        Ok(o) => o,
        Err(e) => return e,
    };

    // Check STORE/STOREDIST incompatibility with WITHDIST/WITHCOORD
    if (opts.store_key.is_some() || opts.storedist_key.is_some())
        && (opts.with_dist || opts.with_coord)
    {
        return Frame::error(
            "ERR STORE option in GEORADIUS is not compatible with WITHDIST, WITHHASH and WITHCOORDS options",
        );
    }

    let mut matches = within_radius(
        state,
        ctx.selected_db,
        &key,
        longitude,
        latitude,
        radius * to_meter,
    );
    apply_sort_and_count(&mut matches, &opts);

    // Handle STORE
    if let Some(ref store_key) = opts.store_key {
        let mut inner = state.lock();
        let now = inner.effective_now();
        let db = inner.db_mut(ctx.selected_db);
        db.del(store_key);
        for m in &matches {
            db.sset_add(store_key, m.score, &m.name, now);
        }
        return Frame::Integer(matches.len() as i64);
    }

    // Handle STOREDIST
    if let Some(ref storedist_key) = opts.storedist_key {
        let mut inner = state.lock();
        let now = inner.effective_now();
        let db = inner.db_mut(ctx.selected_db);
        db.del(storedist_key);
        for m in &matches {
            db.sset_add(storedist_key, m.distance / to_meter, &m.name, now);
        }
        return Frame::Integer(matches.len() as i64);
    }

    format_radius_results(&matches, &opts, to_meter)
}

fn cmd_georadius(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_georadius_impl(state, ctx, args, false, "georadius")
}

fn cmd_georadius_ro(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_georadius_impl(state, ctx, args, true, "georadius_ro")
}

// ── GEORADIUSBYMEMBER / GEORADIUSBYMEMBER_RO ────────────────────────

fn cmd_georadiusbymember_impl(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    args: &[Vec<u8>],
    read_only: bool,
    cmd_name: &str,
) -> Frame {
    let key = to_str(&args[0]);
    let member = to_str(&args[1]);

    let radius: f64 = match to_str(&args[2]).parse() {
        Ok(v) if v >= 0.0 => v,
        _ => return Frame::error(err_wrong_number(cmd_name)),
    };
    let to_meter = match parse_unit(&to_str(&args[3])) {
        Some(v) => v,
        None => return Frame::error(err_wrong_number(cmd_name)),
    };

    let opts = match parse_radius_opts(&args[4..], read_only) {
        Ok(o) => o,
        Err(e) => return e,
    };

    // Check STORE/STOREDIST incompatibility
    if (opts.store_key.is_some() || opts.storedist_key.is_some())
        && (opts.with_dist || opts.with_coord)
    {
        return Frame::error(
            "ERR STORE option in GEORADIUS is not compatible with WITHDIST, WITHHASH and WITHCOORDS options",
        );
    }

    // Look up the member's coordinates
    {
        let inner = state.lock();
        let db = inner.db(ctx.selected_db);

        if !db.keys.contains_key(&key) {
            return Frame::Null;
        }
        if db.keys.get(&key) != Some(&KeyType::SortedSet) {
            return Frame::error(MSG_WRONG_TYPE);
        }

        match db.sset_score(&key, &member) {
            Some(score) => {
                let (longitude, latitude) = from_geohash(score as u64);
                drop(inner);

                let mut matches = within_radius(
                    state,
                    ctx.selected_db,
                    &key,
                    longitude,
                    latitude,
                    radius * to_meter,
                );
                apply_sort_and_count(&mut matches, &opts);

                // Handle STORE
                if let Some(ref store_key) = opts.store_key {
                    let mut inner = state.lock();
                    let now = inner.effective_now();
                    let db = inner.db_mut(ctx.selected_db);
                    db.del(store_key);
                    for m in &matches {
                        db.sset_add(store_key, m.score, &m.name, now);
                    }
                    return Frame::Integer(matches.len() as i64);
                }

                // Handle STOREDIST
                if let Some(ref storedist_key) = opts.storedist_key {
                    let mut inner = state.lock();
                    let now = inner.effective_now();
                    let db = inner.db_mut(ctx.selected_db);
                    db.del(storedist_key);
                    for m in &matches {
                        db.sset_add(storedist_key, m.distance / to_meter, &m.name, now);
                    }
                    return Frame::Integer(matches.len() as i64);
                }

                format_radius_results(&matches, &opts, to_meter)
            }
            None => Frame::error("ERR could not decode requested zset member"),
        }
    }
}

fn cmd_georadiusbymember(state: &Arc<SharedState>, ctx: &mut ConnCtx, args: &[Vec<u8>]) -> Frame {
    cmd_georadiusbymember_impl(state, ctx, args, false, "georadiusbymember")
}

fn cmd_georadiusbymember_ro(
    state: &Arc<SharedState>,
    ctx: &mut ConnCtx,
    args: &[Vec<u8>],
) -> Frame {
    cmd_georadiusbymember_impl(state, ctx, args, true, "georadiusbymember_ro")
}

// ── Helpers ─────────────────────────────────────────────────────────

fn to_str(bytes: &[u8]) -> String {
    String::from_utf8_lossy(bytes).to_string()
}
